// bd is the CLI for beads, a git-native issue tracker.
package main

import (
	"fmt"
	"os"

	"beads-lite/internal/cmd"
)

var (
	run    = func() error { return cmd.Execute() }
	osExit = os.Exit
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}
