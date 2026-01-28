package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"beads2/internal/storage"

	"github.com/spf13/cobra"
)

// newDeleteCmd creates the delete command.
func newDeleteCmd(provider *AppProvider) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <issue-id>",
		Short: "Permanently delete an issue",
		Long: `Permanently delete an issue from the system.

This action is irreversible. By default, a confirmation prompt is shown.
Use --force to skip the confirmation.

Supports prefix matching on IDs. If the provided ID is a unique prefix
of an existing issue ID, that issue will be deleted.

Examples:
  bd delete bd-a1b2           # Delete by full ID
  bd delete bd-a1             # Delete by prefix (if unique)
  bd delete bd-a1b2 --force   # Delete without confirmation`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			idPrefix := args[0]

			// Try exact match first
			issue, err := app.Storage.Get(ctx, idPrefix)
			if err == storage.ErrNotFound {
				// Try prefix matching
				issue, err = findByPrefix(app.Storage, ctx, idPrefix)
			}
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("issue not found: %s", idPrefix)
				}
				return fmt.Errorf("finding issue: %w", err)
			}

			// Confirmation prompt unless --force is used
			if !force {
				fmt.Fprintf(app.Out, "Delete issue %s: %s? [y/N] ", issue.ID, issue.Title)

				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading confirmation: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					fmt.Fprintln(app.Out, "Cancelled")
					return nil
				}
			}

			// Delete the issue
			if err := app.Storage.Delete(ctx, issue.ID); err != nil {
				return fmt.Errorf("deleting issue: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{"id": issue.ID, "status": "deleted"}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Deleted %s\n", issue.ID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// findByPrefix finds an issue by ID prefix.
// Returns ErrNotFound if no match, or an error if multiple matches.
func findByPrefix(store storage.Storage, ctx context.Context, prefix string) (*storage.Issue, error) {
	// List all issues (both open and closed)
	openIssues, err := store.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	closedStatus := storage.StatusClosed
	closedIssues, err := store.List(ctx, &storage.ListFilter{Status: &closedStatus})
	if err != nil {
		return nil, err
	}

	allIssues := append(openIssues, closedIssues...)

	var matches []*storage.Issue
	for _, issue := range allIssues {
		if strings.HasPrefix(issue.ID, prefix) {
			matches = append(matches, issue)
		}
	}

	if len(matches) == 0 {
		return nil, storage.ErrNotFound
	}
	if len(matches) > 1 {
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, fmt.Errorf("ambiguous prefix %q matches multiple issues: %s", prefix, strings.Join(ids, ", "))
	}

	return matches[0], nil
}
