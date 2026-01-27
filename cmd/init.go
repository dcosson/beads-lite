// Package cmd implements the bd command-line interface.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"beads2/filesystem"
)

// InitOptions configures the init command.
type InitOptions struct {
	Path  string // Path to initialize (defaults to current directory)
	Force bool   // Force initialization even if .beads exists
}

// Init initializes a new beads repository.
// It creates the .beads/open/ and .beads/closed/ directories.
// Returns an error if .beads already exists (unless Force is true).
func Init(opts InitOptions) error {
	// Default to current directory
	basePath := opts.Path
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
		if !opts.Force {
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
