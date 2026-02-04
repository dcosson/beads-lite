// Package cmd implements the bd command-line interface.
package cmd

import (
	"context"
	"io"
	"os"

	"beads-lite/internal/config"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/kvstorage"
	"beads-lite/internal/meow"
	"beads-lite/internal/routing"

	"golang.org/x/term"
)

// App holds application state shared across commands.
type App struct {
	Storage        issuestorage.IssueStore
	SlotStore      kvstorage.KVStore
	AgentStore     kvstorage.KVStore
	MergeSlotStore kvstorage.KVStore
	Router         *routing.Router // nil if no routes.jsonl
	ConfigStore    config.Store
	ConfigDir      string // path to .beads directory
	FormulaPath    meow.FormulaSearchPath
	Out            io.Writer
	Err            io.Writer
	JSON           bool // output in JSON format
}

// StorageFor returns the storage for the given issue ID, routing if needed.
// If the ID belongs to a remote rig, opens a filesystem store at the
// resolved remote path. Returns the local store if no routing is needed.
func (a *App) StorageFor(ctx context.Context, id string) (issuestorage.IssueStore, error) {
	if a.Router == nil {
		return a.Storage, nil
	}

	paths, prefix, isRemote, err := a.Router.Resolve(id)
	if err != nil {
		return nil, err
	}
	if !isRemote {
		return a.Storage, nil
	}

	return filesystem.New(paths.DataDir, prefix), nil
}

// IsColor returns true if colored output should be used.
// Color is enabled when stdout is a TTY or CLICOLOR_FORCE=1 is set,
// and disabled when NO_COLOR is set.
func (a *App) IsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") == "1" {
		return true
	}
	if f, ok := a.Out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return true
	}
	return false
}

// Colorize wraps s in the given ANSI code if color is enabled.
// code should be the numeric part only, e.g. "31" for red or "38;5;214" for orange.
func (a *App) Colorize(s string, code string) string {
	if !a.IsColor() {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

// SuccessColor returns the string wrapped in green ANSI codes if color is enabled.
func (a *App) SuccessColor(s string) string {
	return a.Colorize(s, "32")
}

// WarnColor returns the string wrapped in orange ANSI codes if color is enabled.
func (a *App) WarnColor(s string) string {
	return a.Colorize(s, "38;5;214")
}
