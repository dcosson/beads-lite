// Package cmd implements the bd command-line interface.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"beads-lite/internal/config"
	"beads-lite/internal/config/yamlstore"
	"beads-lite/internal/issuestorage/filesystem"

	"github.com/spf13/cobra"
)

// newInitCmd creates the init command.
// Note: init doesn't use the provider since it creates the .beads directory.
func newInitCmd(provider *AppProvider) *cobra.Command {
	var (
		force       bool
		projectName string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new beads-lite repository",
		Long:  `Initialize a new beads-lite repository in the current directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(force, projectName)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force initialization even if .beads exists")
	cmd.Flags().StringVar(&projectName, "project", "issues", "Project name for data directory")

	return cmd
}

func runInit(force bool, projectName string) error {
	if projectName == "" {
		return errors.New("project name cannot be empty")
	}
	// Path resolution: BEADS_DIR env var > CWD
	var basePath string
	if envDir := os.Getenv(config.EnvBeadsDir); envDir != "" {
		basePath = envDir
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		basePath = cwd
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	beadsPath := absPath
	if filepath.Base(beadsPath) != ".beads" {
		beadsPath = filepath.Join(absPath, ".beads")
	}

	// Check if .beads already exists
	if _, err := os.Stat(beadsPath); err == nil {
		if !force {
			return errors.New("beads-lite repository already exists (use --force to reinitialize)")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking .beads directory: %w", err)
	}

	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		return fmt.Errorf("creating .beads directory: %w", err)
	}

	formulasPath := filepath.Join(beadsPath, "formulas")
	if err := os.MkdirAll(formulasPath, 0755); err != nil {
		return fmt.Errorf("creating formulas directory: %w", err)
	}

	configPath := filepath.Join(beadsPath, "config.yaml")
	store, err := yamlstore.New(configPath)
	if err != nil {
		return fmt.Errorf("creating config store: %w", err)
	}
	if err := config.ApplyDefaults(store); err != nil {
		return fmt.Errorf("writing default config: %w", err)
	}
	if err := store.Set("project.name", projectName); err != nil {
		return fmt.Errorf("setting project name: %w", err)
	}

	prefix, _ := store.Get("id.prefix")

	dataPath := filepath.Join(beadsPath, projectName)

	// Create the issue storage
	issueStore := filesystem.New(dataPath, prefix)
	if err := issueStore.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	fmt.Printf("Initialized beads-lite repository at %s\n", beadsPath)
	return nil
}
