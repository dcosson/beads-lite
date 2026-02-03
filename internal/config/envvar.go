package config

import "os"

// Environment variable names for beads-lite configuration.
const (
	EnvBeadsDir = "BEADS_DIR"  // Path to .beads directory
	EnvActor    = "BD_ACTOR"   // Override actor name
	EnvProject  = "BD_PROJECT" // Override project name
	EnvJSON     = "BD_JSON"    // Enable JSON output ("1" or "true")
	EnvQuiet    = "BD_QUIET"   // Suppress non-error output ("1" or "true")
)

// ApplyEnvOverrides checks BD_ACTOR and BD_PROJECT env vars
// and overrides the corresponding config values in memory.
// These overrides are not persisted to the config file.
func ApplyEnvOverrides(s Store) {
	if actor := os.Getenv(EnvActor); actor != "" {
		s.SetInMemory("actor", actor)
	}
	if project := os.Getenv(EnvProject); project != "" {
		s.SetInMemory("project.name", project)
	}
}
