package meow

import (
	"context"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"
)

// MoleculeView holds the full classified view of a molecule's steps.
type MoleculeView struct {
	RootID   string        `json:"root_id"`
	Title    string        `json:"title"`
	Steps    []StepView    `json:"steps"`
	Progress ProgressStats `json:"progress"`
}

// StepView describes a single step with its classified status.
type StepView struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	Status   graph.StepStatus    `json:"status"`
	Assignee string              `json:"assignee,omitempty"`
	Issue    *issuestorage.Issue `json:"-"` // full issue data for JSON conversion in cmd layer
}

// ProgressStats holds completion statistics for a molecule.
type ProgressStats struct {
	Total      int     `json:"total"`
	Completed  int     `json:"completed"`
	InProgress int     `json:"in_progress"`
	Blocked    int     `json:"blocked"`
	Ready      int     `json:"ready"`
	Pending    int     `json:"pending"`
	Percent    float64 `json:"percent"`
}

// Current returns the classified steps of a molecule.
// If opts.MoleculeID is empty, InferMolecule is used to find the active molecule.
// Returns (nil, nil) when inference finds no active molecule.
func Current(ctx context.Context, store issuestorage.IssueStore, opts CurrentOptions) (*MoleculeView, error) {
	molID := opts.MoleculeID
	if molID == "" {
		actor := opts.Actor
		if actor == "" {
			return nil, fmt.Errorf("actor is required when molecule ID is not provided")
		}
		inferred, err := InferMolecule(ctx, store, actor)
		if err != nil {
			return nil, fmt.Errorf("infer molecule: %w", err)
		}
		if inferred == "" {
			return nil, nil
		}
		molID = inferred
	}

	root, err := store.Get(ctx, molID)
	if err != nil {
		return nil, fmt.Errorf("get molecule root %s: %w", molID, err)
	}

	children, err := graph.CollectMoleculeChildren(ctx, store, molID)
	if err != nil {
		return nil, fmt.Errorf("collect children of %s: %w", molID, err)
	}

	closedSet, err := graph.BuildClosedSet(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("build closed set: %w", err)
	}

	classes := graph.ClassifySteps(children, closedSet)

	// Build ordered step views.
	ordered, err := graph.TopologicalOrder(children)
	if err != nil {
		// Fall back to unordered if cycle detected.
		ordered = children
	}

	steps := make([]StepView, 0, len(ordered))
	var stats ProgressStats
	stats.Total = len(ordered)

	for _, child := range ordered {
		status := classes[child.ID]

		switch status {
		case graph.StepDone:
			stats.Completed++
		case graph.StepCurrent:
			stats.InProgress++
		case graph.StepBlocked:
			stats.Blocked++
		case graph.StepReady:
			stats.Ready++
		case graph.StepPending:
			stats.Pending++
		}

		// Apply actor filter.
		if opts.Actor != "" && child.Assignee != opts.Actor {
			continue
		}

		steps = append(steps, StepView{
			ID:       child.ID,
			Title:    child.Title,
			Status:   status,
			Assignee: child.Assignee,
			Issue:    child,
		})
	}

	if stats.Total > 0 {
		stats.Percent = float64(stats.Completed) / float64(stats.Total) * 100
	}

	return &MoleculeView{
		RootID:   root.ID,
		Title:    root.Title,
		Steps:    steps,
		Progress: stats,
	}, nil
}

// Progress computes completion statistics for a molecule without loading
// the full step view.
func Progress(ctx context.Context, store issuestorage.IssueStore, molID string) (*ProgressStats, error) {
	children, err := graph.CollectMoleculeChildren(ctx, store, molID)
	if err != nil {
		return nil, fmt.Errorf("collect children of %s: %w", molID, err)
	}

	closedSet, err := graph.BuildClosedSet(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("build closed set: %w", err)
	}

	classes := graph.ClassifySteps(children, closedSet)

	var stats ProgressStats
	stats.Total = len(children)

	for _, status := range classes {
		switch status {
		case graph.StepDone:
			stats.Completed++
		case graph.StepCurrent:
			stats.InProgress++
		case graph.StepBlocked:
			stats.Blocked++
		case graph.StepReady:
			stats.Ready++
		case graph.StepPending:
			stats.Pending++
		}
	}

	if stats.Total > 0 {
		stats.Percent = float64(stats.Completed) / float64(stats.Total) * 100
	}

	return &stats, nil
}

