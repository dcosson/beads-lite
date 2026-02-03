package config

import (
	"strconv"

	"beads-lite/internal/issuestorage"
)

// DefaultValues returns the default config map for the core keys.
func DefaultValues() map[string]string {
	return map[string]string{
		"create.require-description": "false",
		"defaults.priority":          "medium",
		"defaults.type":              "task",
		"issue_prefix":               "bd",
		"actor":                      "${USER}",
		"project.name":               "issues",
		"hierarchy.max_depth":        strconv.Itoa(issuestorage.DefaultMaxHierarchyDepth),
	}
}

// ApplyDefaults fills any missing core keys in s with their default values.
// Values are set in memory only and not persisted to disk.
func ApplyDefaults(s Store) {
	defaults := DefaultValues()
	all := s.All()
	for k, v := range defaults {
		if _, exists := all[k]; !exists {
			s.SetInMemory(k, v)
		}
	}
}
