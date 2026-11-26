package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dockship/dockship/internal/config"
	"github.com/dockship/dockship/internal/scanner"
)

type mockUI struct{}

func (m *mockUI) OnScanStart()                               {}
func (m *mockUI) OnScanComplete(info *scanner.ProjectInfo)   {}
func (m *mockUI) OnPlanReady(files map[string]string) bool   { return true }
func (m *mockUI) OnBuildStart(tag string, attempt int)       {}
func (m *mockUI) OnBuildComplete(tag, size string, err error){}
func (m *mockUI) OnHealthcheckStart(port int)                {}
func (m *mockUI) OnHealthcheckComplete(passed bool, err error){}
func (m *mockUI) OnLintComplete(output string)               {}
func (m *mockUI) OnToolCall(name, args string)               {}
func (m *mockUI) OnToolResult(name, result string, err error){}
func (m *mockUI) OnAgentThought(thought string)              {}

func TestOrchestratorDryRunGoProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dockship-go-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	goMod := "module example.com/my-service\n\ngo 1.22\n\nrequire github.com/gin-gonic/gin v1.9.1\n"
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		TargetPath: tempDir,
		DryRun:     true,
		NoBuild:    true,
		CI:         "github",
		Registry:   "ghcr",
		Compose:    "auto",
	}

	orch := NewOrchestrator(cfg, &mockUI{})
	res, err := orch.Run(context.Background())
	if err != nil {
		t.Fatalf("orchestrator run failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success=true")
	}
	if res.Language != "Go" {
		t.Errorf("expected language Go, got %s", res.Language)
	}
}

func TestOrchestratorDryRunPythonProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dockship-py-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	reqs := "fastapi==0.110.0\nuvicorn==0.28.0\npsycopg2-binary==2.9.9\n"
	if err := os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte(reqs), 0644); err != nil {
		t.Fatal(err)
	}

	envFile := "DATABASE_URL=postgresql://user:password@localhost:5432/app\nPORT=8000\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envFile), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		TargetPath: tempDir,
		DryRun:     true,
		NoBuild:    true,
		CI:         "gitlab",
		Registry:   "dockerhub",
		Compose:    "auto",
	}

	orch := NewOrchestrator(cfg, &mockUI{})
	res, err := orch.Run(context.Background())
	if err != nil {
		t.Fatalf("orchestrator run failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success=true")
	}
	if res.Language != "Python" {
		t.Errorf("expected language Python, got %s", res.Language)
	}
}
