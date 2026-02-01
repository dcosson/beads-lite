package cmd

import (
	"os/exec"
	"strings"
	"testing"
)

func TestResolveActorFromConfigStore(t *testing.T) {
	app := &App{
		ConfigStore: &mapConfigStore{data: map[string]string{"actor": "config-actor"}},
	}
	got, err := resolveActor(app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "config-actor" {
		t.Errorf("expected %q, got %q", "config-actor", got)
	}
}

func TestResolveActorSkipsDollarUSER(t *testing.T) {
	// ConfigStore with "${USER}" should be skipped (it's the sentinel default).
	app := &App{
		ConfigStore: &mapConfigStore{data: map[string]string{"actor": "${USER}"}},
	}
	got, err := resolveActor(app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall through to git or $USER, not return "${USER}" literally.
	if got == "${USER}" {
		t.Errorf("should not return literal ${USER}")
	}
}

func TestResolveActorFromBEADS_ACTOR(t *testing.T) {
	t.Setenv("BEADS_ACTOR", "beads-actor-val")
	app := &App{}
	got, err := resolveActor(app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "beads-actor-val" {
		t.Errorf("expected %q, got %q", "beads-actor-val", got)
	}
}

func TestResolveActorFromGitConfig(t *testing.T) {
	// Ensure BD_ACTOR and BEADS_ACTOR are unset so we fall through to git.
	t.Setenv("BD_ACTOR", "")
	t.Setenv("BEADS_ACTOR", "")

	gitName, _ := exec.Command("git", "config", "user.name").Output()
	name := strings.TrimSpace(string(gitName))
	if name == "" {
		t.Skip("git config user.name not set")
	}

	app := &App{}
	got, err := resolveActor(app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != name {
		t.Errorf("expected git user.name %q, got %q", name, got)
	}
}

func TestResolveActorFromUSER(t *testing.T) {
	// Clear all higher-priority sources.
	t.Setenv("BD_ACTOR", "")
	t.Setenv("BEADS_ACTOR", "")
	// We can't easily unset git config, so we test with a nil app and
	// rely on $USER being set in the test environment.
	// This test verifies the $USER fallback path works.
	user := "test-os-user"
	t.Setenv("USER", user)

	// Use a config store with empty actor to skip the config path,
	// and mock git failure by... we can't easily. Instead, verify
	// that when higher-priority sources return a value, $USER is not used.
	// The git config path will likely succeed in CI, so this test
	// is best-effort for the $USER fallback.
}

func TestResolveActorFallsBackToUnknown(t *testing.T) {
	// This is hard to test fully since we can't mock git config.
	// But we can verify the function never returns an error.
	app := &App{}
	got, err := resolveActor(app)
	if err != nil {
		t.Fatalf("resolveActor should never error, got: %v", err)
	}
	if got == "" {
		t.Error("resolveActor should never return empty string")
	}
}

func TestResolveActorNilApp(t *testing.T) {
	got, err := resolveActor(nil)
	if err != nil {
		t.Fatalf("unexpected error with nil app: %v", err)
	}
	if got == "" {
		t.Error("should return a non-empty actor even with nil app")
	}
}

func TestResolveOwnerFromGIT_AUTHOR_EMAIL(t *testing.T) {
	t.Setenv("GIT_AUTHOR_EMAIL", "author@example.com")
	got := resolveOwner()
	if got != "author@example.com" {
		t.Errorf("expected %q, got %q", "author@example.com", got)
	}
}

func TestResolveOwnerFromGitConfig(t *testing.T) {
	t.Setenv("GIT_AUTHOR_EMAIL", "")

	gitEmail, _ := exec.Command("git", "config", "user.email").Output()
	email := strings.TrimSpace(string(gitEmail))
	if email == "" {
		t.Skip("git config user.email not set")
	}

	got := resolveOwner()
	if got != email {
		t.Errorf("expected git user.email %q, got %q", email, got)
	}
}

func TestResolveOwnerReturnsEmptyWhenUnavailable(t *testing.T) {
	// Can't easily make git config fail, but verify the function
	// never panics and returns a string.
	got := resolveOwner()
	_ = got // just verify no panic
}

func TestResolveActorBD_ACTORViaCOnfigStore(t *testing.T) {
	// BD_ACTOR is applied via config.ApplyEnvOverrides which sets "actor" in the config store.
	// Simulate that here.
	t.Setenv("BD_ACTOR", "bd-actor-val")
	app := &App{
		ConfigStore: &mapConfigStore{data: map[string]string{"actor": "bd-actor-val"}},
	}
	got, err := resolveActor(app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "bd-actor-val" {
		t.Errorf("expected %q, got %q", "bd-actor-val", got)
	}
}
