// Package config handles beads configuration loading and defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the contents of .beads/config.yaml.
type Config struct {
	Defaults DefaultsConfig `yaml:"defaults"`
	ID       IDConfig       `yaml:"id"`
	Actor    string         `yaml:"actor"`
	Project  ProjectConfig  `yaml:"project"`
}

type DefaultsConfig struct {
	Priority string `yaml:"priority"`
	Type     string `yaml:"type"`
}

type IDConfig struct {
	Prefix string `yaml:"prefix"`
	Length int    `yaml:"length"`
}

type ProjectConfig struct {
	Name string `yaml:"name"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		Defaults: DefaultsConfig{
			Priority: "medium",
			Type:     "task",
		},
		ID: IDConfig{
			Prefix: "bd-",
			Length: 4,
		},
		Actor: "${USER}",
		Project: ProjectConfig{
			Name: "issues",
		},
	}
}

// Load reads config.yaml from path and applies defaults for missing fields.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Project.Name == "" {
		cfg.Project.Name = "issues"
	}

	return cfg, nil
}

// LoadWithFallback loads config from primaryPath, then merges in values
// from fallback locations for any fields that remain at their zero value.
// Fallback locations: ~/.config/bd/config.yaml, ~/.beads/config.yaml
func LoadWithFallback(primaryPath string) (Config, error) {
	cfg, err := Load(primaryPath)
	if err != nil {
		return Config{}, err
	}

	for _, fallbackPath := range configFallbackPaths() {
		if fallbackPath == primaryPath {
			continue
		}
		fallback, fErr := Load(fallbackPath)
		if fErr != nil {
			continue // fallback files are optional
		}
		cfg = mergeConfig(cfg, fallback)
	}

	return cfg, nil
}

// configFallbackPaths returns the ordered list of fallback config file paths.
// It respects XDG_CONFIG_HOME for the first path.
func configFallbackPaths() []string {
	var paths []string

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			xdgConfig = filepath.Join(home, ".config")
		}
	}
	if xdgConfig != "" {
		paths = append(paths, filepath.Join(xdgConfig, "bd", "config.yaml"))
	}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".beads", "config.yaml"))
	}

	return paths
}

// mergeConfig fills zero-value fields in primary from fallback.
func mergeConfig(primary, fallback Config) Config {
	if primary.Actor == "" {
		primary.Actor = fallback.Actor
	}
	if primary.Defaults.Priority == "" {
		primary.Defaults.Priority = fallback.Defaults.Priority
	}
	if primary.Defaults.Type == "" {
		primary.Defaults.Type = fallback.Defaults.Type
	}
	if primary.ID.Prefix == "" {
		primary.ID.Prefix = fallback.ID.Prefix
	}
	if primary.ID.Length == 0 {
		primary.ID.Length = fallback.ID.Length
	}
	if primary.Project.Name == "" {
		primary.Project.Name = fallback.Project.Name
	}
	return primary
}

// Write writes the provided configuration to path.
func Write(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// WriteDefault writes the default configuration to path.
func WriteDefault(path string) error {
	return Write(path, Default())
}

// loadOptional loads a config file, returning an empty Config and nil error
// if the file does not exist.
func loadOptional(path string) (Config, error) {
	cfg, err := Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	return cfg, nil
}
