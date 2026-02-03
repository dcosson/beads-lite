package config

import (
	"testing"
)

func TestApplyEnvOverrides_Actor(t *testing.T) {
	t.Setenv(EnvActor, "test-actor")

	s := &memStore{data: map[string]string{"actor": "${USER}"}}
	ApplyEnvOverrides(s)

	if v, _ := s.Get("actor"); v != "test-actor" {
		t.Errorf("actor = %q, want %q", v, "test-actor")
	}
}

func TestApplyEnvOverrides_Project(t *testing.T) {
	t.Setenv(EnvProject, "test-project")

	s := &memStore{data: map[string]string{"project.name": "issues"}}
	ApplyEnvOverrides(s)

	if v, _ := s.Get("project.name"); v != "test-project" {
		t.Errorf("project.name = %q, want %q", v, "test-project")
	}
}

func TestApplyEnvOverrides_NoOverride(t *testing.T) {
	t.Setenv(EnvActor, "")
	t.Setenv(EnvProject, "")

	s := &memStore{data: map[string]string{
		"actor":        "${USER}",
		"project.name": "issues",
	}}
	ApplyEnvOverrides(s)

	if v, _ := s.Get("actor"); v != "${USER}" {
		t.Errorf("actor = %q, want %q (should not change)", v, "${USER}")
	}
	if v, _ := s.Get("project.name"); v != "issues" {
		t.Errorf("project.name = %q, want %q (should not change)", v, "issues")
	}
}

func TestApplyEnvOverrides_Both(t *testing.T) {
	t.Setenv(EnvActor, "env-actor")
	t.Setenv(EnvProject, "env-project")

	s := &memStore{data: map[string]string{
		"actor":             "${USER}",
		"project.name":      "issues",
		"defaults.priority": "medium",
	}}
	ApplyEnvOverrides(s)

	if v, _ := s.Get("actor"); v != "env-actor" {
		t.Errorf("actor = %q, want %q", v, "env-actor")
	}
	if v, _ := s.Get("project.name"); v != "env-project" {
		t.Errorf("project.name = %q, want %q", v, "env-project")
	}
	// Other keys should remain unchanged
	if v, _ := s.Get("defaults.priority"); v != "medium" {
		t.Errorf("defaults.priority = %q, want %q", v, "medium")
	}
}
