package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newCloseCmd creates the close command.
func newCloseCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <issue-id> [issue-id...]",
		Short: "Close one or more issues",
		Long: `Close one or more issues by moving them from open/ to closed/ directory.

Sets status to closed and records the closed_at timestamp.

Examples:
  bd close bd-a1b2
  bd close bd-a1b2 bd-c3d4 bd-e5f6`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			var closed []string
			var errors []error

			for _, issueID := range args {
				if err := app.Storage.Close(ctx, issueID); err != nil {
					errors = append(errors, fmt.Errorf("closing %s: %w", issueID, err))
				} else {
					closed = append(closed, issueID)
				}
			}

			// Output results
			if app.JSON {
				result := map[string]interface{}{
					"closed": closed,
				}
				if len(errors) > 0 {
					errStrings := make([]string, len(errors))
					for i, e := range errors {
						errStrings[i] = e.Error()
					}
					result["errors"] = errStrings
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output
			for _, id := range closed {
				fmt.Fprintf(app.Out, "Closed %s\n", id)
			}

			// Return first error if any
			if len(errors) > 0 {
				for _, e := range errors {
					fmt.Fprintf(app.Err, "Error: %v\n", e)
				}
				return errors[0]
			}

			return nil
		},
	}

	return cmd
}
