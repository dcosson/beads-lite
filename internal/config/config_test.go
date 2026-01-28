package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithFallback(t *testing.T) {
	// Create primary config with minimal values
	primaryDir := t.TempDir()
	primaryPath := filepath.Join(primaryDir, "config.yaml")
	primaryCfg := Config{
		Actor: "primary-actor",
		Project: ProjectConfig{
			Name: "primary-project",
		},
	}
	if err := Write(primaryPath, primaryCfg); err != nil {
		t.Fatal(err)
	}

	// Create fallback config at ~/.config/bd/config.yaml
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "") // reset so it uses HOME

	fallbackDir := filepath.Join(homeDir, ".config", "bd")
	if err := os.MkdirAll(fallbackDir, 0755); err != nil {
		t.Fatal(err)
	}
	fallbackPath := filepath.Join(fallbackDir, "config.yaml")
	fallbackCfg := Config{
		Actor: "fallback-actor", // should NOT override primary
		Defaults: DefaultsConfig{
			Priority: "high",
			Type:     "bug",
		},
		ID: IDConfig{
			Prefix: "fb-",
			Length: 6,
		},
	}
	if err := Write(fallbackPath, fallbackCfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadWithFallback(primaryPath)
	if err != nil {
		t.Fatalf("LoadWithFallback error: %v", err)
	}

	// Primary values should be preserved
	if loaded.Actor != "primary-actor" {
		t.Errorf("Actor = %q, want %q", loaded.Actor, "primary-actor")
	}
	if loaded.Project.Name != "primary-project" {
		t.Errorf("Project.Name = %q, want %q", loaded.Project.Name, "primary-project")
	}
}

func TestMergeConfig(t *testing.T) {
	primary := Config{
		Actor: "alice",
		// Other fields left at zero values
	}
	fallback := Config{
		Actor: "bob",
		Defaults: DefaultsConfig{
			Priority: "high",
			Type:     "bug",
		},
		ID: IDConfig{
			Prefix: "fb-",
			Length: 8,
		},
		Project: ProjectConfig{
			Name: "fallback-project",
		},
	}

	merged := mergeConfig(primary, fallback)

	// Primary non-zero values should be preserved
	if merged.Actor != "alice" {
		t.Errorf("Actor = %q, want %q", merged.Actor, "alice")
	}

	// Zero-value fields should be filled from fallback
	if merged.Defaults.Priority != "high" {
		t.Errorf("Defaults.Priority = %q, want %q", merged.Defaults.Priority, "high")
	}
	if merged.Defaults.Type != "bug" {
		t.Errorf("Defaults.Type = %q, want %q", merged.Defaults.Type, "bug")
	}
	if merged.ID.Prefix != "fb-" {
		t.Errorf("ID.Prefix = %q, want %q", merged.ID.Prefix, "fb-")
	}
	if merged.ID.Length != 8 {
		t.Errorf("ID.Length = %d, want %d", merged.ID.Length, 8)
	}
	if merged.Project.Name != "fallback-project" {
		t.Errorf("Project.Name = %q, want %q", merged.Project.Name, "fallback-project")
	}
}

func TestMergeConfig_PrimaryFullyPopulated(t *testing.T) {
	primary := Config{
		Actor: "alice",
		Defaults: DefaultsConfig{
			Priority: "low",
			Type:     "feature",
		},
		ID: IDConfig{
			Prefix: "pr-",
			Length: 4,
		},
		Project: ProjectConfig{
			Name: "primary",
		},
	}
	fallback := Config{
		Actor: "bob",
		Defaults: DefaultsConfig{
			Priority: "high",
			Type:     "bug",
		},
		ID: IDConfig{
			Prefix: "fb-",
			Length: 8,
		},
		Project: ProjectConfig{
			Name: "fallback",
		},
	}

	merged := mergeConfig(primary, fallback)

	// All values should come from primary
	if merged.Actor != "alice" {
		t.Errorf("Actor = %q, want %q", merged.Actor, "alice")
	}
	if merged.Defaults.Priority != "low" {
		t.Errorf("Defaults.Priority = %q, want %q", merged.Defaults.Priority, "low")
	}
	if merged.ID.Prefix != "pr-" {
		t.Errorf("ID.Prefix = %q, want %q", merged.ID.Prefix, "pr-")
	}
	if merged.ID.Length != 4 {
		t.Errorf("ID.Length = %d, want %d", merged.ID.Length, 4)
	}
	if merged.Project.Name != "primary" {
		t.Errorf("Project.Name = %q, want %q", merged.Project.Name, "primary")
	}
}

func TestConfigFallbackPaths(t *testing.T) {
	t.Run("default XDG", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", "")

		paths := configFallbackPaths()
		if len(paths) != 2 {
			t.Fatalf("expected 2 fallback paths, got %d", len(paths))
		}
		if paths[0] != filepath.Join(homeDir, ".config", "bd", "config.yaml") {
			t.Errorf("paths[0] = %q, want %q", paths[0], filepath.Join(homeDir, ".config", "bd", "config.yaml"))
		}
		if paths[1] != filepath.Join(homeDir, ".beads", "config.yaml") {
			t.Errorf("paths[1] = %q, want %q", paths[1], filepath.Join(homeDir, ".beads", "config.yaml"))
		}
	})

	t.Run("custom XDG_CONFIG_HOME", func(t *testing.T) {
		homeDir := t.TempDir()
		xdgDir := filepath.Join(t.TempDir(), "custom-config")
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", xdgDir)

		paths := configFallbackPaths()
		if len(paths) != 2 {
			t.Fatalf("expected 2 fallback paths, got %d", len(paths))
		}
		if paths[0] != filepath.Join(xdgDir, "bd", "config.yaml") {
			t.Errorf("paths[0] = %q, want %q", paths[0], filepath.Join(xdgDir, "bd", "config.yaml"))
		}
		if paths[1] != filepath.Join(homeDir, ".beads", "config.yaml") {
			t.Errorf("paths[1] = %q, want %q", paths[1], filepath.Join(homeDir, ".beads", "config.yaml"))
		}
	})
}
