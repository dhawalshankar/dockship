package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dockship/dockship/internal/config"
	"github.com/dockship/dockship/internal/orchestrator"
	"github.com/dockship/dockship/internal/reporter"
	"github.com/spf13/cobra"
)

var (
	version = "0.2.0"

	ciFlag            string
	registryFlag      string
	yesFlag           bool
	dryRunFlag        bool
	noBuildFlag       bool
	composeFlag       string
	modelFlag         string
	maxIterationsFlag int
	verboseFlag       bool
	forceFlag         bool
	jsonFlag          bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dockship [path-to-project]",
		Short: "Dockship: AI-powered containerization & CI/CD pipeline generator",
		Long: `Dockship is an AI-powered CLI that takes any project (any language, any framework)
and fully containerizes it, including CI/CD setup, with a single command:

  dockship <path-to-project>

It analyzes the codebase, generates production-grade Docker and CI/CD artifacts,
and validates them by building and running the container.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}

			cfg, err := config.LoadConfig(targetPath)
			if err != nil {
				return fmt.Errorf("configuration error: %w", err)
			}

			// Apply command-line flag overrides
			if cmd.Flags().Changed("ci") {
				cfg.CI = ciFlag
			}
			if cmd.Flags().Changed("registry") {
				cfg.Registry = registryFlag
			}
			if cmd.Flags().Changed("yes") {
				cfg.Yes = yesFlag
			}
			if cmd.Flags().Changed("dry-run") {
				cfg.DryRun = dryRunFlag
			}
			if cmd.Flags().Changed("no-build") {
				cfg.NoBuild = noBuildFlag
			}
			if cmd.Flags().Changed("compose") {
				cfg.Compose = composeFlag
			}
			if cmd.Flags().Changed("model") {
				cfg.Model = modelFlag
			}
			if cmd.Flags().Changed("max-iterations") {
				cfg.MaxIterations = maxIterationsFlag
			}
			if cmd.Flags().Changed("verbose") {
				cfg.Verbose = verboseFlag
			}
			if cmd.Flags().Changed("force") {
				cfg.Force = forceFlag
			}
			if cmd.Flags().Changed("json") {
				cfg.JSON = jsonFlag
			}

			termReporter := reporter.NewTerminalReporter(cfg)
			orch := orchestrator.NewOrchestrator(cfg, termReporter)

			ctx := context.Background()
			res, err := orch.Run(ctx)
			if err != nil {
				if cfg.JSON {
					fmt.Printf(`{"success": false, "error": %q}`+"\n", err.Error())
				} else {
					fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
				}
				os.Exit(1)
			}

			termReporter.PrintFinalReport(res)
			return nil
		},
	}

	rootCmd.Version = version

	rootCmd.Flags().StringVar(&ciFlag, "ci", "github", "CI/CD provider (github, gitlab, none)")
	rootCmd.Flags().StringVar(&registryFlag, "registry", "ghcr", "Container registry (ghcr, dockerhub, ecr, gcr, none)")
	rootCmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation, write all files")
	rootCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show generated files, write nothing")
	rootCmd.Flags().BoolVar(&noBuildFlag, "no-build", false, "Skip docker build/run validation")
	rootCmd.Flags().StringVar(&composeFlag, "compose", "auto", "Force docker-compose generation (auto, true, false)")
	rootCmd.Flags().StringVar(&modelFlag, "model", "openai/gpt-oss-120b", "Override LLM model")
	rootCmd.Flags().IntVar(&maxIterationsFlag, "max-iterations", 8, "Max agent fix-loop iterations")
	rootCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "Show agent reasoning and tool calls")
	rootCmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing Docker/CI files")
	rootCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output machine-readable JSON")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
