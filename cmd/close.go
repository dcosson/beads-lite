package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// CloseResult represents the result of closing an issue.
type CloseResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// NewCloseCmd creates the close command.
func NewCloseCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <id> [id...]",
		Short: "Close one or more issues",
		Long: `Close one or more issues.

Issues are moved from open/ to closed/, their status is set to "closed",
and their closed_at timestamp is set.

Examples:
  bd close bd-a1b2
  bd close bd-a1b2 bd-c3d4 bd-e5f6   # close multiple
  bd close bd-a1                      # prefix matching`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var results []CloseResult

			for _, arg := range args {
				id, err := app.ResolveID(ctx, arg)
				if err != nil {
					result := CloseResult{
						ID:     arg,
						Status: "error",
						Error:  err.Error(),
					}
					results = append(results, result)
					if !app.JSON {
						fmt.Fprintf(app.Err, "error: %s: %v\n", arg, err)
					}
					continue
				}

				if err := app.Storage.Close(ctx, id); err != nil {
					result := CloseResult{
						ID:     id,
						Status: "error",
						Error:  err.Error(),
					}
					results = append(results, result)
					if !app.JSON {
						fmt.Fprintf(app.Err, "error: %s: %v\n", id, err)
					}
					continue
				}

				result := CloseResult{
					ID:     id,
					Status: "closed",
				}
				results = append(results, result)
				if !app.JSON {
					fmt.Fprintf(app.Out, "closed %s\n", id)
				}
			}

			if app.JSON {
				return app.OutputJSON(results)
			}

			return nil
		},
	}

	return cmd
}
