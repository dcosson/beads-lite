// Package cmd implements the bd command-line interface.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"beads-lite/internal/config"
	"beads-lite/internal/config/yamlstore"
	"beads-lite/internal/issuestorage/filesystem"
	kvfs "beads-lite/internal/kvstorage/filesystem"
	"beads-lite/internal/routing"

	"github.com/spf13/cobra"
)

// newInitCmd creates the init command.
// Note: init doesn't use the provider since it creates the .beads directory.
func newInitCmd(provider *AppProvider) *cobra.Command {
	var (
		force  bool
		prefix string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new beads-lite repository",
		Long:  `Initialize a new beads-lite repository in the current directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := provider.Out
			if out == nil {
				out = os.Stdout
			}
			return runInit(out, force, prefix)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force initialization even if .beads exists")
	cmd.Flags().StringVar(&prefix, "prefix", "", "ID prefix for issues (e.g. 'proj-')")

	return cmd
}

func runInit(out io.Writer, force bool, prefix string) error {
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

	// Save existing issue_prefix before writing defaults (for re-init case).
	existingPrefix, hasExistingPrefix := store.Get("issue_prefix")

	for k, v := range config.DefaultValues() {
		if k == "issue_prefix" {
			continue // Resolved separately below.
		}
		if err := store.Set(k, v); err != nil {
			return fmt.Errorf("writing default config: %w", err)
		}
	}

	dataPath := filepath.Join(beadsPath, filesystem.DataDirName)
	idPrefix := resolvePrefix(prefix, existingPrefix, hasExistingPrefix, dataPath, absPath)
	if err := store.Set("issue_prefix", idPrefix); err != nil {
		return fmt.Errorf("setting issue prefix: %w", err)
	}

	// Create the issue storage (takes beadsPath, creates issues/ subdir internally)
	issueStore := filesystem.New(beadsPath, idPrefix)
	if err := issueStore.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	// Create the slot KV store
	slotStore, err := kvfs.New(beadsPath, "slots")
	if err != nil {
		return fmt.Errorf("creating slot store: %w", err)
	}
	if err := slotStore.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing slot store: %w", err)
	}

	// Create the agent KV store
	agentStore, err := kvfs.New(beadsPath, "agents")
	if err != nil {
		return fmt.Errorf("creating agent store: %w", err)
	}
	if err := agentStore.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing agent store: %w", err)
	}

	// Create the merge-slot KV store
	mergeSlotStore, err := kvfs.New(beadsPath, "merge-slot")
	if err != nil {
		return fmt.Errorf("creating merge-slot store: %w", err)
	}
	if err := mergeSlotStore.Init(context.Background()); err != nil {
		return fmt.Errorf("initializing merge-slot store: %w", err)
	}

	// Create .gitignore in .beads/ directory
	gitignorePath := filepath.Join(beadsPath, ".gitignore")
	gitignoreContent := "issues/ephemeral/\n*.lock\n"
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("creating .gitignore: %w", err)
	}

	fmt.Fprintf(out, "Initialized beads-lite repository at %s\n", beadsPath)
	return nil
}

// resolvePrefix determines the issue_prefix using a fallback chain:
//  1. Explicit --prefix flag value
//  2. Existing config value (re-init case)
//  3. Prefix extracted from existing issue files
//  4. Current directory name
func resolvePrefix(flagValue, existingConfig string, hasExistingConfig bool, dataPath, absPath string) string {
	if flagValue != "" {
		return strings.TrimRight(flagValue, "-")
	}
	if hasExistingConfig {
		return existingConfig
	}
	if p := extractPrefixFromExistingIssues(dataPath); p != "" {
		return strings.TrimRight(p, "-")
	}
	return filepath.Base(absPath)
}

// extractPrefixFromExistingIssues scans the data directory for existing issue
// JSON files and extracts the ID prefix from the first one found.
func extractPrefixFromExistingIssues(dataPath string) string {
	for _, dir := range []string{filesystem.DirOpen, filesystem.DirClosed, filesystem.DirDeleted} {
		dirPath := filepath.Join(dataPath, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || filepath.Ext(name) != ".json" || strings.Contains(name, ".tmp.") {
				continue
			}
			id := strings.TrimSuffix(name, ".json")
			if p := routing.ExtractPrefix(id); p != "" {
				return p
			}
		}
	}
	return ""
}
