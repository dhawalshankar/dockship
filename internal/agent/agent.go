package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dockship/dockship/internal/config"
	"github.com/dockship/dockship/internal/docker"
	"github.com/dockship/dockship/internal/scanner"
	"github.com/sashabaranov/go-openai"
)

type AgentListener interface {
	OnToolCall(name, args string)
	OnToolResult(name, result string, err error)
	OnAgentThought(thought string)
}

type Agent struct {
	cfg          *config.Config
	client       *openai.Client
	dockerClient *docker.Client
	rootDir      string
	listener     AgentListener
	stagedFiles  map[string]string // relPath -> content staged by write_file
}

func NewAgent(cfg *config.Config, dockerClient *docker.Client, listener AgentListener) *Agent {
	var client *openai.Client
	if cfg.GroqAPIKey != "" {
		clientCfg := openai.DefaultConfig(cfg.GroqAPIKey)
		if cfg.BaseURL != "" {
			clientCfg.BaseURL = cfg.BaseURL
		}
		client = openai.NewClientWithConfig(clientCfg)
	}

	absRoot, _ := filepath.Abs(cfg.TargetPath)

	return &Agent{
		cfg:          cfg,
		client:       client,
		dockerClient: dockerClient,
		rootDir:      absRoot,
		listener:     listener,
		stagedFiles:  make(map[string]string),
	}
}

func (a *Agent) GetStagedFiles() map[string]string {
	return a.stagedFiles
}

// RunAgentLoop executes the iterative reasoning and tool-calling loop.
func (a *Agent) RunAgentLoop(ctx context.Context, projInfo *scanner.ProjectInfo) (string, error) {
	// If no API key configured, use intelligent heuristic generation
	if a.client == nil {
		if a.listener != nil {
			a.listener.OnAgentThought("No GROQ_API_KEY detected. Utilizing expert built-in heuristics engine...")
		}
		artifacts := GenerateHeuristicArtifacts(projInfo, a.cfg.CI, a.cfg.Registry, a.cfg.Compose == "true")
		for k, v := range artifacts.Files {
			a.stagedFiles[k] = v
		}
		return "Generated artifacts via Dockship built-in heuristics engine.", nil
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: SystemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: a.buildInitialPrompt(projInfo),
		},
	}

	tools := GetAgentTools()
	maxIterations := a.cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 8
	}

	var finalSummary string

	for iteration := 1; iteration <= maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		resp, err := a.callLLMWithBackoff(ctx, messages, tools)
		if err != nil {
			if a.listener != nil {
				a.listener.OnAgentThought(fmt.Sprintf("LLM API rate-limited or unavailable (%v). Seamlessly switching to built-in expert heuristics engine...", err))
			}
			artifacts := GenerateHeuristicArtifacts(projInfo, a.cfg.CI, a.cfg.Registry, a.cfg.Compose == "true")
			for k, v := range artifacts.Files {
				a.stagedFiles[k] = v
			}
			return "Generated artifacts via Dockship built-in heuristics engine (LLM rate-limit fallback).", nil
		}

		choice := resp.Choices[0]
		assistantMsg := choice.Message
		messages = append(messages, assistantMsg)

		if assistantMsg.Content != "" && a.listener != nil {
			a.listener.OnAgentThought(assistantMsg.Content)
		}

		// If no tools called, we might be finished or waiting
		if len(assistantMsg.ToolCalls) == 0 {
			finalSummary = assistantMsg.Content
			break
		}

		// Execute tool calls
		finished := false
		for _, tc := range assistantMsg.ToolCalls {
			fnName := tc.Function.Name
			fnArgs := tc.Function.Arguments

			if a.listener != nil {
				a.listener.OnToolCall(fnName, fnArgs)
			}

			toolResult, err := a.executeTool(ctx, fnName, fnArgs)
			if a.listener != nil {
				a.listener.OnToolResult(fnName, toolResult, err)
			}

			if err != nil {
				toolResult = fmt.Sprintf("ERROR: %v", err)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Name:       fnName,
				Content:    toolResult,
			})

			if fnName == "finish" {
				finished = true
				var p FinishParams
				_ = json.Unmarshal([]byte(fnArgs), &p)
				finalSummary = p.Summary
			}
		}

		if finished {
			break
		}
	}

	if finalSummary == "" {
		finalSummary = "Completed containerization generation cycle."
	}

	return finalSummary, nil
}