// FindStaleSteps returns ready steps that appear idle — their dependencies
// are met but they haven't been started (status is open, not in_progress).
func FindStaleSteps(ctx context.Context, store issuestorage.IssueStore, molID string) ([]*StaleStep, error) {
	children, err := graph.CollectMoleculeChildren(ctx, store, molID)
	if err != nil {
		return nil, fmt.Errorf("collect children of %s: %w", molID, err)
	}

	closedSet, err := graph.BuildClosedSet(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("build closed set: %w", err)
	}

	ready := graph.FindReadySteps(children, closedSet)

	var stale []*StaleStep
	for _, step := range ready {
		if step.Status == issuestorage.StatusOpen {
			stale = append(stale, &StaleStep{
				ID:     step.ID,
				Title:  step.Title,
				Status: string(step.Status),
				Reason: "ready but not started",
			})
		}
	}

	return stale, nil
}

// InferMolecule finds the active molecule for the given actor using a two-step
// fallback strategy:
//  1. Find any in_progress issue assigned to the actor, walk up to the molecule root.
//  2. Find any hooked issue assigned to the actor, check its blocks deps for molecule roots.
//
// Returns ("", nil) when no molecule is found — callers should treat empty string
// as "nothing assigned" rather than an error.
func InferMolecule(ctx context.Context, store issuestorage.IssueStore, actor string) (string, error) {
	// Step 1: find via in_progress issues.
	if molID, err := findInProgressMolecules(ctx, store, actor); err != nil {
		return "", err
	} else if molID != "" {
		return molID, nil
	}

	// Step 2: fall back to hooked issues.
	if molID, err := findHookedMolecules(ctx, store, actor); err != nil {
		return "", err
	} else if molID != "" {
		return molID, nil
	}

	return "", nil
}

// findInProgressMolecules looks for in_progress issues assigned to the actor,
// walks each up to its molecule root, and returns the first root found.
func findInProgressMolecules(ctx context.Context, store issuestorage.IssueStore, actor string) (string, error) {
	status := issuestorage.StatusInProgress
	issues, err := store.List(ctx, &issuestorage.ListFilter{
		Status:   &status,
		Assignee: &actor,
	})
	if err != nil {
		return "", fmt.Errorf("list in_progress issues for %s: %w", actor, err)
	}

	seen := make(map[string]bool)
	for _, issue := range issues {
		root, err := graph.FindMoleculeRoot(ctx, store, issue.ID)
		if err != nil {
			return "", fmt.Errorf("find molecule root for %s: %w", issue.ID, err)
		}
		if seen[root.ID] {
			continue
		}
		seen[root.ID] = true
		return root.ID, nil
	}
	return "", nil
}

// findHookedMolecules looks for hooked issues assigned to the actor, checks
// their blocks dependencies to find molecule roots.
func findHookedMolecules(ctx context.Context, store issuestorage.IssueStore, actor string) (string, error) {
	status := issuestorage.StatusHooked
	issues, err := store.List(ctx, &issuestorage.ListFilter{
		Status:   &status,
		Assignee: &actor,
	})
	if err != nil {
		return "", fmt.Errorf("list hooked issues for %s: %w", actor, err)
	}

	depType := issuestorage.DepTypeBlocks
	seen := make(map[string]bool)
	for _, issue := range issues {
		for _, blocksID := range issue.DependencyIDs(&depType) {
			root, err := graph.FindMoleculeRoot(ctx, store, blocksID)
			if err != nil {
				// The blocks target may not exist; skip it.
				continue
			}
			if seen[root.ID] {
				continue
			}
			seen[root.ID] = true
			return root.ID, nil
		}
	}
	return "", nil
}

