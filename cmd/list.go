package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads2/storage"

	"github.com/spf13/cobra"
)

// NewListCmd creates the list command.
func NewListCmd(app *App) *cobra.Command {
	var (
		all      bool
		closed   bool
		typeFlag string
		priority string
		labels   []string
		assignee string
		parent   string
		roots    bool
		format   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		Long: `List issues with optional filtering.

By default, shows all open issues. Use flags to filter results.

Examples:
  bd list                    # all open issues
  bd list --all              # open and closed
  bd list --closed           # only closed
  bd list --type bug         # only bugs
  bd list --priority high    # only high priority
  bd list --label backend    # with label
  bd list --parent bd-a1b2   # children of issue
  bd list --roots            # only root issues (no parent)
  bd list --format ids       # just IDs, one per line`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			filter := &storage.ListFilter{}

			// Handle status filtering
			if closed {
				s := storage.StatusClosed
				filter.Status = &s
			}
			// Note: --all means include both open and closed
			// When neither --all nor --closed is set, we filter for non-closed (open)
			// The storage layer handles nil Status as "open only" by default

			if typeFlag != "" {
				t := storage.IssueType(typeFlag)
				filter.Type = &t
			}

			if priority != "" {
				p := storage.Priority(priority)
				filter.Priority = &p
			}

			if len(labels) > 0 {
				filter.Labels = labels
			}

			if assignee != "" {
				filter.Assignee = &assignee
			}

			if roots {
				// Empty string means root only (no parent)
				empty := ""
				filter.Parent = &empty
			} else if parent != "" {
				filter.Parent = &parent
			}

			// Fetch issues
			var issues []*storage.Issue
			var err error

			if all {
				// Get both open and closed issues
				openFilter := copyFilter(filter)
				issues, err = app.Storage.List(ctx, openFilter)
				if err != nil {
					return err
				}

				closedStatus := storage.StatusClosed
				closedFilter := copyFilter(filter)
				closedFilter.Status = &closedStatus
				closedIssues, err := app.Storage.List(ctx, closedFilter)
				if err != nil {
					return err
				}
				issues = append(issues, closedIssues...)
			} else {
				issues, err = app.Storage.List(ctx, filter)
				if err != nil {
					return err
				}
			}

			// Output based on format
			if app.JSON {
				return json.NewEncoder(app.Out).Encode(issues)
			}

			switch format {
			case "ids":
				for _, issue := range issues {
					fmt.Fprintln(app.Out, issue.ID)
				}
			case "long":
				for i, issue := range issues {
					if i > 0 {
						fmt.Fprintln(app.Out)
					}
					printIssueLong(app, issue)
				}
			default: // "short"
				for _, issue := range issues {
					printIssueShort(app, issue)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Include closed issues")
	cmd.Flags().BoolVar(&closed, "closed", false, "Only closed issues")
	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Filter by type (task, bug, feature, epic, chore)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority (critical, high, medium, low)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Filter by label (repeatable, must have all)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent issue ID")
	cmd.Flags().BoolVar(&roots, "roots", false, "Only root issues (no parent)")
	cmd.Flags().StringVarP(&format, "format", "f", "short", "Output format: short, long, ids")

	return cmd
}

// copyFilter creates a shallow copy of a ListFilter.
func copyFilter(f *storage.ListFilter) *storage.ListFilter {
	if f == nil {
		return &storage.ListFilter{}
	}
	copy := *f
	if f.Labels != nil {
		copy.Labels = make([]string, len(f.Labels))
		for i := range f.Labels {
			copy.Labels[i] = f.Labels[i]
		}
	}
	return &copy
}

// printIssueShort prints an issue in short format (one line).
func printIssueShort(app *App, issue *storage.Issue) {
	status := statusSymbol(issue.Status)
	priority := prioritySymbol(issue.Priority)
	typeStr := typeSymbol(issue.Type)

	fmt.Fprintf(app.Out, "%s %s [%s %s] [%s] - %s\n",
		status, issue.ID, priority, issue.Priority, typeStr, issue.Title)
}

// printIssueLong prints an issue in long format (multiple lines).
func printIssueLong(app *App, issue *storage.Issue) {
	status := statusSymbol(issue.Status)

	fmt.Fprintf(app.Out, "%s %s - %s\n", status, issue.ID, issue.Title)
	fmt.Fprintf(app.Out, "   Type: %s | Priority: %s | Status: %s\n",
		issue.Type, issue.Priority, issue.Status)

	if issue.Assignee != "" {
		fmt.Fprintf(app.Out, "   Assignee: %s\n", issue.Assignee)
	}

	if len(issue.Labels) > 0 {
		fmt.Fprintf(app.Out, "   Labels: %s\n", strings.Join(issue.Labels, ", "))
	}

	if issue.Parent != "" {
		fmt.Fprintf(app.Out, "   Parent: %s\n", issue.Parent)
	}

	if len(issue.Children) > 0 {
		fmt.Fprintf(app.Out, "   Children: %s\n", strings.Join(issue.Children, ", "))
	}

	if len(issue.DependsOn) > 0 {
		fmt.Fprintf(app.Out, "   Depends on: %s\n", strings.Join(issue.DependsOn, ", "))
	}

	if len(issue.BlockedBy) > 0 {
		fmt.Fprintf(app.Out, "   Blocked by: %s\n", strings.Join(issue.BlockedBy, ", "))
	}

	if issue.Description != "" {
		fmt.Fprintf(app.Out, "   Description:\n")
		for _, line := range strings.Split(issue.Description, "\n") {
			fmt.Fprintf(app.Out, "      %s\n", line)
		}
	}
}

// statusSymbol returns a symbol representing the issue status.
func statusSymbol(status storage.Status) string {
	switch status {
	case storage.StatusOpen:
		return "○"
	case storage.StatusInProgress:
		return "◐"
	case storage.StatusBlocked:
		return "●"
	case storage.StatusDeferred:
		return "◇"
	case storage.StatusClosed:
		return "✓"
	default:
		return "?"
	}
}

// prioritySymbol returns a symbol representing the issue priority.
func prioritySymbol(priority storage.Priority) string {
	switch priority {
	case storage.PriorityCritical:
		return "‼"
	case storage.PriorityHigh:
		return "●"
	case storage.PriorityMedium:
		return "◐"
	case storage.PriorityLow:
		return "○"
	default:
		return "?"
	}
}

// typeSymbol returns a short string representing the issue type.
func typeSymbol(t storage.IssueType) string {
	switch t {
	case storage.TypeTask:
		return "task"
	case storage.TypeBug:
		return "bug"
	case storage.TypeFeature:
		return "feat"
	case storage.TypeEpic:
		return "epic"
	case storage.TypeChore:
		return "chore"
	default:
		return string(t)
	}
}
