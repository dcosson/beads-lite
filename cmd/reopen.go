package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ReopenResult represents the result of reopening an issue.
type ReopenResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// NewReopenCmd creates the reopen command.
func NewReopenCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a closed issue",
		Long: `Reopen a closed issue.

The issue is moved from closed/ back to open/, its status is set to "open",
and its closed_at timestamp is cleared.

Examples:
  bd reopen bd-a1b2
  bd reopen bd-a1      # prefix matching`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			id, err := app.ResolveID(ctx, args[0])
			if err != nil {
				if app.JSON {
					return app.OutputJSON(ReopenResult{
						ID:     args[0],
						Status: "error",
						Error:  err.Error(),
					})
				}
				return fmt.Errorf("%s: %w", args[0], err)
			}

			if err := app.Storage.Reopen(ctx, id); err != nil {
				if app.JSON {
					return app.OutputJSON(ReopenResult{
						ID:     id,
						Status: "error",
						Error:  err.Error(),
					})
				}
				return fmt.Errorf("%s: %w", id, err)
			}

			if app.JSON {
				return app.OutputJSON(ReopenResult{
					ID:     id,
					Status: "reopened",
				})
			}

			fmt.Fprintf(app.Out, "reopened %s\n", id)
			return nil
		},
	}

	return cmd
}
