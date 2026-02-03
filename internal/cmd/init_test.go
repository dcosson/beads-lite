package cmd

import (
	"bytes"
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
		for _, key := range []string{"actor", "defaults.priority", "defaults.type", "issue_prefix", "project.name"} {
			if _, ok := store.Get(key); !ok {
				t.Errorf("config missing key %q", key)
			}
		}
	})

	t.Run("uses prefix flag for issue_prefix config", func(t *testing.T) {
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
		cmd.SetArgs([]string{"--prefix", "myp-"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, ".beads", "config.yaml")
		store, err := yamlstore.New(configPath)
		if err != nil {
			t.Fatalf("loading config store: %v", err)
		}
		if v, _ := store.Get("issue_prefix"); v != "myp-" {
			t.Errorf("config issue_prefix = %q, want %q", v, "myp-")
		}
	})

	t.Run("appends dash to prefix if missing", func(t *testing.T) {
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
		cmd.SetArgs([]string{"--prefix", "myp"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, ".beads", "config.yaml")
		store, err := yamlstore.New(configPath)
		if err != nil {
			t.Fatalf("loading config store: %v", err)
		}
		if v, _ := store.Get("issue_prefix"); v != "myp-" {
			t.Errorf("config issue_prefix = %q, want %q", v, "myp-")
		}
	})

	t.Run("defaults to directory name prefix when flag not provided", func(t *testing.T) {
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
		expected := filepath.Base(tmpDir) + "-"
		if v, _ := store.Get("issue_prefix"); v != expected {
			t.Errorf("config issue_prefix = %q, want %q", v, expected)
		}
	})

	t.Run("extracts prefix from existing issues on reinit", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .beads dir with issue files but no config.yaml
		beadsPath := filepath.Join(tmpDir, ".beads")
		openPath := filepath.Join(beadsPath, "issues", "open")
		if err := os.MkdirAll(openPath, 0755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(openPath, "proj-abc.json"),
			[]byte(`{"id":"proj-abc","title":"test","status":"open"}`), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

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
		cmd.SetArgs([]string{"--force"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		configPath := filepath.Join(beadsPath, "config.yaml")
		store, err := yamlstore.New(configPath)
		if err != nil {
			t.Fatalf("loading config store: %v", err)
		}
		if v, _ := store.Get("issue_prefix"); v != "proj-" {
			t.Errorf("config issue_prefix = %q, want %q", v, "proj-")
		}
	})

	t.Run("preserves existing config prefix on reinit", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getting current directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("changing directory: %v", err)
		}

		// First init with explicit prefix
		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{"--prefix", "myp-"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("first init failed: %v", err)
		}

		// Re-init without prefix flag
		cmd2 := newInitCmd(provider)
		cmd2.SetArgs([]string{"--force"})
		if err := cmd2.Execute(); err != nil {
			t.Fatalf("reinit failed: %v", err)
		}

		configPath := filepath.Join(tmpDir, ".beads", "config.yaml")
		store, err := yamlstore.New(configPath)
		if err != nil {
			t.Fatalf("loading config store: %v", err)
		}
		if v, _ := store.Get("issue_prefix"); v != "myp-" {
			t.Errorf("config issue_prefix = %q, want %q", v, "myp-")
		}
	})

	t.Run("quiet flag suppresses output", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getting current directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("changing directory: %v", err)
		}

		var out bytes.Buffer
		provider := &AppProvider{
			Out: &out,
		}

		rootCmd := newRootCmd(provider)
		rootCmd.SetArgs([]string{"--quiet", "init"})
		rootCmd.SetOut(&out)

		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("init command with --quiet failed: %v", err)
		}

		if out.Len() != 0 {
			t.Errorf("--quiet should suppress output, got: %q", out.String())
		}

		// Verify .beads dir was still created
		beadsPath := filepath.Join(tmpDir, ".beads")
		if _, err := os.Stat(beadsPath); os.IsNotExist(err) {
			t.Error(".beads directory was not created")
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
