package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newReopenCmd creates the reopen command.
func newReopenCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <issue-id>",
		Short: "Reopen a closed issue",
		Long: `Reopen a closed issue.

Moves the issue from closed/ to open/ directory, sets status to open,
and clears the closed_at timestamp.

Examples:
  bd reopen bd-a1b2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			if err := store.Reopen(ctx, issueID); err != nil {
				return fmt.Errorf("reopening issue %s: %w", issueID, err)
			}

			if app.JSON {
				result := map[string]string{"id": issueID, "status": "reopened"}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Reopened %s\n", issueID)
			return nil
		},
	}

	return cmd
}
