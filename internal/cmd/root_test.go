package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAppProvider_Get(t *testing.T) {
	// Create a temp .beads directory with required structure
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)

	t.Setenv("BEADS_DIR", beadsDir)

	var out, errOut bytes.Buffer
	provider := &AppProvider{
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
	if v, _ := app.ConfigStore.Get("project.name"); v != "issues" {
		t.Errorf("ConfigStore project.name = %q, want %q", v, "issues")
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

func TestAppProvider_Get_InvalidBEADS_DIR(t *testing.T) {
	t.Setenv("BEADS_DIR", "/nonexistent/path")

	provider := &AppProvider{}

	_, err := provider.Get()
	if err == nil {
		t.Error("provider.Get() with invalid BEADS_DIR should return error")
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

func setupBeadsDir(t *testing.T, parentDir string) string {
	t.Helper()
	beadsDir := filepath.Join(parentDir, ".beads")
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "open"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "issues", "closed"), 0755); err != nil {
		t.Fatal(err)
	}
	// Write flat key-value config
	content := "actor: ${USER}\ndefaults.priority: medium\ndefaults.type: task\nid.prefix: bd-\nproject.name: issues\n"
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return beadsDir
}

func TestAppProvider_Get_WithBEADS_DIR(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)

	t.Setenv("BEADS_DIR", beadsDir)

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	app, err := provider.Get()
	if err != nil {
		t.Fatalf("provider.Get() with BEADS_DIR error: %v", err)
	}
	if app.Storage == nil {
		t.Error("App.Storage should not be nil")
	}
	if v, _ := app.ConfigStore.Get("project.name"); v != "issues" {
		t.Errorf("ConfigStore project.name = %q, want %q", v, "issues")
	}
}

func TestAppProvider_Get_EnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)

	t.Setenv("BEADS_DIR", beadsDir)
	t.Setenv("BD_ACTOR", "env-actor")
	t.Setenv("BD_PROJECT", "issues") // keep same project name so data dir exists

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	app, err := provider.Get()
	if err != nil {
		t.Fatalf("provider.Get() error: %v", err)
	}
	if v, _ := app.ConfigStore.Get("actor"); v != "env-actor" {
		t.Errorf("ConfigStore actor = %q, want %q", v, "env-actor")
	}
}

func TestAppProvider_Get_BD_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)

	t.Setenv("BEADS_DIR", beadsDir)
	t.Setenv("BD_JSON", "1")

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	// Use the root command to test PersistentPreRunE
	rootCmd := newRootCmd(provider)
	rootCmd.SetArgs([]string{"list"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)

	// Execute will run PersistentPreRunE which should pick up BD_JSON
	_ = rootCmd.Execute()

	// The provider should have JSONOutput set to true
	if !provider.JSONOutput {
		t.Error("provider.JSONOutput should be true when BD_JSON=1")
	}
}

func TestQuietFlag_SuppressesOutput(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)
	t.Setenv("BEADS_DIR", beadsDir)

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	rootCmd := newRootCmd(provider)
	rootCmd.SetArgs([]string{"--quiet", "version"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("command with --quiet should not error, got: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("--quiet should suppress output, got: %q", out.String())
	}
}

func TestBD_QUIET_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)
	t.Setenv("BEADS_DIR", beadsDir)
	t.Setenv("BD_QUIET", "1")

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	rootCmd := newRootCmd(provider)
	rootCmd.SetArgs([]string{"version"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("command with BD_QUIET=1 should not error, got: %v", err)
	}

	if !provider.Quiet {
		t.Error("provider.Quiet should be true when BD_QUIET=1")
	}

	if out.Len() != 0 {
		t.Errorf("BD_QUIET=1 should suppress output, got: %q", out.String())
	}
}

func TestCompatibilityFlags_Accepted(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)
	t.Setenv("BEADS_DIR", beadsDir)

	flags := []string{
		"--no-daemon",
		"--no-auto-flush",
		"--no-auto-import",
		"--no-db",
		"--lock-timeout=5s",
		"--sandbox",
		"--readonly",
		"--allow-stale",
	}

	for _, flag := range flags {
		t.Run(flag, func(t *testing.T) {
			var out, errOut bytes.Buffer
			provider := &AppProvider{
				Out: &out,
				Err: &errOut,
			}

			rootCmd := newRootCmd(provider)
			rootCmd.SetArgs([]string{flag, "list"})
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&errOut)

			if err := rootCmd.Execute(); err != nil {
				t.Errorf("command with %s should not error, got: %v", flag, err)
			}
		})
	}
}

func TestCompatibilityFlags_Combined(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)
	t.Setenv("BEADS_DIR", beadsDir)

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	rootCmd := newRootCmd(provider)
	rootCmd.SetArgs([]string{"--no-daemon", "--sandbox", "--allow-stale", "list"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)

	if err := rootCmd.Execute(); err != nil {
		t.Errorf("command with combined compat flags should not error, got: %v", err)
	}
}

func TestAppProvider_Get_BD_JSON_FlagOverridesEnv(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := setupBeadsDir(t, tmpDir)

	// Set BD_JSON but also pass --json=false explicitly
	t.Setenv("BEADS_DIR", beadsDir)
	t.Setenv("BD_JSON", "1")

	var out, errOut bytes.Buffer
	provider := &AppProvider{
		Out: &out,
		Err: &errOut,
	}

	rootCmd := newRootCmd(provider)
	rootCmd.SetArgs([]string{"--json=false", "list"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)

	_ = rootCmd.Execute()

	// When --json is explicitly passed, env var should NOT override
	if provider.JSONOutput {
		t.Error("provider.JSONOutput should be false when --json=false is explicitly passed")
	}
}
