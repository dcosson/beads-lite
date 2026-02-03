package config

// Store provides key-value access to beads configuration.
// Keys are flat strings (dotted keys like "defaults.priority" are literal
// strings, not nested paths).
type Store interface {
	// Get returns the value for key and whether it was found.
	Get(key string) (string, bool)

	// Set writes key=value to the store and persists to disk.
	Set(key, value string) error

	// SetInMemory writes key=value to the in-memory store without persisting.
	// Use this for runtime overrides (defaults, env vars) that should not be
	// written back to the config file.
	SetInMemory(key, value string)

	// Unset removes key from the store and persists to disk.
	Unset(key string) error

	// All returns a copy of all key-value pairs.
	All() map[string]string
}
