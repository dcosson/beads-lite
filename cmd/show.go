package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"beads2/storage"

	"github.com/spf13/cobra"
)

// NewShowCmd creates the show command.
func NewShowCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display an issue's details",
		Long: `Display an issue's details including title, description, status,
priority, type, parent, children, dependencies, labels, assignee,
comments, and timestamps.

Supports prefix matching - 'bd-a1' will match 'bd-a1b2' if unique.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

			issue, err := app.ResolveID(ctx, id)
			if err != nil {
				return err
			}

			if app.JSON {
				return outputJSON(app.Out, issue)
			}

			return outputHuman(app.Out, issue)
		},
	}

	return cmd
}

func outputJSON(w io.Writer, issue *storage.Issue) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(issue)
}

func outputHuman(w io.Writer, issue *storage.Issue) error {
	// Status indicator
	statusIcon := statusToIcon(issue.Status)
	priorityIcon := priorityToIcon(issue.Priority)

	// Header line: icon ID - Title [priority icon - status]
	fmt.Fprintf(w, "%s %s - %s   [%s %s - %s]\n",
		statusIcon, issue.ID, issue.Title,
		priorityIcon, issue.Priority, issue.Status)

	// Type and assignee
	if issue.Assignee != "" {
		fmt.Fprintf(w, "Type: %s   Assignee: %s\n", issue.Type, issue.Assignee)
	} else {
		fmt.Fprintf(w, "Type: %s\n", issue.Type)
	}

	// Timestamps
	fmt.Fprintf(w, "Created: %s   Updated: %s\n",
		formatTime(issue.CreatedAt), formatTime(issue.UpdatedAt))
	if issue.ClosedAt != nil {
		fmt.Fprintf(w, "Closed: %s\n", formatTime(*issue.ClosedAt))
	}

	// Labels
	if len(issue.Labels) > 0 {
		fmt.Fprintf(w, "Labels: %s\n", strings.Join(issue.Labels, ", "))
	}

	// Description
	if issue.Description != "" {
		fmt.Fprintf(w, "\nDESCRIPTION\n%s\n", issue.Description)
	}

	// Hierarchy
	if issue.Parent != "" {
		fmt.Fprintf(w, "\nPARENT\n  %s\n", issue.Parent)
	}
	if len(issue.Children) > 0 {
		fmt.Fprintf(w, "\nCHILDREN\n")
		for _, child := range issue.Children {
			fmt.Fprintf(w, "  %s\n", child)
		}
	}

	// Dependencies
	if len(issue.DependsOn) > 0 {
		fmt.Fprintf(w, "\nDEPENDS ON\n")
		for _, dep := range issue.DependsOn {
			fmt.Fprintf(w, "  -> %s\n", dep)
		}
	}
	if len(issue.Dependents) > 0 {
		fmt.Fprintf(w, "\nDEPENDENTS\n")
		for _, dep := range issue.Dependents {
			fmt.Fprintf(w, "  <- %s\n", dep)
		}
	}

	// Blocking
	if len(issue.Blocks) > 0 {
		fmt.Fprintf(w, "\nBLOCKS\n")
		for _, b := range issue.Blocks {
			fmt.Fprintf(w, "  -> %s\n", b)
		}
	}
	if len(issue.BlockedBy) > 0 {
		fmt.Fprintf(w, "\nBLOCKED BY\n")
		for _, b := range issue.BlockedBy {
			fmt.Fprintf(w, "  <- %s\n", b)
		}
	}

	// Comments
	if len(issue.Comments) > 0 {
		fmt.Fprintf(w, "\nCOMMENTS (%d)\n", len(issue.Comments))
		for _, comment := range issue.Comments {
			fmt.Fprintf(w, "  [%s] %s (%s):\n",
				comment.ID, comment.Author, formatTime(comment.CreatedAt))
			// Indent comment body
			for _, line := range strings.Split(comment.Body, "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}

	return nil
}

func statusToIcon(status storage.Status) string {
	switch status {
	case storage.StatusOpen:
		return "○"
	case storage.StatusInProgress:
		return "◐"
	case storage.StatusBlocked:
		return "◌"
	case storage.StatusDeferred:
		return "◇"
	case storage.StatusClosed:
		return "✓"
	default:
		return "?"
	}
}

func priorityToIcon(priority storage.Priority) string {
	switch priority {
	case storage.PriorityCritical:
		return "🔴"
	case storage.PriorityHigh:
		return "🟠"
	case storage.PriorityMedium:
		return "🟡"
	case storage.PriorityLow:
		return "🟢"
	default:
		return "●"
	}
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}
