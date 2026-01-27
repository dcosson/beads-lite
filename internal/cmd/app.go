// Package cmd implements the bd command-line interface.
package cmd

import (
	"io"

	"beads2/internal/storage"
)

// App holds application state shared across commands.
type App struct {
	Storage storage.Storage
	Out     io.Writer
	Err     io.Writer
	JSON    bool // output in JSON format
}
