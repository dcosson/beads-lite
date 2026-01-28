package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// newShowCmd creates the show command.
func newShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <issue-id>",
		Short: "Show full details of an issue",
		Long: `Display detailed information about a single issue.

Supports prefix matching on issue IDs. If the prefix matches exactly one issue,
that issue is displayed. If multiple issues match, all matching IDs are listed.

Examples:
  bd show bd-a1b2       # Exact ID match
  bd show bd-a1         # Prefix match (if unique)
  bd show a1b2          # Prefix match without 'bd-' prefix`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := args[0]

			// Try exact match first
			issue, err := app.Storage.Get(ctx, query)
			if err == nil {
				return outputIssue(app, issue)
			}
			if err != storage.ErrNotFound {
				return fmt.Errorf("getting issue %s: %w", query, err)
			}

			// Exact match failed, try prefix matching
			issue, err = findByPrefix(app.Storage, ctx, query)
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("no issue found matching %q", query)
				}
				return err
			}

			return outputIssue(app, issue)
		},
	}

	return cmd
}

// outputIssue formats and outputs the issue details.
func outputIssue(app *App, issue *storage.Issue) error {
	if app.JSON {
		return json.NewEncoder(app.Out).Encode(issue)
	}

	// Header: ID and Title with status
	fmt.Fprintf(app.Out, "%s: %s\n", issue.ID, issue.Title)
	fmt.Fprintln(app.Out, strings.Repeat("-", len(issue.ID)+len(issue.Title)+2))

	// Basic metadata
	fmt.Fprintf(app.Out, "Status:   %s\n", issue.Status)
	fmt.Fprintf(app.Out, "Priority: %s\n", issue.Priority)
	fmt.Fprintf(app.Out, "Type:     %s\n", issue.Type)

	if issue.Assignee != "" {
		fmt.Fprintf(app.Out, "Assignee: %s\n", issue.Assignee)
	}

	if len(issue.Labels) > 0 {
		fmt.Fprintf(app.Out, "Labels:   %s\n", strings.Join(issue.Labels, ", "))
	}

	// Timestamps
	fmt.Fprintf(app.Out, "Created:  %s\n", issue.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(app.Out, "Updated:  %s\n", issue.UpdatedAt.Format("2006-01-02 15:04:05"))
	if issue.ClosedAt != nil {
		fmt.Fprintf(app.Out, "Closed:   %s\n", issue.ClosedAt.Format("2006-01-02 15:04:05"))
	}

	// Hierarchy
	if issue.Parent != "" {
		fmt.Fprintf(app.Out, "\nParent:   %s\n", issue.Parent)
	}
	if len(issue.Children) > 0 {
		fmt.Fprintf(app.Out, "\nChildren:\n")
		for _, child := range issue.Children {
			fmt.Fprintf(app.Out, "  - %s\n", child)
		}
	}

	// Dependencies
	if len(issue.DependsOn) > 0 {
		fmt.Fprintf(app.Out, "\nDepends On:\n")
		for _, dep := range issue.DependsOn {
			fmt.Fprintf(app.Out, "  - %s\n", dep)
		}
	}
	if len(issue.Dependents) > 0 {
		fmt.Fprintf(app.Out, "\nDependents:\n")
		for _, dep := range issue.Dependents {
			fmt.Fprintf(app.Out, "  - %s\n", dep)
		}
	}

	// Blocking
	if len(issue.Blocks) > 0 {
		fmt.Fprintf(app.Out, "\nBlocks:\n")
		for _, b := range issue.Blocks {
			fmt.Fprintf(app.Out, "  - %s\n", b)
		}
	}
	if len(issue.BlockedBy) > 0 {
		fmt.Fprintf(app.Out, "\nBlocked By:\n")
		for _, b := range issue.BlockedBy {
			fmt.Fprintf(app.Out, "  - %s\n", b)
		}
	}

	// Description
	if issue.Description != "" {
		fmt.Fprintf(app.Out, "\nDescription:\n%s\n", issue.Description)
	}

	// Comments
	if len(issue.Comments) > 0 {
		fmt.Fprintf(app.Out, "\nComments (%d):\n", len(issue.Comments))
		for _, comment := range issue.Comments {
			fmt.Fprintf(app.Out, "\n  [%s] %s (%s):\n", comment.ID, comment.Author, comment.CreatedAt.Format("2006-01-02 15:04"))
			// Indent comment body
			lines := strings.Split(comment.Body, "\n")
			for _, line := range lines {
				fmt.Fprintf(app.Out, "    %s\n", line)
			}
		}
	}

	return nil
}
