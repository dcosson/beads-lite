package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/config/yamlstore"
)

// setupConfigTestApp creates an App with a fresh YAMLStore for config testing.
func setupConfigTestApp(t *testing.T) (*App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("BD_ACTOR", "")
	dir := t.TempDir()
	var out bytes.Buffer
	app := &App{
		ConfigDir: dir,
		Out:       &out,
	}
	return app, &out
}

// seedConfigStore creates a YAMLStore with the given key-value pairs.
func seedConfigStore(t *testing.T, dir string, pairs map[string]string) {
	t.Helper()
	store, err := yamlstore.New(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	for k, v := range pairs {
		if err := store.Set(k, v); err != nil {
			t.Fatalf("seeding store: %v", err)
		}
	}
}

func TestConfigGet_CoreKey(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor": "alice",
	})

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "alice" {
		t.Errorf("config get actor = %q, want %q", got, "alice")
	}
}

func TestConfigGet_CustomKey(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"custom.mykey": "myvalue",
	})

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"custom.mykey"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "myvalue" {
		t.Errorf("config get custom.mykey = %q, want %q", got, "myvalue")
	}
}

func TestConfigGet_NotSet(t *testing.T) {
	app, out := setupConfigTestApp(t)

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "actor (not set)" {
		t.Errorf("config get actor = %q, want %q", got, "actor (not set)")
	}
}

func TestConfigGet_JSON(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor": "bob",
	})

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["key"] != "actor" {
		t.Errorf("key = %v, want actor", result["key"])
	}
	if result["value"] != "bob" {
		t.Errorf("value = %v, want bob", result["value"])
	}
	if result["location"] != "config.yaml" {
		t.Errorf("location = %v, want config.yaml", result["location"])
	}
}

func TestConfigGet_JSON_NotSet(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true

	cmd := newConfigGetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config get --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["value"] != "" {
		t.Errorf("value = %v, want empty string", result["value"])
	}
}

func TestConfigSet_CoreKey(t *testing.T) {
	app, out := setupConfigTestApp(t)

	cmd := newConfigSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Set actor = alice" {
		t.Errorf("output = %q, want %q", got, "Set actor = alice")
	}

	// Verify persistence
	store, err := yamlstore.New(filepath.Join(app.ConfigDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := store.Get("actor")
	if !ok || v != "alice" {
		t.Errorf("persisted value = %q, %v; want %q, true", v, ok, "alice")
	}
}

func TestConfigSet_CustomKey(t *testing.T) {
	app, out := setupConfigTestApp(t)

	cmd := newConfigSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"custom.foo", "bar"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Set custom.foo = bar" {
		t.Errorf("output = %q, want %q", got, "Set custom.foo = bar")
	}
}

func TestConfigSet_JSON(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true

	cmd := newConfigSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor", "charlie"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config set --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["key"] != "actor" {
		t.Errorf("key = %v, want actor", result["key"])
	}
	if result["value"] != "charlie" {
		t.Errorf("value = %v, want charlie", result["value"])
	}
	if result["location"] != "config.yaml" {
		t.Errorf("location = %v, want config.yaml", result["location"])
	}
}

func TestConfigUnset_CoreKey(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor": "alice",
	})

	cmd := newConfigUnsetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Unset actor" {
		t.Errorf("output = %q, want %q", got, "Unset actor")
	}

	// Verify removal
	store, err := yamlstore.New(filepath.Join(app.ConfigDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get("actor"); ok {
		t.Error("actor still set after unset")
	}
}

func TestConfigUnset_CustomKey(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"custom.key": "val",
	})

	cmd := newConfigUnsetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"custom.key"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Unset custom.key" {
		t.Errorf("output = %q, want %q", got, "Unset custom.key")
	}
}

func TestConfigUnset_JSON(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true

	cmd := newConfigUnsetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"actor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config unset --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["key"] != "actor" {
		t.Errorf("key = %v, want actor", result["key"])
	}
}

func TestConfigList_Empty(t *testing.T) {
	app, out := setupConfigTestApp(t)

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); !strings.HasPrefix(got, "Configuration:") {
		t.Errorf("output = %q, want Configuration header", got)
	}
}

func TestConfigList_WithEntries(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor":             "alice",
		"defaults.priority": "high",
	})

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Configuration:") {
		t.Errorf("missing header in: %s", output)
	}
	if !strings.Contains(output, "  actor = alice") {
		t.Errorf("missing actor line in: %s", output)
	}
	if !strings.Contains(output, "  defaults.priority = high") {
		t.Errorf("missing priority line in: %s", output)
	}

	// Verify sorted order: actor comes before defaults.priority
	actorIdx := strings.Index(output, "  actor = alice")
	priorityIdx := strings.Index(output, "  defaults.priority = high")
	if actorIdx >= priorityIdx {
		t.Errorf("entries not sorted: actor at %d, defaults.priority at %d", actorIdx, priorityIdx)
	}
}

func TestConfigList_JSON_Empty(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["compact_model"]; !ok {
		t.Errorf("expected compact_model in result, got %v", result)
	}
}

func TestConfigList_JSON_WithEntries(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor":             "alice",
		"defaults.priority": "high",
	})

	cmd := newConfigListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config list --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["actor"] != "alice" {
		t.Errorf("actor = %q, want alice", result["actor"])
	}
	if result["defaults.priority"] != "high" {
		t.Errorf("defaults.priority = %q, want high", result["defaults.priority"])
	}
}

func TestConfigValidate_Clean(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor":             "alice",
		"defaults.priority": "high",
		"defaults.type":     "task",
		"id.length":         "4",
	})

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Configuration is valid." {
		t.Errorf("output = %q, want %q", got, "Configuration is valid.")
	}
}

func TestConfigValidate_Empty(t *testing.T) {
	app, out := setupConfigTestApp(t)

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate failed: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Configuration is valid." {
		t.Errorf("output = %q, want %q", got, "Configuration is valid.")
	}
}

func TestConfigValidate_Errors(t *testing.T) {
	app, _ := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"defaults.priority": "invalid_priority",
		"defaults.type":     "invalid_type",
		"id.length":         "abc",
	})

	cmd := newConfigValidateCmd(NewTestProvider(app))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("config validate should have failed")
	}
	if !strings.Contains(err.Error(), "3 error(s)") {
		t.Errorf("expected 3 errors, got: %v", err)
	}
}

func TestConfigValidate_JSON_Valid(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"actor": "alice",
	})

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate --json failed: %v", err)
	}

	var result struct {
		Valid  bool     `json:"valid"`
		Issues []string `json:"issues"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !result.Valid {
		t.Error("valid = false, want true")
	}
}

func TestConfigValidate_JSON_Errors(t *testing.T) {
	app, out := setupConfigTestApp(t)
	app.JSON = true
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"defaults.priority": "bogus",
	})

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate --json failed: %v", err)
	}

	var result struct {
		Valid  bool     `json:"valid"`
		Issues []string `json:"issues"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.Valid {
		t.Error("valid = true, want false")
	}
	if len(result.Issues) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Issues))
	}
}

func TestConfigValidate_CustomKeysIgnored(t *testing.T) {
	app, out := setupConfigTestApp(t)
	seedConfigStore(t, app.ConfigDir, map[string]string{
		"custom.anything": "any value is fine",
	})

	cmd := newConfigValidateCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate failed for custom key: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "Configuration is valid." {
		t.Errorf("output = %q, want %q", got, "Configuration is valid.")
	}
}
