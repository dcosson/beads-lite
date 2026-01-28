package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// newReadyCmd creates the ready command.
func newReadyCmd(provider *AppProvider) *cobra.Command {
	var priority string

	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List issues that are ready to work on",
		Long: `List issues that are ready to work on (open, not blocked).

An issue is "ready" if:
- Status is "open" (not in-progress, blocked, deferred, or closed)
- All issues in depends_on are closed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// List all open issues
			openStatus := storage.StatusOpen
			filter := &storage.ListFilter{
				Status: &openStatus,
			}

			// Apply priority filter if specified
			if priority != "" {
				p := storage.Priority(priority)
				filter.Priority = &p
			}

			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing issues: %w", err)
			}

			// Get closed issues to check dependency resolution
			closedStatus := storage.StatusClosed
			closedFilter := &storage.ListFilter{
				Status: &closedStatus,
			}
			closedIssues, err := app.Storage.List(ctx, closedFilter)
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}
			closedSet := make(map[string]bool)
			for _, issue := range closedIssues {
				closedSet[issue.ID] = true
			}

			// Filter to only ready issues
			var ready []*storage.Issue
			for _, issue := range issues {
				if isReady(issue, closedSet) {
					ready = append(ready, issue)
				}
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(ready)
			}

			if len(ready) == 0 {
				fmt.Fprintln(app.Out, "No ready issues found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Ready issues (%d):\n\n", len(ready))
			for _, issue := range ready {
				fmt.Fprintf(app.Out, "  %s  [%s] %s\n", issue.ID, issue.Priority, issue.Title)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority (critical, high, medium, low)")

	return cmd
}

// isReady returns true if an issue is ready to work on.
// An issue is ready if all its dependencies (depends_on) are closed.
func isReady(issue *storage.Issue, closedSet map[string]bool) bool {
	for _, dep := range issue.DependsOn {
		if !closedSet[dep] {
			return false // Dependency not closed
		}
	}
	return true
}
