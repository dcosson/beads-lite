package config

import (
	"os"
	"path/filepath"
	"testing"

	"beads2/storage"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Defaults.Priority != storage.PriorityMedium {
		t.Errorf("expected priority %s, got %s", storage.PriorityMedium, cfg.Defaults.Priority)
	}
	if cfg.Defaults.Type != storage.TypeTask {
		t.Errorf("expected type %s, got %s", storage.TypeTask, cfg.Defaults.Type)
	}
	if cfg.ID.Prefix != "bd-" {
		t.Errorf("expected prefix 'bd-', got '%s'", cfg.ID.Prefix)
	}
	if cfg.ID.Length != 4 {
		t.Errorf("expected length 4, got %d", cfg.ID.Length)
	}
	if cfg.Actor != "$USER" {
		t.Errorf("expected actor '$USER', got '%s'", cfg.Actor)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(beadsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return defaults with env vars expanded
	if cfg.Defaults.Priority != storage.PriorityMedium {
		t.Errorf("expected priority %s, got %s", storage.PriorityMedium, cfg.Defaults.Priority)
	}
	if cfg.Defaults.Type != storage.TypeTask {
		t.Errorf("expected type %s, got %s", storage.TypeTask, cfg.Defaults.Type)
	}

	// Actor should be expanded from $USER
	expectedUser := os.Getenv("USER")
	if cfg.Actor != expectedUser {
		t.Errorf("expected actor '%s', got '%s'", expectedUser, cfg.Actor)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `
defaults:
  priority: high
  type: bug
id:
  prefix: test-
  length: 6
actor: testuser
`
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(beadsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Defaults.Priority != storage.PriorityHigh {
		t.Errorf("expected priority %s, got %s", storage.PriorityHigh, cfg.Defaults.Priority)
	}
	if cfg.Defaults.Type != storage.TypeBug {
		t.Errorf("expected type %s, got %s", storage.TypeBug, cfg.Defaults.Type)
	}
	if cfg.ID.Prefix != "test-" {
		t.Errorf("expected prefix 'test-', got '%s'", cfg.ID.Prefix)
	}
	if cfg.ID.Length != 6 {
		t.Errorf("expected length 6, got %d", cfg.ID.Length)
	}
	if cfg.Actor != "testuser" {
		t.Errorf("expected actor 'testuser', got '%s'", cfg.Actor)
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_VAR", "testvalue")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "dollar syntax",
			input:    "$TEST_VAR",
			expected: "testvalue",
		},
		{
			name:     "braces syntax",
			input:    "${TEST_VAR}",
			expected: "testvalue",
		},
		{
			name:     "mixed with text",
			input:    "prefix-${TEST_VAR}-suffix",
			expected: "prefix-testvalue-suffix",
		},
		{
			name:     "USER variable",
			input:    "$USER",
			expected: os.Getenv("USER"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnv(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	os.Setenv("TEST_ACTOR", "envuser")
	defer os.Unsetenv("TEST_ACTOR")

	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `
actor: ${TEST_ACTOR}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(beadsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Actor != "envuser" {
		t.Errorf("expected actor 'envuser', got '%s'", cfg.Actor)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(beadsDir)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}
