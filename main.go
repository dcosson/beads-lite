// bd is the CLI for beads, a git-native issue tracker.
package main

import (
	"fmt"
	"os"

	"beads2/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
