package cmd

import (
	"fmt"
	"strings"
	"time"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newGateCmd creates the gate command group.
func newGateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Gate commands for async coordination primitives",
	}

	cmd.AddCommand(newGateShowCmd(provider))

	return cmd
}

// newGateShowCmd creates the gate show subcommand.
func newGateShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <gate-id>",
		Short: "Show details of a gate",
		Long: `Display detailed information about a gate issue.

The issue must have type "gate". Supports prefix matching on gate IDs.

Examples:
  bd gate show bl-abc123     # Exact ID match
  bd gate show bl-abc        # Prefix match (if unique)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := args[0]

			// Route to correct storage
			store, err := app.StorageFor(ctx, query)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", query, err)
			}

			// Try exact match first
			issue, err := store.Get(ctx, query)
			if err == nil && issue.Status == issuestorage.StatusTombstone {
				err = issuestorage.ErrNotFound
			}
			if err == issuestorage.ErrNotFound {
				issue, err = findByPrefix(store, ctx, query)
			}
			if err != nil {
				if err == issuestorage.ErrNotFound {
					return fmt.Errorf("no issue found matching %q", query)
				}
				return err
			}

			// Verify it's a gate
			if issue.Type != issuestorage.TypeGate {
				return fmt.Errorf("issue %s is type %q, not \"gate\"", issue.ID, issue.Type)
			}

			// JSON output uses existing ToIssueJSON path
			if app.JSON {
				return outputIssueJSON(app, ctx, issue)
			}

			// Text output matching the spec format
			fmt.Fprintf(app.Out, "Gate: %s\n", issue.ID)
			fmt.Fprintf(app.Out, "Title: %s\n", issue.Title)
			fmt.Fprintf(app.Out, "Status: %s\n", issue.Status)

			if issue.AwaitType != "" || issue.AwaitID != "" {
				fmt.Fprintf(app.Out, "Await: %s %s\n", issue.AwaitType, issue.AwaitID)
			}

			if issue.TimeoutNS != 0 {
				d := time.Duration(issue.TimeoutNS)
				fmt.Fprintf(app.Out, "Timeout: %s\n", d.String())
			}

			if len(issue.Waiters) > 0 {
				fmt.Fprintf(app.Out, "Waiters: %s\n", strings.Join(issue.Waiters, ", "))
			}

			fmt.Fprintf(app.Out, "Created: %s\n", issue.CreatedAt.Format(time.RFC3339))

			return nil
		},
	}

	return cmd
}
