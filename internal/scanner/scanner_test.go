package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerBasic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dockship-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simulated Node.js express project
	pkgJSON := `{
		"name": "sample-express",
		"scripts": {
			"build": "tsc",
			"start": "node dist/index.js"
		},
		"dependencies": {
			"express": "^4.18.2"
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	envContent := "PORT=3000\nDATABASE_URL=postgres://user:secret@localhost:5432/mydb\nAPI_KEY=supersecret123\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tempDir)
	info, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(info.Services) == 0 {
		t.Fatalf("expected at least 1 service, got 0")
	}
	svc := info.Services[0]
	if svc.Language != "Node.js" {
		t.Errorf("expected Node.js, got %s", svc.Language)
	}
	if svc.Framework != "Express" {
		t.Errorf("expected Express, got %s", svc.Framework)
	}

	// Verify env var keys extracted without secrets leaked
	foundDB := false
	for _, env := range info.EnvVars {
		if env == "DATABASE_URL" {
			foundDB = true
		}
		if env == "supersecret123" {
			t.Errorf("SECRET LEAKED IN ENV KEYS!")
		}
	}
	if !foundDB {
		t.Errorf("expected DATABASE_URL in env vars")
	}

	// Verify external service detection
	foundPostgres := false
	for _, s := range info.ExternalServices {
		if s == "postgres" {
			foundPostgres = true
		}
	}
	if !foundPostgres {
		t.Errorf("expected postgres in external services")
	}
}
