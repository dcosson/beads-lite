package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFindBeadsDir_ExplicitPath(t *testing.T) {
	// Create a temp directory with .beads
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// FindBeadsDir with explicit path should return that path
	result, err := FindBeadsDir(beadsDir)
	if err != nil {
		t.Fatalf("FindBeadsDir(%q) error: %v", beadsDir, err)
	}
	if result != beadsDir {
		t.Errorf("FindBeadsDir(%q) = %q, want %q", beadsDir, result, beadsDir)
	}
}

func TestFindBeadsDir_ExplicitPath_NotExist(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent")

	_, err := FindBeadsDir(nonExistent)
	if err == nil {
		t.Error("FindBeadsDir with non-existent path should return error")
	}
}

func TestFindBeadsDir_ExplicitPath_NotDir(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "afile")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := FindBeadsDir(filePath)
	if err == nil {
		t.Error("FindBeadsDir with file path should return error")
	}
}

func TestFindBeadsDir_WalkUp(t *testing.T) {
	// Create directory structure: tmpDir/.beads and tmpDir/a/b/c
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to deep directory and search for .beads
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(deepDir); err != nil {
		t.Fatal(err)
	}

	result, err := FindBeadsDir("")
	if err != nil {
		t.Fatalf("FindBeadsDir(\"\") error: %v", err)
	}

	// Resolve symlinks for comparison (e.g., /var -> /private/var on macOS)
	wantResolved, _ := filepath.EvalSymlinks(beadsDir)
	gotResolved, _ := filepath.EvalSymlinks(result)
	if gotResolved != wantResolved {
		t.Errorf("FindBeadsDir(\"\") = %q, want %q", result, beadsDir)
	}
}

func TestFindBeadsDir_NotFound(t *testing.T) {
	// Create a directory with no .beads anywhere in the tree
	tmpDir := t.TempDir()
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(deepDir); err != nil {
		t.Fatal(err)
	}

	_, err = FindBeadsDir("")
	if err == nil {
		t.Error("FindBeadsDir should return error when no .beads found")
	}
}

func TestAppProvider_Get(t *testing.T) {
	// Create a temp .beads directory with required structure
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "closed"), 0755); err != nil {
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
