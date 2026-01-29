package config

import (
	"fmt"
	"strconv"
	"strings"
)

// validValues maps known keys to their allowed values.
// An empty slice means any non-empty string is accepted.
var validValues = map[string][]string{
	"defaults.priority": {"critical", "high", "medium", "low", "backlog"},
	"defaults.type":     {"task", "bug", "feature", "epic", "chore"},
	"id.prefix":         {},
	"id.length":         {},
	"actor":             {},
	"project.name":      {},
}

// Validate checks all values in s for known keys. It returns an error
// describing every invalid value found, or nil if all values are valid.
func Validate(s Store) error {
	all := s.All()
	var errs []string

	for key, allowed := range validValues {
		val, ok := all[key]
		if !ok {
			continue
		}

		if len(allowed) > 0 {
			if !contains(allowed, val) {
				errs = append(errs, fmt.Sprintf(
					"%s: invalid value %q (allowed: %s)",
					key, val, strings.Join(allowed, ", ")))
			}
			continue
		}

		// Keys with no enumerated values have type-specific checks.
		switch key {
		case "id.length":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				errs = append(errs, fmt.Sprintf(
					"%s: must be a positive integer, got %q", key, val))
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
