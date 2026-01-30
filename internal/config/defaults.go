package config

// DefaultValues returns the default config map for the 6 core keys.
func DefaultValues() map[string]string {
	return map[string]string{
		"defaults.priority":    "medium",
		"defaults.type":        "task",
		"id.prefix":            "bd-",
		"id.length":            "4",
		"actor":                "${USER}",
		"project.name":         "issues",
		"hierarchy.max_depth":  "3",
	}
}

// ApplyDefaults fills any missing core keys in s with their default values.
func ApplyDefaults(s Store) error {
	defaults := DefaultValues()
	all := s.All()
	for k, v := range defaults {
		if _, exists := all[k]; !exists {
			if err := s.Set(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}
