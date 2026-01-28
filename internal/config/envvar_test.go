package config

import (
	"testing"
)

func TestApplyEnvOverrides_Actor(t *testing.T) {
	t.Setenv(EnvActor, "test-actor")

	cfg := Default()
	ApplyEnvOverrides(&cfg)

	if cfg.Actor != "test-actor" {
		t.Errorf("Actor = %q, want %q", cfg.Actor, "test-actor")
	}
}

func TestApplyEnvOverrides_Project(t *testing.T) {
	t.Setenv(EnvProject, "test-project")

	cfg := Default()
	ApplyEnvOverrides(&cfg)

	if cfg.Project.Name != "test-project" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "test-project")
	}
}

func TestApplyEnvOverrides_NoOverride(t *testing.T) {
	// Ensure env vars are not set
	t.Setenv(EnvActor, "")
	t.Setenv(EnvProject, "")

	cfg := Default()
	original := cfg
	ApplyEnvOverrides(&cfg)

	if cfg.Actor != original.Actor {
		t.Errorf("Actor = %q, want %q (should not change)", cfg.Actor, original.Actor)
	}
	if cfg.Project.Name != original.Project.Name {
		t.Errorf("Project.Name = %q, want %q (should not change)", cfg.Project.Name, original.Project.Name)
	}
}

func TestApplyEnvOverrides_Both(t *testing.T) {
	t.Setenv(EnvActor, "env-actor")
	t.Setenv(EnvProject, "env-project")

	cfg := Default()
	ApplyEnvOverrides(&cfg)

	if cfg.Actor != "env-actor" {
		t.Errorf("Actor = %q, want %q", cfg.Actor, "env-actor")
	}
	if cfg.Project.Name != "env-project" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "env-project")
	}
	// Other fields should remain defaults
	if cfg.Defaults.Priority != "medium" {
		t.Errorf("Defaults.Priority = %q, want %q", cfg.Defaults.Priority, "medium")
	}
}
