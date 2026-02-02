// Package cmd implements the bd command-line interface.
package cmd

import (
	"context"
	"io"
	"os"

	"beads-lite/internal/config"
	"beads-lite/internal/meow"
	"beads-lite/internal/routing"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"

	"golang.org/x/term"
)

// App holds application state shared across commands.
type App struct {
	Storage     issuestorage.IssueStore
	Router      *routing.Router // nil if no routes.json
	ConfigStore config.Store
	ConfigDir   string // path to .beads directory
	FormulaPath meow.FormulaSearchPath
	Out         io.Writer
	Err         io.Writer
	JSON        bool // output in JSON format
}

// StorageFor returns the storage for the given issue ID, routing if needed.
// If the ID belongs to a remote rig, opens a filesystem store at the
// resolved remote path. Returns the local store if no routing is needed.
func (a *App) StorageFor(ctx context.Context, id string) (issuestorage.IssueStore, error) {
	if a.Router == nil {
		return a.Storage, nil
	}

	paths, isRemote, err := a.Router.Resolve(id)
	if err != nil {
		return nil, err
	}
	if !isRemote {
		return a.Storage, nil
	}

	return filesystem.New(paths.DataDir), nil
}

// SuccessColor returns the string wrapped in green ANSI codes if stdout is a terminal,
// otherwise returns the string unchanged.
func (a *App) SuccessColor(s string) string {
	if f, ok := a.Out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return "\033[32m" + s + "\033[0m"
	}
	return s
}

// WarnColor returns the string wrapped in orange ANSI codes if stdout is a terminal,
// otherwise returns the string unchanged.
func (a *App) WarnColor(s string) string {
	if f, ok := a.Out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return "\033[38;5;214m" + s + "\033[0m"
	}
	return s
}
