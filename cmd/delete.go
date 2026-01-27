package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// DeleteResult represents the result of deleting an issue.
type DeleteResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// NewDeleteCmd creates the delete command.
func NewDeleteCmd(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete an issue",
		Long: `Permanently delete an issue.

This operation cannot be undone. The issue file is removed from the filesystem.

Unless --force is specified, you will be prompted to confirm the deletion.

Examples:
  bd delete bd-a1b2
  bd delete bd-a1 --force   # skip confirmation, prefix match`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			id, err := app.ResolveID(ctx, args[0])
			if err != nil {
				if app.JSON {
					return app.OutputJSON(DeleteResult{
						ID:     args[0],
						Status: "error",
						Error:  err.Error(),
					})
				}
				return fmt.Errorf("%s: %w", args[0], err)
			}

			// Get issue details for confirmation
			issue, err := app.Storage.Get(ctx, id)
			if err != nil {
				if app.JSON {
					return app.OutputJSON(DeleteResult{
						ID:     id,
						Status: "error",
						Error:  err.Error(),
					})
				}
				return fmt.Errorf("%s: %w", id, err)
			}

			// Prompt for confirmation unless --force
			if !force {
				prompt := fmt.Sprintf("Delete %s (%s)?", id, issue.Title)
				if !app.Confirm(prompt) {
					if app.JSON {
						return app.OutputJSON(DeleteResult{
							ID:     id,
							Status: "cancelled",
						})
					}
					fmt.Fprintf(app.Out, "cancelled\n")
					return nil
				}
			}

			if err := app.Storage.Delete(ctx, id); err != nil {
				if app.JSON {
					return app.OutputJSON(DeleteResult{
						ID:     id,
						Status: "error",
						Error:  err.Error(),
					})
				}
				return fmt.Errorf("%s: %w", id, err)
			}

			if app.JSON {
				return app.OutputJSON(DeleteResult{
					ID:     id,
					Status: "deleted",
				})
			}

			fmt.Fprintf(app.Out, "deleted %s\n", id)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
