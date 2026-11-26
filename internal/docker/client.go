package docker

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	dockerBinary string
	trackedContainers []string
	trackedImages     []string
	mu                sync.Mutex
}

func NewClient() *Client {
	bin := "docker"
	if p, err := exec.LookPath("podman"); err == nil && !isBinaryAvailable("docker") {
		bin = p
	}
	return &Client{
		dockerBinary:      bin,
		trackedContainers: make([]string, 0),
		trackedImages:     make([]string, 0),
	}
}

func isBinaryAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CheckAvailable verifies if Docker daemon/CLI is responsive.
func (c *Client) CheckAvailable(ctx context.Context) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, c.dockerBinary, "version")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker check failed: %v: %s", err, strings.TrimSpace(errOut.String()))
	}
	return nil
}

// Build executes `docker build` and returns the output logs.
func (c *Client) Build(ctx context.Context, contextDir, dockerfilePath, tag string) (string, error) {
	c.mu.Lock()
	c.trackedImages = append(c.trackedImages, tag)
	c.mu.Unlock()

	args := []string{"build", "-t", tag}
	if dockerfilePath != "" {
		args = append(args, "-f", dockerfilePath)
	}
	args = append(args, contextDir)

	cmd := exec.CommandContext(ctx, c.dockerBinary, args...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	err := cmd.Run()
	output := combined.String()
	if err != nil {
		return output, fmt.Errorf("docker build failed: %w\n%s", err, truncateLogs(output, 2000))
	}
	return output, nil
}

// Run executes the container in detached mode and tracks the container ID.
func (c *Client) Run(ctx context.Context, tag string, ports []int, env []string, containerName string) (string, string, error) {
	if containerName == "" {
		containerName = fmt.Sprintf("dockship-test-%d", time.Now().UnixNano()%100000)
	}

	args := []string{"run", "-d", "--name", containerName}
	for _, p := range ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", p, p))
	}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, tag)

	cmd := exec.CommandContext(ctx, c.dockerBinary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", stderr.String(), fmt.Errorf("docker run failed: %w: %s", err, stderr.String())
	}

	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		containerID = containerName
	}

	c.mu.Lock()
	c.trackedContainers = append(c.trackedContainers, containerID)
	c.mu.Unlock()

	return containerID, stdout.String(), nil
}

// Logs fetches container logs.
func (c *Client) Logs(ctx context.Context, containerID string, tail int) (string, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", strconv.Itoa(tail))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, c.dockerBinary, args...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	_ = cmd.Run()
	return combined.String(), nil
}

// WaitPortOrHealth polls the mapped port or inspects docker health until ready or timeout.
func (c *Client) WaitPortOrHealth(ctx context.Context, containerID string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check if container is still running
			statusCmd := exec.CommandContext(ctx, c.dockerBinary, "inspect", "-f", "{{.State.Status}}", containerID)
			var statusOut bytes.Buffer
			statusCmd.Stdout = &statusOut
			if err := statusCmd.Run(); err == nil {
				status := strings.TrimSpace(statusOut.String())
				if status == "exited" || status == "dead" {
					logs, _ := c.Logs(ctx, containerID, 40)
					return fmt.Errorf("container exited prematurely (status: %s):\n%s", status, logs)
				}
			}

			// If port is specified, test TCP connection
			if port > 0 {
				conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
				if err == nil {
					_ = conn.Close()
					return nil
				}
			} else {
				// If no port, check if it stayed running for 2 seconds
				if time.Now().After(deadline.Add(-timeout + 2*time.Second)) {
					return nil
				}
			}

			if time.Now().After(deadline) {
				logs, _ := c.Logs(ctx, containerID, 30)
				return fmt.Errorf("healthcheck timeout reached (%v). Last logs:\n%s", timeout, logs)
			}
		}
	}
}

// InspectImageSize returns a human-readable size of the built image.
func (c *Client) InspectImageSize(ctx context.Context, tag string) (string, error) {
	cmd := exec.CommandContext(ctx, c.dockerBinary, "image", "inspect", tag, "--format", "{{.Size}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	bytesInt, err := strconv.ParseInt(strings.TrimSpace(out.String()), 10, 64)
	if err != nil {
		return "", err
	}
	return formatBytes(bytesInt), nil
}

// StopAndRemove stops and removes a container.
func (c *Client) StopAndRemove(ctx context.Context, containerID string) error {
	_ = exec.CommandContext(ctx, c.dockerBinary, "stop", "-t", "2", containerID).Run()
	return exec.CommandContext(ctx, c.dockerBinary, "rm", "-f", containerID).Run()
}

// CleanupTracked ensures any test containers or images are removed.
func (c *Client) CleanupTracked(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cid := range c.trackedContainers {
		_ = exec.CommandContext(ctx, c.dockerBinary, "rm", "-f", cid).Run()
	}
	c.trackedContainers = nil

	// Note: We keep the final built image if requested, but remove transient fix-attempt tags if needed
}

func truncateLogs(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	half := maxLen / 2
	return s[:half] + "\n... [TRUNCATED] ...\n" + s[len(s)-half:]
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
