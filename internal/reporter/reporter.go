package reporter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/dockship/dockship/internal/config"
	"github.com/dockship/dockship/internal/orchestrator"
	"github.com/dockship/dockship/internal/scanner"
)

var (
	green   = color.New(color.FgGreen, color.Bold).SprintFunc()
	cyan    = color.New(color.FgCyan).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	red     = color.New(color.FgRed, color.Bold).SprintFunc()
	bold    = color.New(color.Bold).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
	dim     = color.New(color.Faint).SprintFunc()
)

type TerminalReporter struct {
	cfg *config.Config
}

func NewTerminalReporter(cfg *config.Config) *TerminalReporter {
	return &TerminalReporter{cfg: cfg}
}

func (r *TerminalReporter) OnScanStart() {
	if r.cfg.JSON {
		return
	}
	fmt.Printf("%s Scanning project...\n", cyan("➤"))
}

func (r *TerminalReporter) OnScanComplete(info *scanner.ProjectInfo) {
	if r.cfg.JSON {
		return
	}

	var techSummary []string
	if len(info.Services) > 0 {
		svc := info.Services[0]
		if svc.Language != "" {
			techSummary = append(techSummary, svc.Language)
		}
		if svc.Framework != "" {
			techSummary = append(techSummary, svc.Framework)
		}
		if svc.PackageManager != "" {
			techSummary = append(techSummary, svc.PackageManager)
		}
		if svc.RuntimeVersion != "" {
			techSummary = append(techSummary, svc.RuntimeVersion)
		}
	}
	summaryStr := strings.Join(techSummary, ", ")
	if summaryStr != "" {
		fmt.Printf("%s Scanning project...            %s\n", green("✔"), dim(fmt.Sprintf("(%s)", summaryStr)))
	}

	if len(info.Services) > 0 {
		var servicesDesc []string
		for _, s := range info.Services {
			portStr := "no port"
			if len(s.Ports) > 0 {
				portStr = fmt.Sprintf(":%d", s.Ports[0])
			}
			servicesDesc = append(servicesDesc, fmt.Sprintf("%s (%s)", s.Name, portStr))
		}
		for _, ext := range info.ExternalServices {
			servicesDesc = append(servicesDesc, fmt.Sprintf("%s (external)", ext))
		}
		fmt.Printf("%s Detected services:             %s\n", green("✔"), dim(strings.Join(servicesDesc, ", ")))
	}
}

func (r *TerminalReporter) OnPlanReady(files map[string]string) bool {
	if r.cfg.JSON {
		return true
	}

	fmt.Printf("%s Plan ready:\n", green("✔"))
	for filename := range files {
		fmt.Printf("    %s %s\n", green("+"), bold(filename))
	}

	if r.cfg.DryRun {
		fmt.Printf("\n%s Dry run mode enabled. Generated file previews:\n", yellow("ℹ"))
		for name, content := range files {
			fmt.Printf("\n%s %s\n%s\n%s\n", cyan("---"), bold(name), cyan("---"), content)
		}
		return true
	}

	if r.cfg.Yes {
		return true
	}

	// Interactive prompt
	fmt.Printf("%s Write these files? (Y/n) ", yellow("?"))
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer == "" || answer == "y" || answer == "yes" {
			return true
		}
	}
	return false
}

func (r *TerminalReporter) OnBuildStart(tag string, attempt int) {
	if r.cfg.JSON {
		return
	}
	if attempt > 1 {
		fmt.Printf("%s Building image...              %s\n", cyan("➤"), yellow(fmt.Sprintf("(attempt %d)", attempt)))
	} else {
		fmt.Printf("%s Building image...\n", cyan("➤"))
	}
}

func (r *TerminalReporter) OnBuildComplete(tag, size string, err error) {
	if r.cfg.JSON {
		return
	}
	if err != nil {
		fmt.Printf("%s Docker build failed: %v\n", red("✖"), err)
	} else {
		sizeDesc := ""
		if size != "" {
			sizeDesc = fmt.Sprintf("Image size: %s", size)
		}
		fmt.Printf("%s Image built successfully!       %s\n", green("✔"), dim(sizeDesc))
	}
}

func (r *TerminalReporter) OnHealthcheckStart(port int) {
	if r.cfg.JSON {
		return
	}
	if port > 0 {
		fmt.Printf("%s Starting test container on port :%d...\n", cyan("➤"), port)
	} else {
		fmt.Printf("%s Starting test container...\n", cyan("➤"))
	}
}

func (r *TerminalReporter) OnHealthcheckComplete(passed bool, err error) {
	if r.cfg.JSON {
		return
	}
	if passed {
		fmt.Printf("%s Container started, healthcheck passed\n", green("✔"))
	} else {
		fmt.Printf("%s Container startup/health check failed: %v\n", yellow("!"), err)
	}
}

func (r *TerminalReporter) OnLintComplete(output string) {
	if r.cfg.JSON || output == "" {
		return
	}
	fmt.Print(output)
}

func (r *TerminalReporter) OnToolCall(name, args string) {
	if r.cfg.Verbose && !r.cfg.JSON {
		fmt.Printf("  %s %s: %s\n", magenta("⚙ Tool call:"), bold(name), dim(args))
	}
}

func (r *TerminalReporter) OnToolResult(name, result string, err error) {
	if r.cfg.Verbose && !r.cfg.JSON {
		if err != nil {
			fmt.Printf("  %s %s: %s\n", red("✖ Tool error:"), bold(name), err.Error())
		} else {
			resPreview := result
			if len(resPreview) > 120 {
				resPreview = resPreview[:120] + "..."
			}
			fmt.Printf("  %s %s: %s\n", green("✔ Tool result:"), bold(name), dim(resPreview))
		}
	}
}

func (r *TerminalReporter) OnAgentThought(thought string) {
	if r.cfg.Verbose && !r.cfg.JSON {
		fmt.Printf("  %s %s\n", cyan("🧠 Agent:"), thought)
	}
}

func (r *TerminalReporter) PrintFinalReport(res *orchestrator.RunResult) {
	if r.cfg.JSON {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\n%s %s\n", green("✔"), bold("Done."))
	if res.ImageSize != "" {
		fmt.Printf("  Image size:     %s\n", bold(res.ImageSize))
	}
	if res.ImageTag != "" {
		fmt.Printf("  Image tag:      %s\n", bold(res.ImageTag))
	}
	fmt.Printf("  Duration:       %s\n", res.BuildDuration.Round(100*1000000))
	fmt.Printf("  Files created:  %s\n", strings.Join(res.FilesWritten, ", "))
	fmt.Printf("\n%s\n", dim("See DOCKSHIP.md for usage and running instructions."))
}
