// Package cmd implements the CLI commands for beads-lite.
package cmd

import (
	"io"
	"os"
	"strings"
	"sync"

	"beads-lite/internal/config"
	"beads-lite/internal/config/yamlstore"
	"beads-lite/internal/storage/filesystem"

	"github.com/spf13/cobra"
)

// AppProvider lazily initializes the App on first use.
type AppProvider struct {
	once sync.Once
	app  *App
	err  error

	// Config captured from flags before Execute()
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
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}

	configStore, err := yamlstore.New(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	if err := config.ApplyDefaults(configStore); err != nil {
		return nil, err
	}
	if err := config.ApplyEnvOverrides(configStore); err != nil {
		return nil, err
	}

	store := filesystem.New(paths.DataDir)
	store.CleanupStaleLocks()

	out := p.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := p.Err
	if errOut == nil {
		errOut = os.Stderr
	}

	return &App{
		Storage:     store,
		ConfigStore: configStore,
		ConfigDir:   paths.ConfigDir,
		Out:         out,
		Err:         errOut,
		JSON:        p.JSONOutput,
	}, nil
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
		Long: `Beads Lite is a git-native issue tracker that stores issues as JSON files.
Issues are stored in .beads/<project>/open/ and .beads/<project>/closed/ directories,
making them easy to review, diff, and track alongside your code.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Apply BD_JSON env var if --json flag was not explicitly passed
			if !cmd.Flags().Changed("json") {
				if envJSON := os.Getenv(config.EnvJSON); envJSON != "" {
					envJSON = strings.ToLower(envJSON)
					if envJSON == "1" || envJSON == "true" {
						provider.JSONOutput = true
					}
				}
			}
			return nil
		},
	}

	// Global flags - these populate the provider config
	rootCmd.PersistentFlags().BoolVar(&provider.JSONOutput, "json", false, "Output in JSON format (env: BD_JSON)")

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
	rootCmd.AddCommand(newCommentsCmd(provider))
	rootCmd.AddCommand(newChildrenCmd(provider))
	rootCmd.AddCommand(newDepCmd(provider))
	rootCmd.AddCommand(newCompactCmd(provider))
	rootCmd.AddCommand(newConfigCmd(provider))
	rootCmd.AddCommand(newMolCmd(provider))

	return rootCmd
}
