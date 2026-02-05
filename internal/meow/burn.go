package meow

import (
	"context"
	"errors"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/routing"
)

// BurnResult contains statistics from a Burn operation.
type BurnResult struct {
	Deleted             []string    `json:"deleted"`
	DeletedCount        int         `json:"deleted_count"`
	DependenciesRemoved int         `json:"dependencies_removed"`
	EventsRemoved       int         `json:"events_removed"`
	LabelsRemoved       int         `json:"labels_removed"`
	OrphanedIssues      interface{} `json:"orphaned_issues"`
	ReferencesUpdated   int         `json:"references_updated"`
}

// Burn cascade-deletes a molecule and all its children.
//
// Behavior differs by issue phase:
//   - Ephemeral issues: hard-deleted with no trace
//   - Persistent issues: closed to create a tombstone record that syncs
//     to remotes via bd sync
//
// Deletion proceeds from leaves up to avoid dangling parent references.
func Burn(ctx context.Context, store *routing.IssueStore, molID string) (*BurnResult, error) {
	// 1. Load root issue.
	root, err := store.Get(ctx, molID)
	if err != nil {
		if errors.Is(err, issuestorage.ErrNotFound) {
			return nil, fmt.Errorf("molecule %s not found: %w", molID, err)
		}
		return nil, fmt.Errorf("load molecule %s: %w", molID, err)
	}

	// 2. Collect all children.
	children, err := graph.CollectMoleculeChildren(ctx, store, molID)
	if err != nil {
		return nil, fmt.Errorf("collect children of %s: %w", molID, err)
	}

	// 3. Build deletion order: leaves first, root last.
	// CollectMoleculeChildren returns BFS order; reverse for leaves-first.
	all := make([]*issuestorage.Issue, 0, len(children)+1)
	for i := len(children) - 1; i >= 0; i-- {
		all = append(all, children[i])
	}
	all = append(all, root)

	burnSet := make(map[string]bool, len(all))
	for _, issue := range all {
		burnSet[issue.ID] = true
	}

	// Count dependencies before deletion.
	depsRemoved := 0
	for _, issue := range all {
		depsRemoved += len(issue.Dependencies) + len(issue.Dependents)
	}
	// Avoid double-counting internal deps.
	internalDeps := 0
	for _, issue := range all {
		for _, dep := range issue.Dependencies {
			if burnSet[dep.ID] {
				internalDeps++
			}
		}
	}
	depsRemoved = internalDeps // Only count deps within the molecule.

	// Count events (approximate: each issue has create + possible status changes).
	eventsRemoved := 0
	for _, issue := range all {
		eventsRemoved++ // create event
		if issue.Status != issuestorage.StatusOpen {
			eventsRemoved++ // status change event
		}
		eventsRemoved += len(issue.Comments)
	}

	// 4-5. Delete each issue according to its Ephemeral flag.
	deletedIDs := make([]string, 0, len(all))
	for _, issue := range all {
		if err := cleanExternalDeps(ctx, store, issue, burnSet); err != nil {
			return nil, fmt.Errorf("clean external deps for %s: %w", issue.ID, err)
		}
		if err := burnIssue(ctx, store, issue); err != nil {
			return nil, fmt.Errorf("burn issue %s: %w", issue.ID, err)
		}
		deletedIDs = append(deletedIDs, issue.ID)
	}

	return &BurnResult{
		Deleted:             deletedIDs,
		DeletedCount:        len(deletedIDs),
		DependenciesRemoved: depsRemoved,
		EventsRemoved:       eventsRemoved,
		LabelsRemoved:       0,
		OrphanedIssues:      nil,
		ReferencesUpdated:   0,
	}, nil
}

// burnIssue removes a single issue. Ephemeral issues are hard-deleted with no
// trace. Persistent issues are closed, leaving a tombstone in the closed store.
func burnIssue(ctx context.Context, store issuestorage.IssueStore, issue *issuestorage.Issue) error {
	if issue.Ephemeral {
		return store.Delete(ctx, issue.ID)
	}
	// Persistent: close to create tombstone in closed/ directory.
	if issue.Status != issuestorage.StatusClosed {
		return store.Modify(ctx, issue.ID, func(i *issuestorage.Issue) error {
			i.Status = issuestorage.StatusClosed
			return nil
		})
	}
	return nil
}

// cleanExternalDeps removes dependency links between issue and any issues
// outside the burn set, preventing dangling references in surviving issues.
func cleanExternalDeps(ctx context.Context, store *routing.IssueStore, issue *issuestorage.Issue, burnSet map[string]bool) error {
	for _, dep := range issue.Dependencies {
		if !burnSet[dep.ID] {
			if err := store.RemoveDependency(ctx, issue.ID, dep.ID); err != nil && !errors.Is(err, issuestorage.ErrNotFound) {
				return err
			}
		}
	}
	for _, dep := range issue.Dependents {
		if !burnSet[dep.ID] {
			if err := store.RemoveDependency(ctx, dep.ID, issue.ID); err != nil && !errors.Is(err, issuestorage.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}
