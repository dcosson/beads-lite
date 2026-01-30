package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"beads-lite/internal/config/yamlstore"
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

		// Check formulas/ directory exists
		formulasPath := filepath.Join(beadsPath, "formulas")
		if _, err := os.Stat(formulasPath); os.IsNotExist(err) {
			t.Error(".beads/formulas directory was not created")
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
		store, err := yamlstore.New(configPath)
		if err != nil {
			t.Fatalf("loading config store: %v", err)
		}
		if v, _ := store.Get("project.name"); v != "work" {
			t.Errorf("config project.name = %q, want %q", v, "work")
		}

		dataPath := filepath.Join(tmpDir, ".beads", "work")
		if _, err := os.Stat(dataPath); os.IsNotExist(err) {
			t.Error(".beads/work directory was not created")
		}
	})

	t.Run("creates flat config format", func(t *testing.T) {
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
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, ".beads", "config.yaml")
		store, err := yamlstore.New(configPath)
		if err != nil {
			t.Fatalf("loading config store: %v", err)
		}

		// Verify all default keys are present as flat key-value pairs
		for _, key := range []string{"actor", "defaults.priority", "defaults.type", "id.prefix", "id.length", "project.name"} {
			if _, ok := store.Get(key); !ok {
				t.Errorf("config missing key %q", key)
			}
		}
	})

	t.Run("BEADS_DIR env var", func(t *testing.T) {
		targetDir := t.TempDir()
		t.Setenv("BEADS_DIR", targetDir)

		// cd to a different temp dir
		otherDir := t.TempDir()
		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getting current directory: %v", err)
		}
		defer os.Chdir(oldWd)
		if err := os.Chdir(otherDir); err != nil {
			t.Fatalf("changing directory: %v", err)
		}

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		// .beads should be created in targetDir, not otherDir
		beadsPath := filepath.Join(targetDir, ".beads")
		if _, err := os.Stat(beadsPath); os.IsNotExist(err) {
			t.Error(".beads directory was not created in BEADS_DIR target")
		}
		// Verify not created in otherDir
		otherBeads := filepath.Join(otherDir, ".beads")
		if _, err := os.Stat(otherBeads); err == nil {
			t.Error(".beads directory should NOT be created in cwd when BEADS_DIR is set")
		}
	})

}
