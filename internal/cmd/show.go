package cmd

import (
	"context"
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
				return outputIssue(app, ctx, issue)
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

			return outputIssue(app, ctx, issue)
		},
	}

	return cmd
}

// enrichedDep represents a dependency enriched with full issue details for JSON output.
type enrichedDep struct {
	ID             string                `json:"id"`
	Title          string                `json:"title"`
	Status         storage.Status        `json:"status"`
	Priority       storage.Priority      `json:"priority"`
	Type           storage.IssueType     `json:"type"`
	DependencyType storage.DependencyType `json:"dependency_type"`
}

// enrichDeps fetches full issue details for each dependency entry.
func enrichDeps(ctx context.Context, store storage.Storage, deps []storage.Dependency) []enrichedDep {
	result := make([]enrichedDep, 0, len(deps))
	for _, dep := range deps {
		issue, err := store.Get(ctx, dep.ID)
		if err != nil {
			// Include even if we can't load the issue
			result = append(result, enrichedDep{
				ID:             dep.ID,
				DependencyType: dep.Type,
			})
			continue
		}
		result = append(result, enrichedDep{
			ID:             issue.ID,
			Title:          issue.Title,
			Status:         issue.Status,
			Priority:       issue.Priority,
			Type:           issue.Type,
			DependencyType: dep.Type,
		})
	}
	return result
}

// outputIssue formats and outputs the issue details.
func outputIssue(app *App, ctx context.Context, issue *storage.Issue) error {
	if app.JSON {
		return outputIssueJSON(app, ctx, issue)
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
	children := issue.Children()
	if len(children) > 0 {
		fmt.Fprintf(app.Out, "\nChildren:\n")
		for _, child := range children {
			fmt.Fprintf(app.Out, "  - %s\n", child)
		}
	}

	// Dependencies
	if len(issue.Dependencies) > 0 {
		fmt.Fprintf(app.Out, "\nDepends On:\n")
		for _, dep := range issue.Dependencies {
			fmt.Fprintf(app.Out, "  - %s [%s]\n", dep.ID, dep.Type)
		}
	}
	if len(issue.Dependents) > 0 {
		fmt.Fprintf(app.Out, "\nDependents:\n")
		for _, dep := range issue.Dependents {
			fmt.Fprintf(app.Out, "  - %s [%s]\n", dep.ID, dep.Type)
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

// issueJSONOutput is the JSON output format for show command, with enriched deps.
type issueJSONOutput struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Status      storage.Status      `json:"status"`
	Priority    storage.Priority    `json:"priority"`
	Type        storage.IssueType   `json:"type"`
	Parent      string              `json:"parent,omitempty"`
	Dependencies []enrichedDep      `json:"dependencies"`
	Dependents   []enrichedDep      `json:"dependents"`
	Labels      []string            `json:"labels,omitempty"`
	Assignee    string              `json:"assignee,omitempty"`
	Comments    []storage.Comment   `json:"comments,omitempty"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
	ClosedAt    *string             `json:"closed_at,omitempty"`
}

// outputIssueJSON outputs the issue in JSON format with enriched dependencies.
func outputIssueJSON(app *App, ctx context.Context, issue *storage.Issue) error {
	out := issueJSONOutput{
		ID:           issue.ID,
		Title:        issue.Title,
		Description:  issue.Description,
		Status:       issue.Status,
		Priority:     issue.Priority,
		Type:         issue.Type,
		Parent:       issue.Parent,
		Dependencies: enrichDeps(ctx, app.Storage, issue.Dependencies),
		Dependents:   enrichDeps(ctx, app.Storage, issue.Dependents),
		Labels:       issue.Labels,
		Assignee:     issue.Assignee,
		Comments:     issue.Comments,
		CreatedAt:    issue.CreatedAt.Format("2006-01-02T15:04:05.999999999-07:00"),
		UpdatedAt:    issue.UpdatedAt.Format("2006-01-02T15:04:05.999999999-07:00"),
	}
	if issue.ClosedAt != nil {
		closedStr := issue.ClosedAt.Format("2006-01-02T15:04:05.999999999-07:00")
		out.ClosedAt = &closedStr
	}

	return json.NewEncoder(app.Out).Encode(out)
}
