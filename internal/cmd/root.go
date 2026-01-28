// Package cmd implements the CLI commands for beads.
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"beads2/internal/config"
	"beads2/internal/storage/filesystem"

	"github.com/spf13/cobra"
)

// AppProvider lazily initializes the App on first use.
type AppProvider struct {
	once sync.Once
	app  *App
	err  error

	// Config captured from flags before Execute()
	BeadsPath  string
	JSONOutput bool
	Out        io.Writer
	Err        io.Writer
}

// Get returns the App, initializing it on first call.
func (p *AppProvider) Get() (*App, error) {
	p.once.Do(func() {
		if p.app == nil {
			p.app, p.err = p.init()
		}
	})
	return p.app, p.err
}

// NewTestProvider creates a provider pre-initialized with the given App.
// Used for testing commands with a mock/test App.
func NewTestProvider(app *App) *AppProvider {
	return &AppProvider{
		app: app,
		Out: app.Out,
		Err: app.Err,
	}
}

func (p *AppProvider) init() (*App, error) {
	paths, cfg, err := config.ResolvePaths(p.BeadsPath)
	if err != nil {
		return nil, err
	}

	store := filesystem.New(paths.DataDir)

	out := p.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := p.Err
	if errOut == nil {
		errOut = os.Stderr
	}

	return &App{
		Storage: store,
		Config:  cfg,
		Out:     out,
		Err:     errOut,
		JSON:    p.JSONOutput,
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

// Execute runs the CLI.
func Execute() error {
	provider := &AppProvider{
		Out: os.Stdout,
		Err: os.Stderr,
	}

	rootCmd := newRootCmd(provider)
	return rootCmd.Execute()
}

// newRootCmd creates the root command with all subcommands.
func newRootCmd(provider *AppProvider) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "bd",
		Short: "A lightweight issue tracker that lives in your repo",
		Long: `Beads is a git-native issue tracker that stores issues as JSON files.
Issues are stored in .beads/<project>/open/ and .beads/<project>/closed/ directories,
making them easy to review, diff, and track alongside your code.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags - these populate the provider config
	rootCmd.PersistentFlags().BoolVar(&provider.JSONOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&provider.BeadsPath, "path", "", "Path to repo or .beads directory (default: search from cwd)")

	// Register all commands
	rootCmd.AddCommand(newInitCmd(provider))
	rootCmd.AddCommand(newCreateCmd(provider))
	rootCmd.AddCommand(newShowCmd(provider))
	rootCmd.AddCommand(newUpdateCmd(provider))
	rootCmd.AddCommand(newDeleteCmd(provider))
	rootCmd.AddCommand(newDoctorCmd(provider))
	rootCmd.AddCommand(newStatsCmd(provider))
	rootCmd.AddCommand(newSearchCmd(provider))
	rootCmd.AddCommand(newReadyCmd(provider))
	rootCmd.AddCommand(newBlockedCmd(provider))
	rootCmd.AddCommand(newCloseCmd(provider))
	rootCmd.AddCommand(newListCmd(provider))
	rootCmd.AddCommand(newReopenCmd(provider))
	rootCmd.AddCommand(newCommentCmd(provider))
	rootCmd.AddCommand(newParentCmd(provider))
	rootCmd.AddCommand(newChildrenCmd(provider))
	rootCmd.AddCommand(newDepCmd(provider))
	rootCmd.AddCommand(newCompactCmd(provider))

	return rootCmd
}
