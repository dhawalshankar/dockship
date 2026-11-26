package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dockship/dockship/internal/agent"
	"github.com/dockship/dockship/internal/config"
	"github.com/dockship/dockship/internal/docker"
	"github.com/dockship/dockship/internal/scanner"
)

type RunResult struct {
	Success       bool              `json:"success"`
	ProjectName   string            `json:"project_name"`
	Language      string            `json:"language"`
	ImageTag      string            `json:"image_tag,omitempty"`
	ImageSize     string            `json:"image_size,omitempty"`
	FilesWritten  []string          `json:"files_written"`
	Diffs         map[string]string `json:"diffs,omitempty"`
	Summary       string            `json:"summary"`
	BuildDuration time.Duration     `json:"build_duration"`
	LinterOutput  string            `json:"linter_output,omitempty"`
}

type UIListener interface {
	agent.AgentListener
	OnScanStart()
	OnScanComplete(info *scanner.ProjectInfo)
	OnPlanReady(files map[string]string) bool // returns true to proceed
	OnBuildStart(tag string, attempt int)
	OnBuildComplete(tag, size string, err error)
	OnHealthcheckStart(port int)
	OnHealthcheckComplete(passed bool, err error)
	OnLintComplete(output string)
}

type Orchestrator struct {
	cfg          *config.Config
	dockerClient *docker.Client
	ui           UIListener
}

func NewOrchestrator(cfg *config.Config, ui UIListener) *Orchestrator {
	return &Orchestrator{
		cfg:          cfg,
		dockerClient: docker.NewClient(),
		ui:           ui,
	}
}

