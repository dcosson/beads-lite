// Package cmd implements the bd command-line interface.
package cmd

import (
	"io"

	"beads2/internal/config"
	"beads2/internal/storage"
)

// App holds application state shared across commands.
type App struct {
	Storage storage.Storage
	Config  config.Config
	Out     io.Writer
	Err     io.Writer
	JSON    bool // output in JSON format
}
