// Package cmd implements the CLI commands for beads.
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"beads2/filesystem"
	"beads2/storage"

	"github.com/spf13/cobra"
)

// App holds the application state shared across all commands.
type App struct {
	Storage storage.Storage
	Out     io.Writer
	Err     io.Writer
	JSON    bool
}

var (
	// Global flags
	jsonOutput bool
	beadsPath  string

	// The App instance, initialized in PersistentPreRunE
	app *App
)

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "bd",
	Short: "A lightweight issue tracker that lives in your repo",
	Long: `Beads is a git-native issue tracker that stores issues as JSON files.
Issues are stored in .beads/open/ and .beads/closed/ directories,
making them easy to review, diff, and track alongside your code.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip initialization for commands that don't need storage
		if cmd.Name() == "init" || cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		var err error
		app, err = NewApp(beadsPath, jsonOutput, os.Stdout, os.Stderr)
		if err != nil {
			return err
		}
		return nil
	},
}

// NewApp creates a new App instance with initialized storage.
func NewApp(path string, jsonOutput bool, out, errOut io.Writer) (*App, error) {
	beadsDir, err := FindBeadsDir(path)
	if err != nil {
		return nil, err
	}

	store := filesystem.New(beadsDir)

	return &App{
		Storage: store,
		Out:     out,
		Err:     errOut,
		JSON:    jsonOutput,
	}, nil
}

// FindBeadsDir locates the .beads directory.
// If path is provided, it uses that directly.
// Otherwise, it walks up from the current directory looking for .beads.
func FindBeadsDir(path string) (string, error) {
	if path != "" {
		// Use the provided path directly
		info, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("cannot access beads directory %s: %w", path, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("beads path is not a directory: %s", path)
		}
		return path, nil
	}

	// Walk up from current directory looking for .beads
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get current directory: %w", err)
	}

	dir := cwd
	for {
		beadsDir := filepath.Join(dir, ".beads")
		info, err := os.Stat(beadsDir)
		if err == nil && info.IsDir() {
			return beadsDir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .beads
			return "", fmt.Errorf("no .beads directory found (searched from %s to /)", cwd)
		}
		dir = parent
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// GetApp returns the initialized App instance.
// This should only be called from subcommand Run functions.
func GetApp() *App {
	return app
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&beadsPath, "path", "", "Path to .beads directory (default: search from cwd)")
}
