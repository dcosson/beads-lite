// Package cmd implements the bd command-line interface.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"beads2/internal/storage/filesystem"

	"github.com/spf13/cobra"
)

// newInitCmd creates the init command.
// Note: init doesn't use the provider since it creates the .beads directory.
func newInitCmd(provider *AppProvider) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new beads repository",
		Long:  `Initialize a new beads repository in the current directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(provider.BeadsPath, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force initialization even if .beads exists")

	return cmd
}

func runInit(path string, force bool) error {
	// Default to current directory
	basePath := path
	if basePath == "" {
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

	beadsPath := filepath.Join(absPath, ".beads")

	// Check if .beads already exists
	if _, err := os.Stat(beadsPath); err == nil {
		if !force {
			return errors.New("beads repository already exists (use --force to reinitialize)")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking .beads directory: %w", err)
	}

	// Create the storage
	storage := filesystem.New(beadsPath)
	if err := storage.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	fmt.Printf("Initialized beads repository at %s\n", beadsPath)
	return nil
}
