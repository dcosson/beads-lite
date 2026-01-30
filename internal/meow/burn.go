package meow

import (
	"context"
	"errors"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/storage"
)

// Burn cascade-deletes a molecule and all its children.
//
// Behavior differs by issue phase:
//   - Ephemeral issues: hard-deleted with no trace
//   - Persistent issues: closed to create a tombstone record that syncs
//     to remotes via bd sync
//
// Deletion proceeds from leaves up to avoid dangling parent references.
func Burn(ctx context.Context, store storage.Storage, molID string) error {
	// 1. Load root issue.
	root, err := store.Get(ctx, molID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("molecule %s not found: %w", molID, err)
		}
		return fmt.Errorf("load molecule %s: %w", molID, err)
	}

	// 2. Collect all children.
	children, err := graph.CollectMoleculeChildren(ctx, store, molID)
	if err != nil {
		return fmt.Errorf("collect children of %s: %w", molID, err)
	}

	// 3. Build deletion order: leaves first, root last.
	// CollectMoleculeChildren returns BFS order; reverse for leaves-first.
	all := make([]*storage.Issue, 0, len(children)+1)
	for i := len(children) - 1; i >= 0; i-- {
		all = append(all, children[i])
	}
	all = append(all, root)

	burnSet := make(map[string]bool, len(all))
	for _, issue := range all {
		burnSet[issue.ID] = true
	}

	// 4-5. Delete each issue according to its Ephemeral flag.
	for _, issue := range all {
		if err := cleanExternalDeps(ctx, store, issue, burnSet); err != nil {
			return fmt.Errorf("clean external deps for %s: %w", issue.ID, err)
		}
		if err := burnIssue(ctx, store, issue); err != nil {
			return fmt.Errorf("burn issue %s: %w", issue.ID, err)
		}
	}

	return nil
}

// burnIssue removes a single issue. Ephemeral issues are hard-deleted with no
// trace. Persistent issues are closed, leaving a tombstone in the closed store.
func burnIssue(ctx context.Context, store storage.Storage, issue *storage.Issue) error {
	if issue.Ephemeral {
		return store.Delete(ctx, issue.ID)
	}
	// Persistent: close to create tombstone in closed/ directory.
	if issue.Status != storage.StatusClosed {
		return store.Close(ctx, issue.ID)
	}
	return nil
}

// cleanExternalDeps removes dependency links between issue and any issues
// outside the burn set, preventing dangling references in surviving issues.
func cleanExternalDeps(ctx context.Context, store storage.Storage, issue *storage.Issue, burnSet map[string]bool) error {
	for _, dep := range issue.Dependencies {
		if !burnSet[dep.ID] {
			if err := store.RemoveDependency(ctx, issue.ID, dep.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
				return err
			}
		}
	}
	for _, dep := range issue.Dependents {
		if !burnSet[dep.ID] {
			if err := store.RemoveDependency(ctx, dep.ID, issue.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}
