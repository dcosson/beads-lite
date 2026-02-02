package meow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
func Current(ctx context.Context, store issuestorage.IssueStore, opts CurrentOptions) (*MoleculeView, error) {
	molID := opts.MoleculeID
	if molID == "" {
		actor := opts.Actor
		if actor == "" {
			actor = ResolveUser()
		}
		inferred, err := InferMolecule(ctx, store, actor)
		if err != nil {
			return nil, fmt.Errorf("infer molecule: %w", err)
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

// InferMolecule finds the active molecule for the given actor by looking for
// an in_progress epic with no parent that is assigned to the actor.
func InferMolecule(ctx context.Context, store issuestorage.IssueStore, actor string) (string, error) {
	status := issuestorage.StatusInProgress
	epicType := issuestorage.TypeEpic
	issues, err := store.List(ctx, &issuestorage.ListFilter{
		Status:   &status,
		Type:     &epicType,
		Assignee: &actor,
	})
	if err != nil {
		return "", fmt.Errorf("list in_progress epics for %s: %w", actor, err)
	}

	// Filter to root epics (no parent).
	var roots []*issuestorage.Issue
	for _, issue := range issues {
		if issue.Parent == "" {
			roots = append(roots, issue)
		}
	}

	if len(roots) == 0 {
		return "", fmt.Errorf("no in_progress molecule found for actor %q", actor)
	}

	return roots[0].ID, nil
}

// ResolveUser determines the current user identity using the priority:
//  1. BD_ACTOR env var
//  2. git config user.name
//  3. $USER (OS username)
func ResolveUser() string {
	if actor := os.Getenv("BD_ACTOR"); actor != "" {
		return actor
	}

	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			return name
		}
	}

	if user := os.Getenv("USER"); user != "" {
		return user
	}

	return "unknown"
}
