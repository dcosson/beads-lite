package config

import "os"

// Environment variable names for beads-lite configuration.
const (
	EnvBeadsDir = "BEADS_DIR"  // Path to .beads directory
	EnvActor    = "BD_ACTOR"   // Override actor name
	EnvProject  = "BD_PROJECT" // Override project name
	EnvJSON     = "BD_JSON"    // Enable JSON output ("1" or "true")
)

// ApplyEnvOverrides checks BD_ACTOR and BD_PROJECT env vars
// and overrides the corresponding config values if set.
func ApplyEnvOverrides(s Store) error {
	if actor := os.Getenv(EnvActor); actor != "" {
		if err := s.Set("actor", actor); err != nil {
			return err
		}
	}
	if project := os.Getenv(EnvProject); project != "" {
		if err := s.Set("project.name", project); err != nil {
			return err
		}
	}
	return nil
}
