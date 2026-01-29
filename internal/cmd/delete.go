package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// deleteResultSimple holds the outcome of a simple delete operation (no cascade).
type deleteResultSimple struct {
	Deleted             string `json:"deleted"`
	DependenciesRemoved int    `json:"dependencies_removed"`
	ReferencesUpdated   int    `json:"references_updated"`
}

// deleteResultCascade holds the outcome of a cascade delete operation.
type deleteResultCascade struct {
	Deleted             []string `json:"deleted"`
	DeletedCount        int      `json:"deleted_count"`
	DependenciesRemoved int      `json:"dependencies_removed"`
	EventsRemoved       int      `json:"events_removed"`
	LabelsRemoved       int      `json:"labels_removed"`
	OrphanedIssues      []string `json:"orphaned_issues"` // null when nil
	ReferencesUpdated   int      `json:"references_updated"`
}

// newDeleteCmd creates the delete command.
func newDeleteCmd(provider *AppProvider) *cobra.Command {
	var (
		force   bool
		cascade bool
	)

	cmd := &cobra.Command{
		Use:   "delete <issue-id>",
		Short: "Permanently delete an issue",
		Long: `Permanently delete an issue from the system.

This action is irreversible. By default, a confirmation prompt is shown.
Use --force to skip the confirmation.

Supports prefix matching on IDs. If the provided ID is a unique prefix
of an existing issue ID, that issue will be deleted.

With --cascade, all issues that depend on the deleted issue will also
be deleted, recursively following the dependent chain.

Examples:
  bd delete bd-a1b2                  # Delete by full ID
  bd delete bd-a1                    # Delete by prefix (if unique)
  bd delete bd-a1b2 --force          # Delete without confirmation
  bd delete bd-a1b2 --cascade --force # Delete and cascade to dependents`,
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

			// Collect issues to delete
			toDelete := []string{issue.ID}
			depsRemoved := 0

			if cascade {
				// Collect all dependent issues recursively
				visited := make(map[string]bool)
				toDelete, err = collectDependentsRecursive(ctx, app.Storage, issue.ID, visited)
				if err != nil {
					return fmt.Errorf("collecting dependents: %w", err)
				}
			}

			// Confirmation prompt unless --force is used
			if !force {
				if len(toDelete) == 1 {
					fmt.Fprintf(app.Out, "Delete issue %s: %s? [y/N] ", issue.ID, issue.Title)
				} else {
					fmt.Fprintf(app.Out, "Delete %d issues (%s and %d dependents)? [y/N] ",
						len(toDelete), issue.ID, len(toDelete)-1)
				}

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

			// Build set of issues being deleted for cleanup
			deleteSet := make(map[string]bool)
			for _, id := range toDelete {
				deleteSet[id] = true
			}

			// Count all dependencies involving deleted issues, and clean up external ones
			for _, id := range toDelete {
				issueToDelete, err := app.Storage.Get(ctx, id)
				if err != nil {
					continue // Issue might already be processed
				}

				// Count all dependencies this issue has
				depsRemoved += len(issueToDelete.Dependencies)

				// Clean up dependencies to non-deleted issues
				for _, dep := range issueToDelete.Dependencies {
					if !deleteSet[dep.ID] {
						// Remove this issue from the dependency target's Dependents list
						if err := app.Storage.RemoveDependency(ctx, id, dep.ID); err != nil {
							// Ignore errors - issue might already be cleaned up
						}
					}
				}

				// Clean up dependents from non-deleted issues (don't count - would double-count)
				for _, dep := range issueToDelete.Dependents {
					if !deleteSet[dep.ID] {
						// Remove dependency from the dependent issue
						if err := app.Storage.RemoveDependency(ctx, dep.ID, id); err != nil {
							// Ignore errors
						}
					}
				}
			}

			// Delete all collected issues
			for _, id := range toDelete {
				if err := app.Storage.Delete(ctx, id); err != nil {
					// Continue trying to delete others even if one fails
				}
			}

			// Output the result
			if app.JSON {
				if cascade {
					// Cascade delete has a more detailed output format
					result := deleteResultCascade{
						Deleted:             []string{issue.ID}, // Only include the requested issue ID
						DeletedCount:        len(toDelete),
						DependenciesRemoved: depsRemoved,
						EventsRemoved:       len(toDelete) + depsRemoved, // Simplified: 1 per issue + 1 per dep
						LabelsRemoved:       0,
						OrphanedIssues:      nil, // null in JSON
						ReferencesUpdated:   0,
					}
					return json.NewEncoder(app.Out).Encode(result)
				}
				// Simple delete has a simpler output format
				result := deleteResultSimple{
					Deleted:             issue.ID,
					DependenciesRemoved: depsRemoved,
					ReferencesUpdated:   0,
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			if len(toDelete) == 1 {
				fmt.Fprintf(app.Out, "Deleted %s\n", issue.ID)
			} else {
				fmt.Fprintf(app.Out, "Deleted %d issues (cascade from %s)\n", len(toDelete), issue.ID)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "Recursively delete all dependent issues")

	return cmd
}

// collectDependentsRecursive collects all issues that depend on the given issue,
// following the dependent chain recursively.
func collectDependentsRecursive(ctx context.Context, store storage.Storage, issueID string, visited map[string]bool) ([]string, error) {
	if visited[issueID] {
		return nil, nil
	}
	visited[issueID] = true

	result := []string{issueID}

	issue, err := store.Get(ctx, issueID)
	if err != nil {
		return result, nil // Can't get dependents if issue doesn't exist
	}

	// Recursively collect all dependents
	for _, dep := range issue.Dependents {
		children, err := collectDependentsRecursive(ctx, store, dep.ID, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, children...)
	}

	return result, nil
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
