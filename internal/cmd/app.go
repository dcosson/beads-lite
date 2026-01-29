// Package cmd implements the bd command-line interface.
package cmd

import (
	"io"
	"os"

	"beads-lite/internal/config"
	"beads-lite/internal/storage"

	"golang.org/x/term"
)

// App holds application state shared across commands.
type App struct {
	Storage     storage.Storage
	Config      config.Config
	ConfigStore config.Store
	Out         io.Writer
	Err         io.Writer
	JSON        bool // output in JSON format
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
