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

	t.Run("succeeds if beads directory exists but is empty", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create empty .beads directory
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

		if err := cmd.Execute(); err != nil {
			t.Errorf("init should succeed with empty .beads directory: %v", err)
		}

		// Verify directories were created
		openPath := filepath.Join(beadsPath, "issues", "open")
		if _, err := os.Stat(openPath); os.IsNotExist(err) {
			t.Error(".beads/issues/open directory was not created")
		}
	})

	t.Run("fails if beads already exists and is non-empty", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .beads directory with a file in it
		beadsPath := filepath.Join(tmpDir, ".beads")
		if err := os.MkdirAll(beadsPath, 0755); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(beadsPath, "config.yaml"), []byte("test"), 0644); err != nil {
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
			t.Error("init command should have failed when .beads already exists with content")
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
		for _, key := range []string{"actor", "defaults.priority", "defaults.type", "issue_prefix"} {
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
		if v, _ := store.Get("issue_prefix"); v != "myp" {
			t.Errorf("config issue_prefix = %q, want %q", v, "myp")
		}
	})

	t.Run("strips dash from prefix flag", func(t *testing.T) {
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
		if v, _ := store.Get("issue_prefix"); v != "myp" {
			t.Errorf("config issue_prefix = %q, want %q", v, "myp")
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
		expected := filepath.Base(tmpDir)
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
		if v, _ := store.Get("issue_prefix"); v != "proj" {
			t.Errorf("config issue_prefix = %q, want %q", v, "proj")
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
		if v, _ := store.Get("issue_prefix"); v != "myp" {
			t.Errorf("config issue_prefix = %q, want %q", v, "myp")
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

	t.Run("creates gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		gitignorePath := filepath.Join(tmpDir, ".beads", ".gitignore")
		data, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf(".gitignore not created: %v", err)
		}
		content := string(data)
		if content != "issues/ephemeral/\n*.lock\n" {
			t.Errorf(".gitignore content = %q, want %q", content, "issues/ephemeral/\n*.lock\n")
		}
	})

	t.Run("creates ephemeral directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		ephemeralPath := filepath.Join(tmpDir, ".beads", "issues", "ephemeral")
		info, err := os.Stat(ephemeralPath)
		if err != nil {
			t.Fatalf("ephemeral directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("ephemeral path is not a directory")
		}
	})

	t.Run("creates kv stores as siblings of issues", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldWd)

		provider := &AppProvider{}
		cmd := newInitCmd(provider)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("init command failed: %v", err)
		}

		beadsPath := filepath.Join(tmpDir, ".beads")
		for _, dir := range []string{"slots", "agents", "merge-slot"} {
			dirPath := filepath.Join(beadsPath, dir)
			info, err := os.Stat(dirPath)
			if err != nil {
				t.Errorf("%s directory not created: %v", dir, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("%s is not a directory", dir)
			}
		}

		// Verify they are NOT inside issues/
		issuesPath := filepath.Join(beadsPath, "issues")
		for _, dir := range []string{"slots", "agents", "merge-slot"} {
			if _, err := os.Stat(filepath.Join(issuesPath, dir)); err == nil {
				t.Errorf("%s should NOT be inside issues/, should be a sibling", dir)
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
