package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"beads-lite/internal/config"
)

func TestAppProvider_Get(t *testing.T) {
	// Create a temp .beads directory with required structure
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteDefault(filepath.Join(beadsDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		BeadsPath:  beadsDir,
		JSONOutput: true,
		Out:        &out,
		Err:        &errOut,
	}

	app, err := provider.Get()
	if err != nil {
		t.Fatalf("provider.Get() error: %v", err)
	}

	if app.Storage == nil {
		t.Error("App.Storage should not be nil")
	}
	if app.Out != &out {
		t.Error("App.Out not set correctly")
	}
	if app.Err != &errOut {
		t.Error("App.Err not set correctly")
	}
	if !app.JSON {
		t.Error("App.JSON should be true")
	}
	if app.Config.Project.Name != "issues" {
		t.Errorf("App.Config.Project.Name = %q, want %q", app.Config.Project.Name, "issues")
	}

	// Second call should return same app (lazy init)
	app2, err := provider.Get()
	if err != nil {
		t.Fatalf("second provider.Get() error: %v", err)
	}
	if app2 != app {
		t.Error("provider.Get() should return same app on second call")
	}
}

func TestAppProvider_Get_InvalidPath(t *testing.T) {
	provider := &AppProvider{
		BeadsPath: "/nonexistent/path",
	}

	_, err := provider.Get()
	if err == nil {
		t.Error("provider.Get() with invalid path should return error")
	}
}

func TestNewTestProvider(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out:  &out,
		JSON: true,
	}

	provider := NewTestProvider(app)
	gotApp, err := provider.Get()
	if err != nil {
		t.Fatalf("NewTestProvider().Get() error: %v", err)
	}
	if gotApp != app {
		t.Error("NewTestProvider should return the provided app")
	}
}
