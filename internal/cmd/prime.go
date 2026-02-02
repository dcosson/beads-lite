package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newPrimeCmd creates the prime command.
// In beads-lite the storage layer is filesystem-based with no separate priming
// step, so this command is a no-op accepted for compatibility with the
// reference implementation.
func newPrimeCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prime",
		Short: "Prime beads data (no-op in beads-lite)",
		Long: `In the reference implementation, prime pre-loads or warms caches for
bead operations. beads-lite uses direct filesystem storage and does not
require a separate priming step. This command is accepted for
compatibility but performs no action.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			if app.JSON {
				fmt.Fprintln(app.Out, `{"status":"noop","message":"prime is not needed in beads-lite"}`)
				return nil
			}

			fmt.Fprintln(app.Out, "prime: no-op (beads-lite uses direct filesystem storage)")
			return nil
		},
	}

	return cmd
}