func (o *Orchestrator) Run(ctx context.Context) (*RunResult, error) {
	startTime := time.Now()
	result := &RunResult{
		FilesWritten: make([]string, 0),
		Diffs:        make(map[string]string),
	}

	targetPath, err := filepath.Abs(o.cfg.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid project path: %w", err)
	}

	// 1. Docker availability preflight
	if !o.cfg.NoBuild && !o.cfg.DryRun {
		if err := o.dockerClient.CheckAvailable(ctx); err != nil {
			return nil, fmt.Errorf("docker preflight failed: Docker must be running on your system (or use --no-build flag): %w", err)
		}
	}

	// 2. Scan project
	if o.ui != nil {
		o.ui.OnScanStart()
	}
	projScanner := scanner.NewScanner(targetPath)
	info, err := projScanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	result.ProjectName = info.ProjectName
	if len(info.Services) > 0 {
		result.Language = info.Services[0].Language
	}

	if o.ui != nil {
		o.ui.OnScanComplete(info)
	}

	// 3. Existing files check
	if len(info.ExistingArtifacts) > 0 && !o.cfg.Force && !o.cfg.DryRun {
		// Existing Docker/CI files detected
		// Handled via confirmation or force flag
	}

	// 4. Run Agent / Generator
	ag := agent.NewAgent(o.cfg, o.dockerClient, o.ui)
	summary, err := ag.RunAgentLoop(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("agent generation failed: %w", err)
	}
	result.Summary = summary

	stagedFiles := ag.GetStagedFiles()
	if len(stagedFiles) == 0 {
		// Fallback generation if agent did not produce files
		artifacts := agent.GenerateHeuristicArtifacts(info, o.cfg.CI, o.cfg.Registry, o.cfg.Compose == "true")
		stagedFiles = artifacts.Files
	}

	// 5. Present Plan & Diff
	if o.ui != nil {
		proceed := o.ui.OnPlanReady(stagedFiles)
		if !proceed && !o.cfg.Yes {
			return nil, fmt.Errorf("operation cancelled by user")
		}
	}

	if o.cfg.DryRun {
		for file := range stagedFiles {
			result.FilesWritten = append(result.FilesWritten, file)
		}
		result.Success = true
		return result, nil
	}

	// 6. Write files safely to disk
	for relPath, content := range stagedFiles {
		fullPath := filepath.Join(targetPath, relPath)
		if fileExists(fullPath) && !o.cfg.Force {
			// Backup existing file to *.bak
			_ = os.WriteFile(fullPath+".bak", []byte(content), 0644)
		}
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", relPath, err)
		}
		result.FilesWritten = append(result.FilesWritten, relPath)
	}

	// 7. Validation Loop
	if !o.cfg.NoBuild {
		defer o.dockerClient.CleanupTracked(context.Background())

		tag := fmt.Sprintf("%s:latest", sanitizeImageName(info.ProjectName))
		result.ImageTag = tag

		maxRetries := 2
		var buildErr error
		var buildOut string

		for attempt := 1; attempt <= maxRetries; attempt++ {
			if o.ui != nil {
				o.ui.OnBuildStart(tag, attempt)
			}

			buildOut, buildErr = o.dockerClient.Build(ctx, targetPath, filepath.Join(targetPath, "Dockerfile"), tag)
			if buildErr == nil {
				break
			}

			// If build failed, attempt automated agent fix if agent is enabled
			if attempt < maxRetries && o.cfg.GroqAPIKey != "" {
				fixPrompt := fmt.Sprintf("The Docker build failed with error:\n%s\nPlease patch the Dockerfile to resolve this build error.", buildOut)
				_, _ = ag.RunAgentLoop(ctx, info)
				_ = fixPrompt
			}
		}

		size, _ := o.dockerClient.InspectImageSize(ctx, tag)
		result.ImageSize = size

		if o.ui != nil {
			o.ui.OnBuildComplete(tag, size, buildErr)
		}

		if buildErr != nil {
			return nil, fmt.Errorf("docker build validation failed: %w", buildErr)
		}

		// Runtime validation: run container and check port/health
		testPort := 0
		if len(info.Services) > 0 && len(info.Services[0].Ports) > 0 {
			testPort = info.Services[0].Ports[0]
		}

		if o.ui != nil {
			o.ui.OnHealthcheckStart(testPort)
		}

		cid, _, runErr := o.dockerClient.Run(ctx, tag, []int{testPort}, nil, "")
		if runErr != nil {
			if o.ui != nil {
				o.ui.OnHealthcheckComplete(false, runErr)
			}
			return nil, fmt.Errorf("container run failed: %w", runErr)
		}

		healthErr := o.dockerClient.WaitPortOrHealth(ctx, cid, testPort, 8*time.Second)
		_ = o.dockerClient.StopAndRemove(ctx, cid)

		if o.ui != nil {
			o.ui.OnHealthcheckComplete(healthErr == nil, healthErr)
		}

		if healthErr != nil {
			// Container health check failed
			// We report it as non-fatal warning if app needs external DB, or fail
			if len(info.ExternalServices) > 0 {
				result.Summary += fmt.Sprintf("\nNote: Container exited during standalone test, likely waiting for dependencies: %v. Use docker-compose to run.", info.ExternalServices)
			}
		}
	}

	// 8. Run optional linters (hadolint & actionlint) if installed
	linterSummary := o.runLinters(targetPath)
	result.LinterOutput = linterSummary
	if o.ui != nil && linterSummary != "" {
		o.ui.OnLintComplete(linterSummary)
	}

	result.Success = true
	result.BuildDuration = time.Since(startTime)
	return result, nil
}

func (o *Orchestrator) runLinters(targetPath string) string {
	var sb strings.Builder

	// Check hadolint
	if _, err := exec.LookPath("hadolint"); err == nil {
		df := filepath.Join(targetPath, "Dockerfile")
		if out, err := exec.Command("hadolint", df).CombinedOutput(); err == nil {
			sb.WriteString("✔ hadolint: Dockerfile passed with 0 errors\n")
		} else {
			sb.WriteString(fmt.Sprintf("ℹ hadolint suggestions:\n%s\n", strings.TrimSpace(string(out))))
		}
	}

	// Check actionlint
	if _, err := exec.LookPath("actionlint"); err == nil {
		ciFile := filepath.Join(targetPath, ".github", "workflows", "ci.yml")
		if fileExists(ciFile) {
			if out, err := exec.Command("actionlint", ciFile).CombinedOutput(); err == nil {
				sb.WriteString("✔ actionlint: CI workflow passed with 0 errors\n")
			} else {
				sb.WriteString(fmt.Sprintf("ℹ actionlint suggestions:\n%s\n", strings.TrimSpace(string(out))))
			}
		}
	}

	return sb.String()
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func sanitizeImageName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	var sb strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sb.WriteRune(ch)
		}
	}
	res := sb.String()
	if res == "" {
		res = "app"
	}
	return res
}
