package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newMigrateCmd creates the migrate command.
// In beads-lite the storage layer is filesystem-based with no database schema,
// so this command is a no-op accepted for compatibility with the reference
// implementation which uses Dolt and requires schema migrations.
func newMigrateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations (no-op in beads-lite)",
		Long: `In the reference implementation, migrate applies database schema changes.
beads-lite uses direct filesystem storage with no database, so there are
no migrations to run. This command is accepted for compatibility but
performs no action.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			if app.JSON {
				fmt.Fprintln(app.Out, `{"status":"noop","message":"migrate is not needed in beads-lite"}`)
				return nil
			}

			fmt.Fprintln(app.Out, "migrate: no-op (beads-lite uses direct filesystem storage)")
			return nil
		},
	}

	return cmd
}
