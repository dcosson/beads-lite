package meow

import (
	"context"
	"path/filepath"
	"testing"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func newMolStore(t *testing.T) issuestorage.IssueStore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func createMolIssue(t *testing.T, ctx context.Context, s issuestorage.IssueStore, title string, typ issuestorage.IssueType) *issuestorage.Issue {
	t.Helper()
	issue := &issuestorage.Issue{
		Title:    title,
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     typ,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create %q: %v", title, err)
	}
	issue.ID = id
	return issue
}

// buildTestMolecule creates: root (epic) -> [A, B, C] where B blocks->A, C blocks->B.
func buildTestMolecule(t *testing.T, ctx context.Context, s issuestorage.IssueStore) (root *issuestorage.Issue, children map[string]*issuestorage.Issue) {
	t.Helper()
	root = createMolIssue(t, ctx, s, "Test Molecule", issuestorage.TypeEpic)
	a := createMolIssue(t, ctx, s, "Step A", issuestorage.TypeTask)
	b := createMolIssue(t, ctx, s, "Step B", issuestorage.TypeTask)
	c := createMolIssue(t, ctx, s, "Step C", issuestorage.TypeTask)

	// Parent-child relationships.
	for _, child := range []*issuestorage.Issue{a, b, c} {
		if err := s.AddDependency(ctx, child.ID, root.ID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency parent-child: %v", err)
		}
	}

	// B blocked by A, C blocked by B.
	if err := s.AddDependency(ctx, b.ID, a.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency B->A: %v", err)
	}
	if err := s.AddDependency(ctx, c.ID, b.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency C->B: %v", err)
	}

	children = map[string]*issuestorage.Issue{
		"A": a,
		"B": b,
		"C": c,
	}
	return root, children
}

func TestCurrent_ShowsCorrectStatusMarkers(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)
	root, children := buildTestMolecule(t, ctx, s)

	// Initial state: A=ready, B=blocked (by A), C=blocked (by B).
	view, err := Current(ctx, s, CurrentOptions{MoleculeID: root.ID})
	if err != nil {
		t.Fatalf("Current: %v", err)
	}

	if view.RootID != root.ID {
		t.Errorf("RootID: got %s, want %s", view.RootID, root.ID)
	}
	if view.Title != "Test Molecule" {
		t.Errorf("Title: got %q, want %q", view.Title, "Test Molecule")
	}
	if len(view.Steps) != 3 {
		t.Fatalf("Steps: got %d, want 3", len(view.Steps))
	}

	byID := make(map[string]StepView)
	for _, step := range view.Steps {
		byID[step.ID] = step
	}

	assertStepStatus(t, byID, children["A"].ID, graph.StepReady)
	assertStepStatus(t, byID, children["B"].ID, graph.StepBlocked)
	assertStepStatus(t, byID, children["C"].ID, graph.StepBlocked)

	// Close A: B should become ready.
	if err := s.Modify(ctx, children["A"].ID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("Close A: %v", err)
	}

	view, err = Current(ctx, s, CurrentOptions{MoleculeID: root.ID})
	if err != nil {
		t.Fatalf("Current after closing A: %v", err)
	}
	byID = make(map[string]StepView)
	for _, step := range view.Steps {
		byID[step.ID] = step
	}
	assertStepStatus(t, byID, children["A"].ID, graph.StepDone)
	assertStepStatus(t, byID, children["B"].ID, graph.StepReady)
	assertStepStatus(t, byID, children["C"].ID, graph.StepBlocked)
}

func TestCurrent_InProgressMarkedAsCurrent(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)
	root, children := buildTestMolecule(t, ctx, s)

	// Set A to in_progress.
	if err := s.Modify(ctx, children["A"].ID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusInProgress
		return nil
	}); err != nil {
		t.Fatalf("Update A: %v", err)
	}

	view, err := Current(ctx, s, CurrentOptions{MoleculeID: root.ID})
	if err != nil {
		t.Fatalf("Current: %v", err)
	}

	byID := make(map[string]StepView)
	for _, step := range view.Steps {
		byID[step.ID] = step
	}
	assertStepStatus(t, byID, children["A"].ID, graph.StepCurrent)
}

func TestCurrent_NonExistentMolecule(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)

	_, err := Current(ctx, s, CurrentOptions{MoleculeID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent molecule")
	}
}

