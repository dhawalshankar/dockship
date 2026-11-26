package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerChromeExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dockship-extension-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	manifestJSON := `{
		"manifest_version": 3,
		"name": "Kimbho Translator",
		"version": "1.0.0",
		"description": "Translates text directly in browser"
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner(tempDir)
	info, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(info.Services) == 0 {
		t.Fatalf("expected at least 1 service detected")
	}

	svc := info.Services[0]
	if svc.Language != "Chrome Extension" {
		t.Errorf("expected Chrome Extension, got %s", svc.Language)
	}
	if svc.Framework != "Manifest V3" {
		t.Errorf("expected Manifest V3, got %s", svc.Framework)
	}
	if len(svc.Ports) == 0 || svc.Ports[0] != 80 {
		t.Errorf("expected port 80 for static web / extension preview, got %v", svc.Ports)
	}
}
