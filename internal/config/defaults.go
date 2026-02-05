package config

import (
	"strconv"

	"beads-lite/internal/idgen"
)

// BeadsVariantKey is the config key that identifies the beads variant.
const BeadsVariantKey = "beads_variant"

// BeadsVariantValue is the value for beads-lite repositories.
const BeadsVariantValue = "beads-lite"

// DefaultValues returns the default config map for the core keys.
func DefaultValues() map[string]string {
	return map[string]string{
		BeadsVariantKey:              BeadsVariantValue,
		"create.require-description": "false",
		"defaults.priority":          "2",
		"defaults.type":              "task",
		"issue_prefix":               "bd",
		"actor":                      "${USER}",
		"hierarchy.max_depth":        strconv.Itoa(idgen.DefaultMaxHierarchyDepth),
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
