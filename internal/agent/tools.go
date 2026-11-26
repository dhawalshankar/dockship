package agent

import (
	"encoding/json"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func GetAgentTools() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "list_dir",
				Description: "List files and subdirectories at a path relative to the project root",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "Relative path inside the project (e.g. '.' or 'src')",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "read_file",
				Description: "Read content of a project file (up to max_bytes). Secrets and keys are strictly filtered.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "Relative path of file to read",
						},
						"max_bytes": {
							Type:        jsonschema.Integer,
							Description: "Max bytes to read (default 8000)",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search",
				Description: "Search for a keyword or regex across files in the project",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"pattern": {
							Type:        jsonschema.String,
							Description: "Search pattern or keyword",
						},
					},
					Required: []string{"pattern"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "write_file",
				Description: "Write or overwrite a generated artifact file (sandboxed inside project)",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "Relative path of the file (e.g. 'Dockerfile', '.dockerignore', 'docker-compose.yml', '.github/workflows/ci.yml', 'DOCKSHIP.md')",
						},
						"content": {
							Type:        jsonschema.String,
							Description: "Exact file content to write",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "docker_build",
				Description: "Execute a docker build against the project context to validate the Dockerfile",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tag": {
							Type:        jsonschema.String,
							Description: "Image tag to build (e.g. 'dockship-test:latest')",
						},
						"dockerfile": {
							Type:        jsonschema.String,
							Description: "Relative path to Dockerfile (defaults to 'Dockerfile')",
						},
					},
					Required: []string{"tag"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "docker_run",
				Description: "Run the built image to verify container startup and healthcheck",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"tag": {
							Type:        jsonschema.String,
							Description: "Image tag to run",
						},
						"ports": {
							Type: jsonschema.Array,
							Items: &jsonschema.Definition{
								Type: jsonschema.Integer,
							},
							Description: "Ports to publish (e.g. [3000])",
						},
					},
					Required: []string{"tag"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "docker_logs",
				Description: "Fetch logs from a running or exited container to diagnose errors",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"container_id": {
							Type:        jsonschema.String,
							Description: "Container ID or name",
						},
					},
					Required: []string{"container_id"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "finish",
				Description: "Conclude the agent run when artifacts are generated and validated",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"summary": {
							Type:        jsonschema.String,
							Description: "Summary of actions taken, fixes applied, and final status",
						},
					},
					Required: []string{"summary"},
				},
			},
		},
	}
}

// Tool invocation parameter structs
type ListDirParams struct {
	Path string `json:"path"`
}

type ReadFileParams struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"max_bytes"`
}

type SearchParams struct {
	Pattern string `json:"pattern"`
}

type WriteFileParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type DockerBuildParams struct {
	Tag        string `json:"tag"`
	Dockerfile string `json:"dockerfile"`
}

type DockerRunParams struct {
	Tag   string `json:"tag"`
	Ports []int  `json:"ports"`
}

type DockerLogsParams struct {
	ContainerID string `json:"container_id"`
}

type FinishParams struct {
	Summary string `json:"summary"`
}

func ParseParams[T any](raw string) (*T, error) {
	var val T
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return nil, err
	}
	return &val, nil
}
