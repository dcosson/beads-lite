package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newReadyCmd creates the ready command.
func newReadyCmd(provider *AppProvider) *cobra.Command {
	var (
		priority string
		molID    string
		molType  string
		assignee string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List issues that are ready to work on",
		Long: `List issues that are ready to work on (open, not blocked).

An issue is "ready" if:
- Status is "open" (not in-progress, blocked, deferred, or closed)
- All issues in depends_on are closed
- Ephemeral issues are always excluded

Without --mol, molecule steps (issues with a parent) are excluded to
avoid overwhelming output. With --mol, ONLY steps from that molecule
are shown, along with parallel group info.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// List all open issues
			filter := &issuestorage.ListFilter{
				Statuses: []issuestorage.Status{issuestorage.StatusOpen},
			}

			// Apply priority filter if specified
			if priority != "" {
				p, err := parsePriority(priority)
				if err != nil {
					return err
				}
				filter.Priority = &p
			}

			// Apply assignee filter if specified
			if assignee != "" {
				filter.Assignees = []string{assignee}
			}

			// Apply mol-type filter if specified
			if molType != "" {
				if !issuestorage.ValidateMolType(molType) {
					return fmt.Errorf("invalid mol-type %q: must be one of swarm, patrol, work", molType)
				}
				mt := issuestorage.MolType(molType)
				filter.MolType = &mt
			}

			// When --mol is specified, scope to children of that molecule
			if molID != "" {
				filter.Parent = &molID
			}

			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing issues: %w", err)
			}

			// Get closed issues to check dependency resolution
			closedFilter := &issuestorage.ListFilter{
				Statuses: []issuestorage.Status{issuestorage.StatusClosed},
			}
			closedIssues, err := app.Storage.List(ctx, closedFilter)
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}
			closedSet := make(map[string]bool)
			for _, issue := range closedIssues {
				closedSet[issue.ID] = true
			}

			// Read cascade config flag
			cascade := cascadeEnabled(app)

			// Filter to only ready issues, excluding ephemeral and
			// (when not in --mol mode) molecule steps.
			var ready []*issuestorage.Issue
			for _, issue := range issues {
				// Ephemeral issues are never ready (hardcoded exclusion).
				if issue.Ephemeral {
					continue
				}
				// Without --mol, exclude molecule steps (issues with a parent)
				// to keep the output focused on top-level work.
				if molID == "" && issue.Parent != "" {
					continue
				}
				blocked, err := graph.IsEffectivelyBlocked(ctx, app.Storage, issue, closedSet, cascade)
				if err != nil {
					return fmt.Errorf("checking blocked status for %s: %w", issue.ID, err)
				}
				if !blocked {
					ready = append(ready, issue)
				}
			}

			// Apply limit
			if limit > 0 && len(ready) > limit {
				ready = ready[:limit]
			}

			if app.JSON {
				// Use IssueSimpleJSON format (no dependency counts)
				result := make([]IssueSimpleJSON, len(ready))
				for i, issue := range ready {
					result[i] = ToIssueSimpleJSON(issue)
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			if len(ready) == 0 {
				fmt.Fprintln(app.Out, "No ready issues found.")
				return nil
			}

			if molID != "" {
				fmt.Fprintf(app.Out, "Ready steps in %s (%d):\n\n", molID, len(ready))
				// Show parallel group info: all ready steps can run concurrently.
				if len(ready) > 1 {
					fmt.Fprintf(app.Out, "  ⚡ %d steps can run in parallel:\n", len(ready))
				}
				for _, issue := range ready {
					fmt.Fprintf(app.Out, "  %s  [%s] %s\n", issue.ID, issue.Priority.Display(), issue.Title)
				}
			} else {
				fmt.Fprintf(app.Out, "Ready issues (%d):\n\n", len(ready))
				for _, issue := range ready {
					fmt.Fprintf(app.Out, "  %s  [%s] %s\n", issue.ID, issue.Priority.Display(), issue.Title)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority (0-4 or P0-P4)")
	cmd.Flags().StringVar(&molID, "mol", "", "Restrict to ready steps within a molecule")
	cmd.Flags().StringVar(&molType, "mol-type", "", "Filter by molecule type (swarm, patrol, work)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of issues to show")

	return cmd
}

// cascadeEnabled reads the graph.cascade_parent_blocking config flag.
// Returns true (the default) if the flag is not set or is "true".
func cascadeEnabled(app *App) bool {
	if app.ConfigStore == nil {
		return true
	}
	v, ok := app.ConfigStore.Get("graph.cascade_parent_blocking")
	if !ok {
		return true
	}
	return v != "false"
}
