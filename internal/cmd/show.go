package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/issuestorage"

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

			// Route to correct storage
			store, err := app.StorageFor(ctx, query)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", query, err)
			}

			// Try exact match first
			issue, err := store.Get(ctx, query)
			if err == nil && issue.Status == issuestorage.StatusTombstone {
				// Tombstoned issues are not visible to show
				err = issuestorage.ErrNotFound
			}
			if err == issuestorage.ErrNotFound {
				// Exact match failed (or tombstoned), try prefix matching
				issue, err = findByPrefix(store, ctx, query)
			}
			if err != nil {
				if err == issuestorage.ErrNotFound {
					return fmt.Errorf("no issue found matching %q", query)
				}
				return err
			}

			return outputIssue(app, ctx, issue)
		},
	}

	return cmd
}

// outputIssue formats and outputs the issue details with colored formatting.
func outputIssue(app *App, ctx context.Context, issue *issuestorage.Issue) error {
	if app.JSON {
		return outputIssueJSON(app, ctx, issue)
	}

	w := app.Out

	// --- Header line ---
	// <icon> <id> [EPIC] · <title>   [● P# · STATUS]
	icon, iconColor := statusIcon(issue.Status)
	coloredIcon := app.Colorize(icon, iconColor)

	epicTag := ""
	if issue.Type == issuestorage.TypeEpic {
		epicTag = " " + "[" + app.Colorize("EPIC", issueTypeColor(issue.Type)) + "]"
	}

	priDisplay := issue.Priority.Display()
	priColor := priorityColor(issue.Priority)
	statusStr := strings.ToUpper(string(issue.Status))
	_, statusColor := statusIcon(issue.Status)
	var statusBracket string
	if issue.Status == issuestorage.StatusClosed {
		statusBracket = "[" + app.Colorize(priDisplay, priColor) + " · " + app.Colorize(statusStr, statusColor) + "]"
	} else {
		statusBracket = "[" + app.Colorize("● "+priDisplay, priColor) + " · " + app.Colorize(statusStr, statusColor) + "]"
	}

	if issue.Status == issuestorage.StatusTombstone {
		fmt.Fprintf(w, "%s %s%s · %s   [TOMBSTONE]\n", coloredIcon, issue.ID, epicTag, issue.Title)
	} else {
		fmt.Fprintf(w, "%s %s%s · %s   %s\n", coloredIcon, issue.ID, epicTag, issue.Title, statusBracket)
	}

	// --- Metadata line ---
	// Owner: X · Assignee: Y · Type: Z
	var meta []string
	if issue.Owner != "" {
		meta = append(meta, "Owner: "+issue.Owner)
	}
	if issue.Assignee != "" {
		meta = append(meta, "Assignee: "+issue.Assignee)
	}
	meta = append(meta, "Type: "+string(issue.Type))
	fmt.Fprintln(w, strings.Join(meta, " · "))

	// --- Dates line ---
	fmt.Fprintf(w, "Created: %s · Updated: %s\n", issue.CreatedAt.Format("2006-01-02"), issue.UpdatedAt.Format("2006-01-02"))

	// --- Tombstone metadata ---
	if issue.DeletedAt != nil {
		fmt.Fprintf(w, "Deleted: %s\n", issue.DeletedAt.Format("2006-01-02"))
		if issue.DeletedBy != "" {
			fmt.Fprintf(w, "Deleted By: %s\n", issue.DeletedBy)
		}
		if issue.DeleteReason != "" {
			fmt.Fprintf(w, "Reason: %s\n", issue.DeleteReason)
		}
		if issue.OriginalType != "" {
			fmt.Fprintf(w, "Original Type: %s\n", issue.OriginalType)
		}
	}

	// --- Description ---
	if issue.Description != "" {
		fmt.Fprintf(w, "\nDescription\n\n")
		for _, line := range strings.Split(issue.Description, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	// --- Labels ---
	if len(issue.Labels) > 0 {
		fmt.Fprintf(w, "\nLabels: %s\n", strings.Join(issue.Labels, ", "))
	}

	// --- Parent ---
	if issue.Parent != "" {
		parentIssue, err := app.Storage.Get(ctx, issue.Parent)
		if err == nil {
			fmt.Fprintf(w, "\nParent\n")
			fmt.Fprintf(w, "  %s\n", formatIssueLine(app, parentIssue))
		} else {
			fmt.Fprintf(w, "\nParent: %s\n", issue.Parent)
		}
	}

	// --- Children (parent-child dependents only) ---
	children := issue.Children()
	if len(children) > 0 {
		fmt.Fprintf(w, "\nChildren\n")
		for _, childID := range children {
			childIssue, err := app.Storage.Get(ctx, childID)
			if err == nil {
				fmt.Fprintf(w, "  ↳ %s\n", formatIssueLine(app, childIssue))
			} else {
				fmt.Fprintf(w, "  ↳ %s\n", childID)
			}
		}
	}

	// --- Depends On (non-parent-child dependencies) ---
	var deps []issuestorage.Dependency
	for _, dep := range issue.Dependencies {
		if dep.Type != issuestorage.DepTypeParentChild {
			deps = append(deps, dep)
		}
	}
	if len(deps) > 0 {
		fmt.Fprintf(w, "\nDepends On\n")
		for _, dep := range deps {
			depIssue, err := app.Storage.Get(ctx, dep.ID)
			if err == nil {
				fmt.Fprintf(w, "  → %s\n", formatIssueLine(app, depIssue))
			} else {
				fmt.Fprintf(w, "  → %s\n", dep.ID)
			}
		}
	}

	// --- Blocks (non-parent-child dependents) ---
	var blocks []issuestorage.Dependency
	for _, dep := range issue.Dependents {
		if dep.Type != issuestorage.DepTypeParentChild {
			blocks = append(blocks, dep)
		}
	}
	if len(blocks) > 0 {
		fmt.Fprintf(w, "\nBlocks\n")
		for _, dep := range blocks {
			depIssue, err := app.Storage.Get(ctx, dep.ID)
			if err == nil {
				fmt.Fprintf(w, "  ← %s\n", formatIssueLine(app, depIssue))
			} else {
				fmt.Fprintf(w, "  ← %s\n", dep.ID)
			}
		}
	}

	// --- Comments ---
	if len(issue.Comments) > 0 {
		fmt.Fprintf(w, "\nComments (%d)\n", len(issue.Comments))
		for _, comment := range issue.Comments {
			fmt.Fprintf(w, "\n  [%d] %s (%s):\n", comment.ID, comment.Author, comment.CreatedAt.Format("2006-01-02 15:04"))
			for _, line := range strings.Split(comment.Text, "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}

	return nil
}

// outputIssueJSON outputs the issue in JSON format matching original beads.
// Returns an array with the single issue, with enriched dependencies.
func outputIssueJSON(app *App, ctx context.Context, issue *issuestorage.Issue) error {
	out := ToIssueJSON(ctx, app.Storage, issue, true, false)
	// Original beads returns an array for show
	return json.NewEncoder(app.Out).Encode([]IssueJSON{out})
}
