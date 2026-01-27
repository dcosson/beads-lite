// Package cmd implements the CLI commands for beads.
package cmd

import (
	"io"

	"beads2/storage"
)

// App holds application state shared across commands.
type App struct {
	Storage storage.Storage
	Out     io.Writer
	Err     io.Writer
	JSON    bool // output in JSON format
}
