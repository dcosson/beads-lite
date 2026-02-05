package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// softDelete converts an issue to a tombstone (soft-delete) via Modify.
// Sets status to tombstone, records deletion metadata, and moves the issue
// to deleted storage. Returns ErrAlreadyTombstoned if already tombstoned.
func softDelete(ctx context.Context, store issuestorage.IssueStore, id string, actor string, reason string) error {
	return store.Modify(ctx, id, func(issue *issuestorage.Issue) error {
		if issue.Status == issuestorage.StatusTombstone {
			return issuestorage.ErrAlreadyTombstoned
		}
		issue.OriginalType = issue.Type
		issue.Status = issuestorage.StatusTombstone
		now := time.Now()
		issue.DeletedAt = &now
		issue.DeletedBy = actor
		issue.DeleteReason = reason
		return nil
	})
}

// deleteResult holds the JSON output of a delete operation.
type deleteResult struct {
	DeletedCount        int  `json:"deleted_count"`
	EventsRemoved       int  `json:"events_removed"`
	TotalCount          int  `json:"total_count"`
	DependenciesRemoved int  `json:"dependencies_removed,omitempty"`
	DryRun              bool `json:"dry_run,omitempty"`
	IssueCount          int  `json:"issue_count,omitempty"`
}

// newDeleteCmd creates the delete command.
func newDeleteCmd(provider *AppProvider) *cobra.Command {
	var (
		force    bool
		cascade  bool
		hard     bool
		dryRun   bool
		reason   string
		fromFile string
	)

	cmd := &cobra.Command{
		Use:   "delete <issue-id>",
		Short: "Delete an issue (soft-delete by default)",
		Long: `Delete an issue from the system.

By default, issues are soft-deleted (tombstoned): the issue is moved to a
deleted/ directory with status=tombstone and deletion metadata preserved.
Tombstoned issues are excluded from normal queries but remain retrievable
via 'bd show' and 'bd list --status tombstone'.

Use --hard to permanently remove the issue file (irreversible).

Supports prefix matching on IDs. If the provided ID is a unique prefix
of an existing issue ID, that issue will be deleted.

With --cascade, all issues that depend on the deleted issue will also
be deleted, recursively following the dependent chain.

Examples:
  bd delete bd-a1b2                       # Soft-delete (tombstone)
  bd delete bd-a1b2 --hard                # Permanently delete
  bd delete bd-a1b2 --force               # Skip confirmation
  bd delete bd-a1b2 --reason "duplicate"  # Record deletion reason
  bd delete bd-a1b2 --cascade --force     # Cascade to dependents
  bd delete bd-a1b2 --dry-run             # Preview without deleting
  bd delete bd-a1b2 --from-file ids.txt   # Delete multiple from file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			idPrefix := args[0]

			store := app.Storage

			// Try exact match first
			issue, err := store.Get(ctx, idPrefix)
			if err == issuestorage.ErrNotFound {
				// Try prefix matching
				issue, err = findByPrefix(store, ctx, idPrefix)
			}
			if err != nil {
				if err == issuestorage.ErrNotFound {
					return fmt.Errorf("issue not found: %s", idPrefix)
				}
				return fmt.Errorf("finding issue: %w", err)
			}

			// Collect issues to delete
			toDelete := []string{issue.ID}
			depsRemoved := 0

			// Read additional IDs from file if specified
			if fromFile != "" {
				fileIDs, err := readIDsFromFile(fromFile)
				if err != nil {
					return fmt.Errorf("reading IDs from file: %w", err)
				}
				for _, fid := range fileIDs {
					resolved, err := store.Get(ctx, fid)
					if err == issuestorage.ErrNotFound {
						resolved, err = findByPrefix(store, ctx, fid)
					}
					if err != nil {
						return fmt.Errorf("resolving ID %q from file: %w", fid, err)
					}
					toDelete = append(toDelete, resolved.ID)
				}
			}

			if cascade {
				// Collect all dependent issues recursively
				visited := make(map[string]bool)
				toDelete, err = collectDependentsRecursive(ctx, store, issue.ID, visited)
				if err != nil {
					return fmt.Errorf("collecting dependents: %w", err)
				}
			}

			// Build set of issues being deleted for cleanup
			deleteSet := make(map[string]bool)
			for _, id := range toDelete {
				deleteSet[id] = true
			}

			// Dry run: preview and exit
			if dryRun {
				if app.JSON {
					result := deleteResult{
						DeletedCount:  len(toDelete),
						EventsRemoved: len(toDelete) + depsRemoved,
						TotalCount:    1,
						IssueCount:    len(toDelete),
						DryRun:        true,
					}
					return json.NewEncoder(app.Out).Encode(result)
				}
				action := "Tombstone"
				if hard {
					action = "Permanently delete"
				}
				if len(toDelete) == 1 {
					fmt.Fprintf(app.Out, "[dry-run] Would %s: %s\n", strings.ToLower(action), issue.ID)
				} else {
					fmt.Fprintf(app.Out, "[dry-run] Would %s %d issues:\n", strings.ToLower(action), len(toDelete))
					for _, id := range toDelete {
						fmt.Fprintf(app.Out, "  - %s\n", id)
					}
				}
				return nil
			}

			// Confirmation prompt unless --force is used
			if !force {
				action := "Tombstone"
				if hard {
					action = "Permanently delete"
				}
				if len(toDelete) == 1 {
					fmt.Fprintf(app.Out, "%s issue %s: %s? [y/N] ", action, issue.ID, issue.Title)
				} else {
					fmt.Fprintf(app.Out, "%s %d issues (%s and %d dependents)? [y/N] ",
						action, len(toDelete), issue.ID, len(toDelete)-1)
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

			// Rewrite text references in surviving issues (must happen before
			// dependency cleanup since it uses dependency links to find connected issues)
			refsUpdated := 0
			for _, id := range toDelete {
				n := rewriteTextReferences(ctx, store, id, deleteSet)
				refsUpdated += n
			}

			// Count all dependencies involving deleted issues, and clean up external ones
			for _, id := range toDelete {
				issueToDelete, err := store.Get(ctx, id)
				if err != nil {
					continue // Issue might already be processed
				}

				// Count all dependencies this issue has
				depsRemoved += len(issueToDelete.Dependencies)

				// Clean up dependencies to non-deleted issues
				for _, dep := range issueToDelete.Dependencies {
					if !deleteSet[dep.ID] {
						if err := store.RemoveDependency(ctx, id, dep.ID); err != nil {
							// Ignore errors - issue might already be cleaned up
						}
					}
				}

				// Clean up dependents from non-deleted issues
				for _, dep := range issueToDelete.Dependents {
					if !deleteSet[dep.ID] {
						if err := store.RemoveDependency(ctx, dep.ID, id); err != nil {
							// Ignore errors
						}
					}
				}
			}

			// Delete or tombstone all collected issues
			for _, id := range toDelete {
				if hard {
					if err := store.Delete(ctx, id); err != nil {
						// Continue trying to delete others even if one fails
					}
				} else {
					deleteReason := reason
					if deleteReason == "" {
						deleteReason = "batch delete"
					}
					actor := "batch delete"
					if err := softDelete(ctx, store, id, actor, deleteReason); err != nil {
						// Continue trying to process others even if one fails
					}
				}
			}

			// Output the result
			if app.JSON {
				result := deleteResult{
					DeletedCount:        len(toDelete),
					EventsRemoved:       len(toDelete) + depsRemoved,
					TotalCount:          1,
					DependenciesRemoved: depsRemoved,
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			action := "Tombstoned"
			if hard {
				action = "Deleted"
			}

			if len(toDelete) == 1 {
				fmt.Fprintf(app.Out, "%s %s\n", action, issue.ID)
			} else {
				fmt.Fprintf(app.Out, "%s %d issues (cascade from %s)\n", action, len(toDelete), issue.ID)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "Recursively delete all dependent issues")
	cmd.Flags().BoolVar(&hard, "hard", false, "Permanently delete instead of tombstoning")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be deleted without doing it")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for deletion (stored in tombstone)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read additional issue IDs from file (one per line)")

	return cmd
}

// collectDependentsRecursive collects all issues that depend on the given issue,
// following the dependent chain recursively.
func collectDependentsRecursive(ctx context.Context, store issuestorage.IssueStore, issueID string, visited map[string]bool) ([]string, error) {
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
// Tombstoned issues are excluded from prefix matching.
func findByPrefix(store issuestorage.IssueStore, ctx context.Context, prefix string) (*issuestorage.Issue, error) {
	// List all issues (both open and closed, excluding tombstones)
	openIssues, err := store.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	closedStatus := issuestorage.StatusClosed
	closedIssues, err := store.List(ctx, &issuestorage.ListFilter{Status: &closedStatus})
	if err != nil {
		return nil, err
	}

	allIssues := append(openIssues, closedIssues...)

	var matches []*issuestorage.Issue
	for _, issue := range allIssues {
		if strings.HasPrefix(issue.ID, prefix) {
			matches = append(matches, issue)
		}
	}

	if len(matches) == 0 {
		return nil, issuestorage.ErrNotFound
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

// readIDsFromFile reads issue IDs from a file, one per line.
// Empty lines and lines starting with # are skipped.
func readIDsFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	return ids, nil
}

// rewriteTextReferences replaces references to deletedID in surviving issues'
// descriptions with [deleted:ID] format. Returns the number of references updated.
func rewriteTextReferences(ctx context.Context, store issuestorage.IssueStore, deletedID string, deleteSet map[string]bool) int {
	// Get the issue to find its connected issues
	issue, err := store.Get(ctx, deletedID)
	if err != nil {
		return 0
	}

	// Collect all connected surviving issue IDs
	connectedIDs := make(map[string]bool)
	for _, dep := range issue.Dependencies {
		if !deleteSet[dep.ID] {
			connectedIDs[dep.ID] = true
		}
	}
	for _, dep := range issue.Dependents {
		if !deleteSet[dep.ID] {
			connectedIDs[dep.ID] = true
		}
	}

	// Build regex pattern for the deleted ID
	pattern := regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(deletedID) + `($|[^A-Za-z0-9_-])`)
	replacement := "${1}[deleted:" + deletedID + "]${2}"

	count := 0
	for connID := range connectedIDs {
		connIssue, err := store.Get(ctx, connID)
		if err != nil {
			continue
		}

		newDesc := pattern.ReplaceAllString(connIssue.Description, replacement)
		if newDesc != connIssue.Description {
			store.Modify(ctx, connID, func(i *issuestorage.Issue) error {
				i.Description = pattern.ReplaceAllString(i.Description, replacement)
				return nil
			})
			count++
		}
	}

	return count
}
