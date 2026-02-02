package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newListCmd creates the list command.
func newListCmd(provider *AppProvider) *cobra.Command {
	var (
		status    string
		priority  string
		issueType string
		labels    []string
		parent    string
		assignee  string
		all       bool
		closed    bool
		roots     bool
		format    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues with filtering",
		Long: `List issues with various filters.

By default, lists open issues. Use flags to filter by status, type,
priority, labels, parent, or assignee.

Examples:
  bd list                      # List all open issues
  bd list --all                # List all issues (open and closed)
  bd list --closed             # List only closed issues
  bd list --status=in-progress # List in-progress issues
  bd list --type=bug           # List bugs
  bd list --priority=high      # List high priority issues
  bd list --labels=urgent,v2   # List issues with both labels
  bd list --parent=be-abc      # List children of issue be-abc
  bd list --roots              # List root issues (no parent)
  bd list --assignee=alice     # List issues assigned to alice`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Build filter
			filter := &issuestorage.ListFilter{}

			// Status handling
			if all {
				// No status filter - list everything
				filter.Status = nil
			} else if closed {
				s := issuestorage.StatusClosed
				filter.Status = &s
			} else if status != "" {
				// Allow tombstone as a valid filter for list (even though
				// update rejects it — listing tombstones is an admin query)
				if strings.ToLower(status) == "tombstone" {
					s := issuestorage.StatusTombstone
					filter.Status = &s
				} else {
					s, err := parseStatus(status)
					if err != nil {
						return err
					}
					filter.Status = &s
				}
			} else {
				// Default: list open issues
				s := issuestorage.StatusOpen
				filter.Status = &s
			}

			if priority != "" {
				p := issuestorage.Priority(priority)
				filter.Priority = &p
			}

			if issueType != "" {
				t := issuestorage.IssueType(issueType)
				filter.Type = &t
			}

			if len(labels) > 0 {
				filter.Labels = labels
			}

			if assignee != "" {
				filter.Assignee = &assignee
			}

			if roots {
				// Empty string signals "root only" (no parent)
				empty := ""
				filter.Parent = &empty
			} else if parent != "" {
				filter.Parent = &parent
			}

			// Get open issues with filter
			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing issues: %w", err)
			}

			// If --all flag, we need to also get closed issues
			if all && filter.Status == nil {
				// Get closed issues with same filters
				closedFilter := *filter
				closedStatus := issuestorage.StatusClosed
				closedFilter.Status = &closedStatus
				closedIssues, err := app.Storage.List(ctx, &closedFilter)
				if err != nil {
					return fmt.Errorf("listing closed issues: %w", err)
				}
				issues = append(issues, closedIssues...)
			}

			// JSON output
			if app.JSON {
				result := make([]IssueListJSON, len(issues))
				for i, issue := range issues {
					result[i] = ToIssueListJSON(issue)
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output (--format is accepted but not implemented, matching original beads)
			if len(issues) == 0 {
				fmt.Fprintln(app.Out, "No issues found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Issues (%d):\n\n", len(issues))
			for _, issue := range issues {
				statusStr := string(issue.Status)
				typeStr := string(issue.Type)
				priorityStr := string(issue.Priority)

				fmt.Fprintf(app.Out, "  %s  [%s] [%s] [%s] %s\n",
					issue.ID, statusStr, typeStr, priorityStr, issue.Title)

				if issue.Assignee != "" {
					fmt.Fprintf(app.Out, "       Assignee: %s\n", issue.Assignee)
				}
				if len(issue.Labels) > 0 {
					fmt.Fprintf(app.Out, "       Labels: %s\n", strings.Join(issue.Labels, ", "))
				}
				if issue.Parent != "" {
					fmt.Fprintf(app.Out, "       Parent: %s\n", issue.Parent)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (open, in-progress, blocked, deferred, closed)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority (critical, high, medium, low)")
	cmd.Flags().StringVarP(&issueType, "type", "t", "", "Filter by type (task, bug, feature, epic, chore)")
	cmd.Flags().StringSliceVarP(&labels, "labels", "l", nil, "Filter by labels (comma-separated, must have all)")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent issue ID")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().BoolVar(&all, "all", false, "List all issues (open and closed)")
	cmd.Flags().BoolVar(&closed, "closed", false, "List only closed issues")
	cmd.Flags().BoolVar(&roots, "roots", false, "List only root issues (no parent)")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Output format (not implemented, accepts any value)")

	return cmd
}