func (a *Agent) callLLMWithBackoff(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) (*openai.ChatCompletionResponse, error) {
	var resp openai.ChatCompletionResponse
	var err error

	model := a.cfg.Model
	if model == "" {
		model = "openai/gpt-oss-120b"
	}

	for attempt := 0; attempt < 3; attempt++ {
		req := openai.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.1,
		}

		resp, err = a.client.CreateChatCompletion(ctx, req)
		if err == nil {
			return &resp, nil
		}

		// Rate limit check
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate_limit") {
			// Try fallback model if on 120b
			if strings.Contains(model, "120b") {
				model = "openai/gpt-oss-20b"
			}
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}

		break
	}

	return nil, err
}

func (a *Agent) executeTool(ctx context.Context, name, args string) (string, error) {
	switch name {
	case "list_dir":
		p, err := ParseParams[ListDirParams](args)
		if err != nil {
			return "", err
		}
		pathVal := p.Path
		if pathVal == "" {
			pathVal = p.Dir
		}
		target := filepath.Join(a.rootDir, pathVal)
		if !a.isPathSafe(target) {
			return "", fmt.Errorf("permission denied: path is outside project root")
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			return "", err
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name()+"/")
			} else {
				names = append(names, e.Name())
			}
		}
		return strings.Join(names, "\n"), nil

	case "read_file":
		p, err := ParseParams[ReadFileParams](args)
		if err != nil {
			return "", err
		}
		fileVal := p.Path
		if fileVal == "" {
			fileVal = p.File
		}
		target := filepath.Join(a.rootDir, fileVal)
		if !a.isPathSafe(target) {
			return "", fmt.Errorf("permission denied: path is outside project root")
		}
		// Security check: Never read sensitive files
		lower := strings.ToLower(filepath.Base(target))
		if strings.HasPrefix(lower, ".env") || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") || strings.Contains(lower, "id_rsa") {
			return "", fmt.Errorf("access denied: reading raw secret or env files is forbidden by security policy")
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return "", err
		}
		maxB := p.MaxBytes
		if maxB <= 0 || maxB > 16000 {
			maxB = 8000
		}
		if len(data) > maxB {
			data = data[:maxB]
		}
		return string(data), nil

	case "search":
		p, err := ParseParams[SearchParams](args)
		if err != nil {
			return "", err
		}
		pat := p.Pattern
		if pat == "" {
			pat = p.Query
		}
		if pat == "" {
			pat = p.Keyword
		}
		if pat == "" {
			return "No search pattern specified.", nil
		}
		var matches []string
		_ = filepath.WalkDir(a.rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || len(matches) > 15 {
				return nil
			}
			rel, _ := filepath.Rel(a.rootDir, path)
			if strings.HasPrefix(rel, ".git") || strings.Contains(rel, "node_modules") {
				return nil
			}
			data, _ := os.ReadFile(path)
			if strings.Contains(string(data), pat) {
				matches = append(matches, rel)
			}
			return nil
		})
		if len(matches) == 0 {
			return "No matches found.", nil
		}
		return strings.Join(matches, "\n"), nil

	case "write_file":
		p, err := ParseParams[WriteFileParams](args)
		if err != nil {
			return "", err
		}
		filePath := p.Path
		if filePath == "" {
			filePath = p.File
		}
		content := p.Content
		if content == "" {
			content = p.Code
		}
		target := filepath.Join(a.rootDir, filePath)
		if !a.isPathSafe(target) {
			return "", fmt.Errorf("permission denied: path is outside project root")
		}
		cleanRel, _ := filepath.Rel(a.rootDir, target)
		cleanRel = filepath.ToSlash(cleanRel)
		a.stagedFiles[cleanRel] = content

		// If not dry-run and validation is active, write to disk so docker can build it
		if !a.cfg.DryRun {
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			if err := os.WriteFile(target, []byte(content), 0644); err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("Successfully staged/wrote %s (%d bytes)", cleanRel, len(content)), nil

	case "docker_build":
		p, err := ParseParams[DockerBuildParams](args)
		if err != nil {
			return "", err
		}
		if a.cfg.NoBuild {
			return "Docker build skipped (--no-build flag active)", nil
		}
		df := filepath.Join(a.rootDir, p.Dockerfile)
		if p.Dockerfile == "" {
			df = filepath.Join(a.rootDir, "Dockerfile")
		}
		out, err := a.dockerClient.Build(ctx, a.rootDir, df, p.Tag)
		if err != nil {
			return fmt.Sprintf("BUILD FAILED:\n%s", out), err
		}
		return fmt.Sprintf("BUILD SUCCESSFUL:\n%s", out), nil

	case "docker_run":
		p, err := ParseParams[DockerRunParams](args)
		if err != nil {
			return "", err
		}
		if a.cfg.NoBuild {
			return "Docker run skipped (--no-build flag active)", nil
		}
		cid, _, err := a.dockerClient.Run(ctx, p.Tag, p.Ports, nil, "")
		if err != nil {
			return fmt.Sprintf("RUN FAILED: %v", err), err
		}
		targetPort := 0
		if len(p.Ports) > 0 {
			targetPort = p.Ports[0]
		}
		// Wait for health or timeout
		waitErr := a.dockerClient.WaitPortOrHealth(ctx, cid, targetPort, 10*time.Second)
		logs, _ := a.dockerClient.Logs(ctx, cid, 50)
		_ = a.dockerClient.StopAndRemove(ctx, cid)

		if waitErr != nil {
			return fmt.Sprintf("CONTAINER RUN CRASHED OR HEALTHCHECK FAILED:\nError: %v\nLogs:\n%s", waitErr, logs), waitErr
		}
		return fmt.Sprintf("CONTAINER STARTED AND HEALTHCHECK PASSED on port %d. Logs:\n%s", targetPort, logs), nil

	case "docker_logs":
		p, err := ParseParams[DockerLogsParams](args)
		if err != nil {
			return "", err
		}
		logs, err := a.dockerClient.Logs(ctx, p.ContainerID, 100)
		return logs, err

	case "finish":
		p, err := ParseParams[FinishParams](args)
		if err != nil {
			return "", err
		}
		summary := p.Summary
		if summary == "" {
			summary = p.Message
		}
		return fmt.Sprintf("FINISHED: %s", summary), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (a *Agent) isPathSafe(targetPath string) bool {
	cleanTarget := filepath.Clean(targetPath)
	cleanRoot := filepath.Clean(a.rootDir)
	return strings.HasPrefix(cleanTarget, cleanRoot)
}

func (a *Agent) buildInitialPrompt(info *scanner.ProjectInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Please analyze and containerize project: %s\n\n", info.ProjectName))
	sb.WriteString("### DETECTED RECON SUMMARY:\n")
	if len(info.Services) > 0 {
		for i, svc := range info.Services {
			sb.WriteString(fmt.Sprintf("Service %d:\n", i+1))
			sb.WriteString(fmt.Sprintf("  - Language: %s\n", svc.Language))
			sb.WriteString(fmt.Sprintf("  - Framework: %s\n", svc.Framework))
			sb.WriteString(fmt.Sprintf("  - Package Manager: %s\n", svc.PackageManager))
			sb.WriteString(fmt.Sprintf("  - Runtime Version: %s\n", svc.RuntimeVersion))
			sb.WriteString(fmt.Sprintf("  - Build Command: %s\n", svc.BuildCommand))
			sb.WriteString(fmt.Sprintf("  - Start Command: %s\n", svc.StartCommand))
			sb.WriteString(fmt.Sprintf("  - Ports: %v\n", svc.Ports))
		}
	} else {
		sb.WriteString("No standard language manifests detected at root. Explore files using list_dir/read_file.\n")
	}

	if len(info.ExternalServices) > 0 {
		sb.WriteString(fmt.Sprintf("External services detected: %v\n", info.ExternalServices))
	}
	if len(info.EnvVars) > 0 {
		sb.WriteString(fmt.Sprintf("Required environment keys: %v\n", info.EnvVars))
	}
	if len(info.ExistingArtifacts) > 0 {
		sb.WriteString(fmt.Sprintf("Existing Docker/CI files: %v\n", info.ExistingArtifacts))
	}

	sb.WriteString("\n### PROJECT FILE TREE:\n")
	sb.WriteString(info.FileTreeSummary)

	sb.WriteString(fmt.Sprintf("\nTarget CI Provider: %s\nTarget Registry: %s\n", a.cfg.CI, a.cfg.Registry))
	sb.WriteString("Follow best practices: multi-stage, non-root, layer-caching, .dockerignore, docker-compose if needed, CI/CD, DOCKSHIP.md. Validate with docker_build and docker_run, then call finish.")

	return sb.String()
}
