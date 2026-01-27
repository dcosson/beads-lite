package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Paths captures resolved locations for config and data.
type Paths struct {
	ConfigDir  string // path to .beads directory
	ConfigFile string // path to .beads/config.yaml
	DataDir    string // path to .beads/<project>
}

// ResolvePaths resolves config and data paths.
// If basePath is provided, it is treated as the .beads directory.
// Otherwise, it searches upward for .beads/config.yaml and defaults to cwd/.beads.
func ResolvePaths(basePath string) (Paths, Config, error) {
	if basePath != "" {
		normalized, err := normalizeBasePath(basePath)
		if err != nil {
			return Paths{}, Config{}, err
		}
		return resolveFromBase(normalized)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Paths{}, Config{}, fmt.Errorf("cannot get current directory: %w", err)
	}

	configDir, configFile, found, err := findConfigUpward(cwd)
	if err != nil {
		return Paths{}, Config{}, err
	}
	if !found {
		configDir = filepath.Join(cwd, ".beads")
		configFile = filepath.Join(configDir, "config.yaml")
		if _, err := os.Stat(configFile); err != nil {
			return Paths{}, Config{}, missingConfigErr(configFile)
		}
	}

	cfg, err := Load(configFile)
	if err != nil {
		return Paths{}, Config{}, err
	}

	dataDir := filepath.Join(configDir, cfg.Project.Name)
	if err := ensureDirExists(dataDir); err != nil {
		return Paths{}, Config{}, missingDataErr(dataDir)
	}

	return Paths{
		ConfigDir:  configDir,
		ConfigFile: configFile,
		DataDir:    dataDir,
	}, cfg, nil
}

func resolveFromBase(basePath string) (Paths, Config, error) {
	info, err := os.Stat(basePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Paths{}, Config{}, missingConfigErr(filepath.Join(basePath, "config.yaml"))
		}
		return Paths{}, Config{}, fmt.Errorf("cannot access beads directory %s: %w", basePath, err)
	}
	if !info.IsDir() {
		return Paths{}, Config{}, fmt.Errorf("beads path is not a directory: %s", basePath)
	}

	configFile := filepath.Join(basePath, "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		return Paths{}, Config{}, missingConfigErr(configFile)
	}

	cfg, err := Load(configFile)
	if err != nil {
		return Paths{}, Config{}, err
	}

	dataDir := filepath.Join(basePath, cfg.Project.Name)
	if err := ensureDirExists(dataDir); err != nil {
		return Paths{}, Config{}, missingDataErr(dataDir)
	}

	return Paths{
		ConfigDir:  basePath,
		ConfigFile: configFile,
		DataDir:    dataDir,
	}, cfg, nil
}

func normalizeBasePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	if filepath.Base(absPath) != ".beads" {
		absPath = filepath.Join(absPath, ".beads")
	}
	return absPath, nil
}

func findConfigUpward(start string) (string, string, bool, error) {
	dir := start
	for {
		configDir := filepath.Join(dir, ".beads")
		configFile := filepath.Join(configDir, "config.yaml")
		if info, err := os.Stat(configFile); err == nil && !info.IsDir() {
			return configDir, configFile, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", false, fmt.Errorf("checking config: %w", err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false, nil
		}
		dir = parent
	}
}

func ensureDirExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

func missingConfigErr(path string) error {
	return fmt.Errorf("beads config not found at %s (run `bd init`)", path)
}

func missingDataErr(path string) error {
	return fmt.Errorf("beads data not found at %s (run `bd init`)", path)
}
