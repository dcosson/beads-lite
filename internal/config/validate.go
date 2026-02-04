package config

import (
	"fmt"
	"strconv"
	"strings"
)

// validValues maps known keys to their allowed values.
// An empty slice means any non-empty string is accepted.
var validValues = map[string][]string{
	"defaults.priority":   {"0", "1", "2", "3", "4", "P0", "P1", "P2", "P3", "P4", "critical", "high", "medium", "low", "backlog"},
	"defaults.type":       {"task", "bug", "feature", "epic", "chore"},
	"issue_prefix":        {},
	"actor":               {},
	"project.name":        {},
	"hierarchy.max_depth": {},
	"types.custom":        {},
	"status.custom":       {},
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
			effective := allowed
			// For defaults.type, also accept custom types from types.custom
			if key == "defaults.type" {
				if customStr, ok := all["types.custom"]; ok {
					effective = append(append([]string{}, allowed...), splitCustomValues(customStr)...)
				}
			}
			if !contains(effective, val) {
				errs = append(errs, fmt.Sprintf(
					"%s: invalid value %q (allowed: %s)",
					key, val, strings.Join(effective, ", ")))
			}
			continue
		}

		// Keys with no enumerated values have type-specific checks.
		switch key {
		case "hierarchy.max_depth":
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

// SplitCustomValues splits a comma-separated string into trimmed, non-empty values.
func SplitCustomValues(s string) []string {
	return splitCustomValues(s)
}

func splitCustomValues(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
