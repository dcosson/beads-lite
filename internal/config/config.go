// Package config handles beads configuration loading and defaults.
package config

import (
	"fmt"
	"os"

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
