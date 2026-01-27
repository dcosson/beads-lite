package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolvePaths_ExplicitBase(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefault(filepath.Join(beadsDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}

	paths, cfg, err := ResolvePaths(beadsDir)
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}
	gotConfig, _ := filepath.EvalSymlinks(paths.ConfigDir)
	wantConfig, _ := filepath.EvalSymlinks(beadsDir)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, beadsDir)
	}
	gotData, _ := filepath.EvalSymlinks(paths.DataDir)
	wantData, _ := filepath.EvalSymlinks(filepath.Join(beadsDir, "issues"))
	if gotData != wantData {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, filepath.Join(beadsDir, "issues"))
	}
	if cfg.Project.Name != "issues" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "issues")
	}
}

func TestResolvePaths_SearchUpward(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefault(filepath.Join(beadsDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}

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

	paths, _, err := ResolvePaths("")
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}

	gotConfig := resolvePath(paths.ConfigDir)
	wantConfig := resolvePath(beadsDir)
	if gotConfig != wantConfig {
		t.Errorf("ConfigDir = %q, want %q", paths.ConfigDir, beadsDir)
	}
	gotData := resolvePath(paths.DataDir)
	wantData := resolvePath(filepath.Join(beadsDir, "issues"))
	if gotData != wantData {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, filepath.Join(beadsDir, "issues"))
	}
}

func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

func TestResolvePaths_CustomProjectName(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "work", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "work", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(beadsDir, "config.yaml")
	cfg := Default()
	cfg.Project.Name = "work"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	paths, loaded, err := ResolvePaths(beadsDir)
	if err != nil {
		t.Fatalf("ResolvePaths error: %v", err)
	}
	if loaded.Project.Name != "work" {
		t.Errorf("Project.Name = %q, want %q", loaded.Project.Name, "work")
	}
	if paths.DataDir != filepath.Join(beadsDir, "work") {
		t.Errorf("DataDir = %q, want %q", paths.DataDir, filepath.Join(beadsDir, "work"))
	}
}

func TestResolvePaths_MissingConfig(t *testing.T) {
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

	_, _, err = ResolvePaths("")
	if err == nil {
		t.Fatal("ResolvePaths should error when config is missing")
	}
	if !strings.Contains(err.Error(), "bd init") {
		t.Fatalf("expected error to mention bd init, got: %v", err)
	}
}

func TestResolvePaths_MissingDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefault(filepath.Join(beadsDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}

	_, _, err := ResolvePaths(beadsDir)
	if err == nil {
		t.Fatal("ResolvePaths should error when data dir is missing")
	}

	expected := "beads data not found at " + filepath.Join(beadsDir, "issues") + " (run `bd init`)"
	if err.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, err.Error())
	}
}
