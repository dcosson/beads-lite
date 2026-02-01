package cmd

import (
	"os"
	"os/exec"
	"strings"
)

// resolveActor determines the current actor identity (a name/identifier).
// Resolution priority matches the reference beads implementation:
//  1. BD_ACTOR env var (via config store, set by ApplyEnvOverrides)
//  2. BEADS_ACTOR env var
//  3. git config user.name
//  4. $USER env var
//  5. "unknown"
func resolveActor(app *App) (string, error) {
	if app != nil && app.ConfigStore != nil {
		if actor, ok := app.ConfigStore.Get("actor"); ok && actor != "" && actor != "${USER}" {
			return actor, nil
		}
	}

	if actor := os.Getenv("BEADS_ACTOR"); actor != "" {
		return actor, nil
	}

	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name, nil
		}
	}

	if user := os.Getenv("USER"); user != "" {
		return user, nil
	}

	return "unknown", nil
}

// resolveOwner returns the issue owner, which is always an email address.
// Resolution priority matches the reference beads implementation:
//  1. GIT_AUTHOR_EMAIL env var (set during git commit operations)
//  2. git config user.email
//  3. "" (empty string — owner is optional)
func resolveOwner() string {
	if email := os.Getenv("GIT_AUTHOR_EMAIL"); email != "" {
		return email
	}

	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		if email := strings.TrimSpace(string(out)); email != "" {
			return email
		}
	}

	return ""
}
