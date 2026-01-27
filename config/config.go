// Package config handles configuration loading for beads.
package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"beads2/storage"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration.
type Config struct {
	Defaults struct {
		Priority storage.Priority  `yaml:"priority"`
		Type     storage.IssueType `yaml:"type"`
	} `yaml:"defaults"`
	ID struct {
		Prefix string `yaml:"prefix"`
		Length int    `yaml:"length"`
	} `yaml:"id"`
	Actor string `yaml:"actor"`
}

// Default returns a Config with sensible default values.
func Default() *Config {
	c := &Config{}
	c.Defaults.Priority = storage.PriorityMedium
	c.Defaults.Type = storage.TypeTask
	c.ID.Prefix = "bd-"
	c.ID.Length = 4
	c.Actor = "$USER"
	return c
}

// Load loads configuration from .beads/config.yaml in the given beads directory.
// If the file doesn't exist, returns default configuration.
// Environment variables in values are expanded after loading.
func Load(beadsDir string) (*Config, error) {
	cfg := Default()

	configPath := filepath.Join(beadsDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.ExpandEnvVars()
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.ExpandEnvVars()
	return cfg, nil
}

// ExpandEnvVars expands environment variables in configuration values.
// Supports ${VAR} and $VAR syntax.
func (c *Config) ExpandEnvVars() {
	c.Actor = expandEnv(c.Actor)
	c.ID.Prefix = expandEnv(c.ID.Prefix)
}

// expandEnv expands environment variables in a string.
// Supports ${VAR} and $VAR syntax.
func expandEnv(s string) string {
	// First handle ${VAR} syntax
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		return os.Getenv(varName)
	})

	// Then handle $VAR syntax (only for simple variable names)
	re = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[1:]
		// Don't expand if it's already been expanded or if env var doesn't exist
		if val := os.Getenv(varName); val != "" {
			return val
		}
		// Check if this is a literal $ followed by text (not an env var)
		if strings.HasPrefix(match, "$") && os.Getenv(varName) == "" {
			// Return empty for missing env vars
			return ""
		}
		return match
	})

	return s
}
