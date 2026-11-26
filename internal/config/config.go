package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Flags & CLI Options
	TargetPath    string `yaml:"target_path"`
	CI            string `yaml:"ci"`             // github, gitlab, none
	Registry      string `yaml:"registry"`       // ghcr, dockerhub, ecr, gcr, none
	Yes           bool   `yaml:"yes"`            // skip confirmation
	DryRun        bool   `yaml:"dry_run"`        // show generated files, write nothing
	NoBuild       bool   `yaml:"no_build"`       // skip docker build/run validation
	Compose       string `yaml:"compose"`        // auto, true, false
	Model         string `yaml:"model"`          // LLM model override
	MaxIterations int    `yaml:"max_iterations"` // agent fix-loop cap
	Verbose       bool   `yaml:"verbose"`        // show tool calls & reasoning
	Force         bool   `yaml:"force"`          // overwrite existing Docker/CI files
	JSON          bool   `yaml:"json"`           // machine readable output

	// LLM Settings
	GroqAPIKey string `yaml:"groq_api_key"`
	BaseURL    string `yaml:"base_url"`

	// Per-project overrides from .dockship.yml
	Overrides ProjectOverrides `yaml:"overrides"`
}

type ProjectOverrides struct {
	Ports       []int    `yaml:"ports"`
	BaseImage   string   `yaml:"base_image"`
	Registry    string   `yaml:"registry"`
	IgnoredPaths []string `yaml:"ignored_paths"`
	Services    []string `yaml:"services"`
}

func DefaultConfig() *Config {
	return &Config{
		CI:            "github",
		Registry:      "ghcr",
		Compose:       "auto",
		Model:         "openai/gpt-oss-120b",
		MaxIterations: 8,
		BaseURL:       "https://api.groq.com/openai/v1",
	}
}

// LoadConfig resolves configuration from defaults, user config file, env vars, and per-project .dockship.yml
func LoadConfig(targetPath string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.TargetPath = targetPath

	// 1. Env vars
	if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		cfg.GroqAPIKey = strings.TrimSpace(apiKey)
	} else if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		// Fallback to OpenAI if user provided OpenAI credentials
		cfg.GroqAPIKey = strings.TrimSpace(apiKey)
		if os.Getenv("OPENAI_BASE_URL") != "" {
			cfg.BaseURL = os.Getenv("OPENAI_BASE_URL")
		} else {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
	}

	if envModel := os.Getenv("DOCKSHIP_MODEL"); envModel != "" {
		cfg.Model = envModel
	}
	if envBaseURL := os.Getenv("GROQ_BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	// 2. Global user config (~/.config/dockship/config.toml or .yaml)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalYaml := filepath.Join(homeDir, ".config", "dockship", "config.yaml")
		if data, err := os.ReadFile(globalYaml); err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	// 3. Project-level .dockship.yml
	if targetPath != "" {
		projectYaml := filepath.Join(targetPath, ".dockship.yml")
		if data, err := os.ReadFile(projectYaml); err == nil {
			var projCfg struct {
				CI        string           `yaml:"ci"`
				Registry  string           `yaml:"registry"`
				Compose   string           `yaml:"compose"`
				Model     string           `yaml:"model"`
				Overrides ProjectOverrides `yaml:"overrides"`
			}
			if err := yaml.Unmarshal(data, &projCfg); err == nil {
				if projCfg.CI != "" {
					cfg.CI = projCfg.CI
				}
				if projCfg.Registry != "" {
					cfg.Registry = projCfg.Registry
				}
				if projCfg.Compose != "" {
					cfg.Compose = projCfg.Compose
				}
				if projCfg.Model != "" {
					cfg.Model = projCfg.Model
				}
				cfg.Overrides = projCfg.Overrides
			}
		}
	}

	return cfg, nil
}
