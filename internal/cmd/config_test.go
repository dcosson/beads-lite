package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/config"
	"beads-lite/internal/config/yamlstore"
)

func setupConfigTestApp(t *testing.T) (*App, config.Store) {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "store.yaml")
	store, err := yamlstore.New(storePath)
	if err != nil {
		t.Fatalf("failed to create config store: %v", err)
	}
	if err := config.ApplyDefaults(store); err != nil {
		t.Fatalf("failed to apply defaults: %v", err)
	}
	return &App{
		ConfigStore: store,
		Out:         &bytes.Buffer{},
		Err:         &bytes.Buffer{},
	}, store
}

func TestConfigGetCoreKey(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"defaults.priority"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "medium" {
		t.Errorf("expected %q, got %q", "medium", output)
	}
}

func TestConfigGetMissingKey(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent.key"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "nonexistent.key (not set)" {
		t.Errorf("expected %q, got %q", "nonexistent.key (not set)", output)
	}
}

func TestConfigGetJSON(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"defaults.priority"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["key"] != "defaults.priority" {
		t.Errorf("expected key %q, got %q", "defaults.priority", result["key"])
	}
	if result["value"] != "medium" {
		t.Errorf("expected value %q, got %q", "medium", result["value"])
	}
}

func TestConfigGetMissingJSON(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["key"] != "nonexistent" {
		t.Errorf("expected key %q, got %q", "nonexistent", result["key"])
	}
	if result["set"] != false {
		t.Errorf("expected set=false, got %v", result["set"])
	}
}

func TestConfigSetCoreKey(t *testing.T) {
	app, store := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"defaults.priority", "high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "Set defaults.priority = high" {
		t.Errorf("expected %q, got %q", "Set defaults.priority = high", output)
	}

	val, ok := store.Get("defaults.priority")
	if !ok || val != "high" {
		t.Errorf("expected store to have defaults.priority=high, got %q (ok=%v)", val, ok)
	}
}

func TestConfigSetCustomKey(t *testing.T) {
	app, store := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"custom.key", "myvalue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "Set custom.key = myvalue" {
		t.Errorf("expected %q, got %q", "Set custom.key = myvalue", output)
	}

	val, ok := store.Get("custom.key")
	if !ok || val != "myvalue" {
		t.Errorf("expected store to have custom.key=myvalue, got %q (ok=%v)", val, ok)
	}
}

func TestConfigSetJSON(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"defaults.type", "bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["key"] != "defaults.type" {
		t.Errorf("expected key %q, got %q", "defaults.type", result["key"])
	}
	if result["value"] != "bug" {
		t.Errorf("expected value %q, got %q", "bug", result["value"])
	}
	if result["status"] != "set" {
		t.Errorf("expected status %q, got %q", "set", result["status"])
	}
}

func TestConfigUnsetCoreKey(t *testing.T) {
	app, store := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigUnsetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "Unset actor" {
		t.Errorf("expected %q, got %q", "Unset actor", output)
	}

	_, ok := store.Get("actor")
	if ok {
		t.Error("expected actor to be unset")
	}
}

func TestConfigUnsetCustomKey(t *testing.T) {
	app, store := setupConfigTestApp(t)

	// Set then unset a custom key
	if err := store.Set("custom.key", "val"); err != nil {
		t.Fatalf("failed to set custom key: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newConfigUnsetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"custom.key"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "Unset custom.key" {
		t.Errorf("expected %q, got %q", "Unset custom.key", output)
	}

	_, ok := store.Get("custom.key")
	if ok {
		t.Error("expected custom.key to be unset")
	}
}

func TestConfigUnsetJSON(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigUnsetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config unset --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["key"] != "actor" {
		t.Errorf("expected key %q, got %q", "actor", result["key"])
	}
	if result["status"] != "unset" {
		t.Errorf("expected status %q, got %q", "unset", result["status"])
	}
}

func TestConfigListWithDefaults(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list failed: %v", err)
	}

	output := out.String()
	if !strings.HasPrefix(output, "Configuration:\n") {
		t.Errorf("expected output to start with 'Configuration:', got %q", output)
	}
	if !strings.Contains(output, "  defaults.priority = medium") {
		t.Errorf("expected output to contain defaults.priority, got %q", output)
	}
	if !strings.Contains(output, "  id.prefix = bd-") {
		t.Errorf("expected output to contain id.prefix, got %q", output)
	}
}

func TestConfigListEmpty(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "store.yaml")
	store, err := yamlstore.New(storePath)
	if err != nil {
		t.Fatalf("failed to create config store: %v", err)
	}
	app := &App{
		ConfigStore: store,
		Out:         &bytes.Buffer{},
		Err:         &bytes.Buffer{},
	}
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "No configuration set" {
		t.Errorf("expected %q, got %q", "No configuration set", output)
	}
}

func TestConfigListJSON(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["defaults.priority"] != "medium" {
		t.Errorf("expected defaults.priority=medium, got %q", result["defaults.priority"])
	}
	if result["id.prefix"] != "bd-" {
		t.Errorf("expected id.prefix=bd-, got %q", result["id.prefix"])
	}
}

func TestConfigValidateClean(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	if output != "Configuration is valid." {
		t.Errorf("expected %q, got %q", "Configuration is valid.", output)
	}
}

func TestConfigValidateErrors(t *testing.T) {
	app, store := setupConfigTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Set an invalid value
	if err := store.Set("defaults.priority", "invalid"); err != nil {
		t.Fatalf("failed to set invalid value: %v", err)
	}

	cmd := newConfigValidateCmd(NewTestProvider(app))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected error to mention validation, got %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "defaults.priority") {
		t.Errorf("expected output to mention defaults.priority, got %q", output)
	}
}

func TestConfigValidateCleanJSON(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["valid"] != true {
		t.Errorf("expected valid=true, got %v", result["valid"])
	}
}

func TestConfigValidateErrorsJSON(t *testing.T) {
	app, store := setupConfigTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	if err := store.Set("defaults.priority", "invalid"); err != nil {
		t.Fatalf("failed to set invalid value: %v", err)
	}

	cmd := newConfigValidateCmd(NewTestProvider(app))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["valid"] != false {
		t.Errorf("expected valid=false, got %v", result["valid"])
	}
	errors, ok := result["errors"].([]interface{})
	if !ok || len(errors) == 0 {
		t.Errorf("expected non-empty errors array, got %v", result["errors"])
	}
}
