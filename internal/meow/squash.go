package meow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"beads-lite/internal/graph"
	"beads-lite/internal/storage"
)

// SquashOptions configures a Squash operation (digest creation).
type SquashOptions struct {
	MoleculeID   string
	Summary      string
	KeepChildren bool
}

// SquashResult describes the outcome of a Squash.
type SquashResult struct {
	DigestID     string   `json:"digest_id"`
	SquashedIDs  []string `json:"squashed_ids"`
	KeepChildren bool     `json:"keep_children"`
}

// Squash creates a permanent digest issue from ephemeral wisp children,
// then either promotes or deletes those children.
//
// If the molecule has no ephemeral children, Squash is a no-op and returns
// (nil, nil). The caller can print "No ephemeral children found" in that case.
func Squash(ctx context.Context, store storage.Storage, opts SquashOptions) (*SquashResult, error) {
	// 1. Load root issue.
	root, err := store.Get(ctx, opts.MoleculeID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("molecule %s not found: %w", opts.MoleculeID, err)
		}
		return nil, fmt.Errorf("load molecule %s: %w", opts.MoleculeID, err)
	}

	// 2. Collect children.
	children, err := graph.CollectMoleculeChildren(ctx, store, opts.MoleculeID)
	if err != nil {
		return nil, fmt.Errorf("collect children of %s: %w", opts.MoleculeID, err)
	}

	// 3. Filter to ephemeral children only.
	var ephemeral []*storage.Issue
	for _, child := range children {
		if child.Ephemeral {
			ephemeral = append(ephemeral, child)
		}
	}

	// 4. No ephemeral children → no-op.
	if len(ephemeral) == 0 {
		return nil, nil
	}

	// 5. Create digest issue as child of root.
	digestID, err := store.GetNextChildID(ctx, root.ID)
	if err != nil {
		return nil, fmt.Errorf("get next child ID for %s: %w", root.ID, err)
	}

	summary := opts.Summary
	if summary == "" {
		summary = autoSummary(ephemeral)
	}

	digest := &storage.Issue{
		ID:          digestID,
		Title:       fmt.Sprintf("Digest: %s", root.Title),
		Description: summary,
		Status:      storage.StatusOpen,
		Priority:    storage.PriorityMedium,
		Type:        storage.TypeTask,
		Ephemeral:   false,
		CloseReason: fmt.Sprintf("Squashed from %d wisps", len(ephemeral)),
	}

	if _, err := store.Create(ctx, digest); err != nil {
		return nil, fmt.Errorf("create digest issue: %w", err)
	}

	// Link digest as child of root.
	if err := store.AddDependency(ctx, digestID, root.ID, storage.DepTypeParentChild); err != nil {
		return nil, fmt.Errorf("add parent-child dep for digest: %w", err)
	}

	// Close the digest immediately. store.Close overwrites CloseReason,
	// so re-apply our squash-specific reason afterwards.
	closeReason := digest.CloseReason
	if err := store.Close(ctx, digestID); err != nil {
		return nil, fmt.Errorf("close digest issue: %w", err)
	}
	closed, err := store.Get(ctx, digestID)
	if err != nil {
		return nil, fmt.Errorf("reload closed digest: %w", err)
	}
	closed.CloseReason = closeReason
	if err := store.Update(ctx, closed); err != nil {
		return nil, fmt.Errorf("set digest close reason: %w", err)
	}

	// 6. Post-squash: handle ephemeral children.
	squashedIDs := make([]string, 0, len(ephemeral))
	for _, child := range ephemeral {
		squashedIDs = append(squashedIDs, child.ID)
	}

	if opts.KeepChildren {
		// Promote ephemeral children to persistent.
		for _, child := range ephemeral {
			child.Ephemeral = false
			if err := store.Update(ctx, child); err != nil {
				return nil, fmt.Errorf("promote child %s: %w", child.ID, err)
			}
		}
	} else {
		// Hard-delete all ephemeral children (no tombstones).
		for _, child := range ephemeral {
			if err := store.Delete(ctx, child.ID); err != nil {
				return nil, fmt.Errorf("delete ephemeral child %s: %w", child.ID, err)
			}
		}
	}

	return &SquashResult{
		DigestID:     digestID,
		SquashedIDs:  squashedIDs,
		KeepChildren: opts.KeepChildren,
	}, nil
}

// autoSummary generates a description from ephemeral child titles.
func autoSummary(children []*storage.Issue) string {
	lines := make([]string, 0, len(children))
	for _, child := range children {
		lines = append(lines, fmt.Sprintf("- %s", child.Title))
	}
	return strings.Join(lines, "\n")
}