func TestProgress_ReturnsCorrectCounts(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)
	root, children := buildTestMolecule(t, ctx, s)

	// Initial: 0 done, 0 in_progress, 1 ready (A), 2 blocked (B, C).
	stats, err := Progress(ctx, s, root.ID)
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("Total: got %d, want 3", stats.Total)
	}
	if stats.Completed != 0 {
		t.Errorf("Completed: got %d, want 0", stats.Completed)
	}
	if stats.Ready != 1 {
		t.Errorf("Ready: got %d, want 1", stats.Ready)
	}
	if stats.Blocked != 2 {
		t.Errorf("Blocked: got %d, want 2", stats.Blocked)
	}
	if stats.Percent != 0 {
		t.Errorf("Percent: got %f, want 0", stats.Percent)
	}

	// Close A: 1 done, 1 ready (B), 1 blocked (C).
	if err := s.Modify(ctx, children["A"].ID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("Close A: %v", err)
	}

	stats, err = Progress(ctx, s, root.ID)
	if err != nil {
		t.Fatalf("Progress after close: %v", err)
	}
	if stats.Completed != 1 {
		t.Errorf("Completed: got %d, want 1", stats.Completed)
	}
	if stats.Ready != 1 {
		t.Errorf("Ready: got %d, want 1", stats.Ready)
	}
	if stats.Blocked != 1 {
		t.Errorf("Blocked: got %d, want 1", stats.Blocked)
	}

	wantPercent := float64(1) / float64(3) * 100
	if stats.Percent != wantPercent {
		t.Errorf("Percent: got %f, want %f", stats.Percent, wantPercent)
	}
}

func TestProgress_NonExistentMolecule(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)

	_, err := Progress(ctx, s, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent molecule")
	}
}

func TestFindStaleSteps_FindsReadyButUnstarted(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)
	root, _ := buildTestMolecule(t, ctx, s)

	// A is ready but open (not started) = stale.
	stale, err := FindStaleSteps(ctx, s, root.ID)
	if err != nil {
		t.Fatalf("FindStaleSteps: %v", err)
	}

	if len(stale) != 1 {
		t.Fatalf("expected 1 stale step, got %d", len(stale))
	}
	if stale[0].Reason != "ready but not started" {
		t.Errorf("reason: got %q, want %q", stale[0].Reason, "ready but not started")
	}
}

func TestFindStaleSteps_NoStaleWhenInProgress(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)
	root, children := buildTestMolecule(t, ctx, s)

	// Set A to in_progress — no longer stale.
	if err := s.Modify(ctx, children["A"].ID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusInProgress
		return nil
	}); err != nil {
		t.Fatalf("Update A: %v", err)
	}

	stale, err := FindStaleSteps(ctx, s, root.ID)
	if err != nil {
		t.Fatalf("FindStaleSteps: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected 0 stale steps when A is in_progress, got %d", len(stale))
	}
}

func TestInferMolecule_FindsEpicForActor(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)

	// Create an in_progress root epic assigned to "alice".
	epic := &issuestorage.Issue{
		Title:    "Alice's Molecule",
		Status:   issuestorage.StatusInProgress,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
		Assignee: "alice",
	}
	epicID, err := s.Create(ctx, epic)
	if err != nil {
		t.Fatalf("Create epic: %v", err)
	}

	got, err := InferMolecule(ctx, s, "alice")
	if err != nil {
		t.Fatalf("InferMolecule: %v", err)
	}
	if got != epicID {
		t.Errorf("got %s, want %s", got, epicID)
	}
}

func TestInferMolecule_NoEpicForActor(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)

	_, err := InferMolecule(ctx, s, "nobody")
	if err == nil {
		t.Fatal("expected error for actor with no molecule")
	}
	if got := err.Error(); got == "" {
		t.Error("expected descriptive error message")
	}
}

func TestInferMolecule_IgnoresChildEpics(t *testing.T) {
	ctx := context.Background()
	s := newMolStore(t)

	// Root epic assigned to alice (in_progress).
	root := &issuestorage.Issue{
		Title:    "Root",
		Status:   issuestorage.StatusInProgress,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
		Assignee: "alice",
	}
	rootID, err := s.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create root: %v", err)
	}

	// Child epic (has parent) — should not be returned.
	child := &issuestorage.Issue{
		Title:    "Child Epic",
		Status:   issuestorage.StatusInProgress,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
		Assignee: "alice",
	}
	childID, err := s.Create(ctx, child)
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}
	if err := s.AddDependency(ctx, childID, rootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	got, err := InferMolecule(ctx, s, "alice")
	if err != nil {
		t.Fatalf("InferMolecule: %v", err)
	}
	if got != rootID {
		t.Errorf("got %s, want root %s (not child %s)", got, rootID, childID)
	}
}

func assertStepStatus(t *testing.T, byID map[string]StepView, id string, want graph.StepStatus) {
	t.Helper()
	step, ok := byID[id]
	if !ok {
		t.Errorf("step %s not found in view", id)
		return
	}
	if step.Status != want {
		t.Errorf("step %s status: got %s, want %s", id, step.Status, want)
	}
}
