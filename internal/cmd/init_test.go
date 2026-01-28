package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"beads-lite/internal/config"
)

func TestInit(t *testing.T) {
	t.Run("creates beads directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Change working directory temporarily
		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		// Check .beads directory exists
		beadsPath := filepath.Join(tmpDir, ".beads")
		if _, err := os.Stat(beadsPath); os.IsNotExist(err) {
			t.Error(".beads directory was not created")
		}

		// Check config.yaml exists
		configPath := filepath.Join(beadsPath, "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error(".beads/config.yaml was not created")
		}

		// Check data directories exist
		dataPath := filepath.Join(beadsPath, "issues")
		openPath := filepath.Join(dataPath, "open")
		if _, err := os.Stat(openPath); os.IsNotExist(err) {
			t.Error(".beads/issues/open directory was not created")
		}

		// Check closed/ subdirectory exists
		closedPath := filepath.Join(dataPath, "closed")
		if _, err := os.Stat(closedPath); os.IsNotExist(err) {
			t.Error(".beads/issues/closed directory was not created")
		}
	})

	t.Run("fails if beads already exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .beads directory first
		beadsPath := filepath.Join(tmpDir, ".beads")
		if err := os.MkdirAll(beadsPath, 0755); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Change working directory temporarily
		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		if err == nil {
			t.Error("init command should have failed when .beads already exists")
		}
	})

	t.Run("force flag allows reinitialization", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .beads directory first
		beadsPath := filepath.Join(tmpDir, ".beads")
		if err := os.MkdirAll(beadsPath, 0755); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Change working directory temporarily
		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{"--force"})

		if err := cmd.Execute(); err != nil {
			t.Errorf("init command with --force should succeed: %v", err)
		}

		// Verify directories were created
		openPath := filepath.Join(beadsPath, "issues", "open")
		if _, err := os.Stat(openPath); os.IsNotExist(err) {
			t.Error(".beads/issues/open directory was not created after --force")
		}
	})

	t.Run("defaults to current directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Change to temp directory
		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getting current directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("changing directory: %v", err)
		}

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		// Check .beads was created in tmpDir
		beadsPath := filepath.Join(tmpDir, ".beads")
		if _, err := os.Stat(beadsPath); os.IsNotExist(err) {
			t.Error(".beads directory was not created in current directory")
		}
	})

	t.Run("uses project flag for data directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getting current directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("changing directory: %v", err)
		}

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{"--project", "work"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, ".beads", "config.yaml")
		cfg, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}
		if cfg.Project.Name != "work" {
			t.Errorf("config project name = %q, want %q", cfg.Project.Name, "work")
		}

		dataPath := filepath.Join(tmpDir, ".beads", "work")
		if _, err := os.Stat(dataPath); os.IsNotExist(err) {
			t.Error(".beads/work directory was not created")
		}
	})
}
