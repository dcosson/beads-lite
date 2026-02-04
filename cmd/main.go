// bd is the CLI for beads, a git-native issue tracker.
package main

import (
	"fmt"
	"os"

	"beads-lite/internal/cmd"
)

func run() error {
	return cmd.Execute()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
