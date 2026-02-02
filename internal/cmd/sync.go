package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newSyncCmd creates the sync command.
// In beads-lite the storage layer is filesystem-based with no separate sync
// step, so this command is a no-op accepted for compatibility with the
// reference implementation.
func newSyncCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync beads data (no-op in beads-lite)",
		Long: `In the reference implementation, sync pushes and pulls bead changes
via a git sync branch. beads-lite uses direct filesystem storage and
does not require a separate sync step. This command is accepted for
compatibility but performs no action.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			if app.JSON {
				fmt.Fprintln(app.Out, `{"status":"noop","message":"sync is not needed in beads-lite"}`)
				return nil
			}

			fmt.Fprintln(app.Out, "sync: no-op (beads-lite uses direct filesystem storage)")
			return nil
		},
	}

	cmd.Flags().Bool("import-only", false, "Only import changes (no-op in beads-lite)")

	return cmd
}
