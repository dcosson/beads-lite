package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads2/internal/storage"

	"github.com/spf13/cobra"
)

// newListCmd creates the list command.
func newListCmd(provider *AppProvider) *cobra.Command {
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
		Long: `List issues with optional filters.

By default, lists all open issues. Use flags to filter by status, type,
priority, labels, parent, or assignee.

Examples:
  bd list                    # all open issues
  bd list --all              # open and closed
  bd list --closed           # only closed
  bd list --type bug         # only bugs
  bd list --priority high    # only high priority
  bd list --label backend    # with label
  bd list --parent bd-a1b2   # children of issue
  bd list --roots            # only root issues (no parent)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			var issues []*storage.Issue

			if closed {
				// Only closed issues
				closedStatus := storage.StatusClosed
				filter := buildFilter(closedStatus, typeFlag, priority, labels, assignee, parent)
				issues, err = app.Storage.List(ctx, filter)
				if err != nil {
					return fmt.Errorf("listing issues: %w", err)
				}
			} else if all {
				// Both open and closed
				openIssues, err := app.Storage.List(ctx, buildFilter("", typeFlag, priority, labels, assignee, parent))
				if err != nil {
					return fmt.Errorf("listing open issues: %w", err)
				}

				closedStatus := storage.StatusClosed
				closedIssues, err := app.Storage.List(ctx, buildFilter(closedStatus, typeFlag, priority, labels, assignee, parent))
				if err != nil {
					return fmt.Errorf("listing closed issues: %w", err)
				}

				issues = append(openIssues, closedIssues...)
			} else {
				// Default: open issues only (nil filter returns open issues)
				filter := buildFilter("", typeFlag, priority, labels, assignee, parent)
				issues, err = app.Storage.List(ctx, filter)
				if err != nil {
					return fmt.Errorf("listing issues: %w", err)
				}
			}

			// Apply --roots filter in-memory (filter out issues with a parent)
			if roots {
				issues = filterRoots(issues)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(issues)
			}

			return formatOutput(app, issues, format)
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Include closed issues")
	cmd.Flags().BoolVar(&closed, "closed", false, "Only closed issues")
	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Filter by type (task, bug, feature, epic, chore)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority (critical, high, medium, low)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Filter by label (can repeat, must have all)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent (show children of issue)")
	cmd.Flags().BoolVar(&roots, "roots", false, "Only root issues (no parent)")
	cmd.Flags().StringVarP(&format, "format", "f", "short", "Output format: short, long, ids")

	return cmd
}

// buildFilter constructs a ListFilter from the given parameters.
func buildFilter(status storage.Status, typeFlag, priority string, labels []string, assignee, parent string) *storage.ListFilter {
	filter := &storage.ListFilter{}

	if status != "" {
		s := status
		filter.Status = &s
	}

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

	if parent != "" {
		filter.Parent = &parent
	}

	return filter
}

// filterRoots returns only issues without a parent.
func filterRoots(issues []*storage.Issue) []*storage.Issue {
	var roots []*storage.Issue
	for _, issue := range issues {
		if issue.Parent == "" {
			roots = append(roots, issue)
		}
	}
	return roots
}

// formatOutput writes issues to the app output in the specified format.
func formatOutput(app *App, issues []*storage.Issue, format string) error {
	if len(issues) == 0 {
		fmt.Fprintln(app.Out, "No issues found.")
		return nil
	}

	switch format {
	case "ids":
		for _, issue := range issues {
			fmt.Fprintln(app.Out, issue.ID)
		}
	case "long":
		for i, issue := range issues {
			if i > 0 {
				fmt.Fprintln(app.Out, strings.Repeat("-", 40))
			}
			fmt.Fprintf(app.Out, "ID:       %s\n", issue.ID)
			fmt.Fprintf(app.Out, "Title:    %s\n", issue.Title)
			fmt.Fprintf(app.Out, "Type:     %s\n", issue.Type)
			fmt.Fprintf(app.Out, "Status:   %s\n", issue.Status)
			fmt.Fprintf(app.Out, "Priority: %s\n", issue.Priority)
			if issue.Assignee != "" {
				fmt.Fprintf(app.Out, "Assignee: %s\n", issue.Assignee)
			}
			if len(issue.Labels) > 0 {
				fmt.Fprintf(app.Out, "Labels:   %s\n", strings.Join(issue.Labels, ", "))
			}
			if issue.Parent != "" {
				fmt.Fprintf(app.Out, "Parent:   %s\n", issue.Parent)
			}
			if issue.Description != "" {
				fmt.Fprintf(app.Out, "\n%s\n", issue.Description)
			}
			fmt.Fprintln(app.Out)
		}
	default: // "short"
		for _, issue := range issues {
			fmt.Fprintf(app.Out, "%s  [%s] [%s] %s\n", issue.ID, issue.Type, issue.Status, issue.Title)
		}
	}

	return nil
}
