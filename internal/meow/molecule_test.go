package meow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"beads-lite/internal/graph"
	"beads-lite/internal/storage"
	"beads-lite/internal/storage/filesystem"
)

func newMolStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	s := filesystem.New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func createMolIssue(t *testing.T, ctx context.Context, s storage.Storage, title string, typ storage.IssueType) *storage.Issue {
	t.Helper()
	issue := &storage.Issue{
		Title:    title,
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
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
func buildTestMolecule(t *testing.T, ctx context.Context, s storage.Storage) (root *storage.Issue, children map[string]*storage.Issue) {
	t.Helper()
	root = createMolIssue(t, ctx, s, "Test Molecule", storage.TypeEpic)
	a := createMolIssue(t, ctx, s, "Step A", storage.TypeTask)
	b := createMolIssue(t, ctx, s, "Step B", storage.TypeTask)
	c := createMolIssue(t, ctx, s, "Step C", storage.TypeTask)

	// Parent-child relationships.
	for _, child := range []*storage.Issue{a, b, c} {
		if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency parent-child: %v", err)
		}
	}

	// B blocked by A, C blocked by B.
	if err := s.AddDependency(ctx, b.ID, a.ID, storage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency B->A: %v", err)
	}
	if err := s.AddDependency(ctx, c.ID, b.ID, storage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency C->B: %v", err)
	}

	children = map[string]*storage.Issue{
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
	if err := s.Close(ctx, children["A"].ID); err != nil {
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
	a, err := s.Get(ctx, children["A"].ID)
	if err != nil {
		t.Fatalf("Get A: %v", err)
	}
	a.Status = storage.StatusInProgress
	if err := s.Update(ctx, a); err != nil {
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
	if err := s.Close(ctx, children["A"].ID); err != nil {
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
	a, err := s.Get(ctx, children["A"].ID)
	if err != nil {
		t.Fatalf("Get A: %v", err)
	}
	a.Status = storage.StatusInProgress
	if err := s.Update(ctx, a); err != nil {
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
	epic := &storage.Issue{
		Title:    "Alice's Molecule",
		Status:   storage.StatusInProgress,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeEpic,
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
	root := &storage.Issue{
		Title:    "Root",
		Status:   storage.StatusInProgress,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeEpic,
		Assignee: "alice",
	}
	rootID, err := s.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create root: %v", err)
	}

	// Child epic (has parent) — should not be returned.
	child := &storage.Issue{
		Title:    "Child Epic",
		Status:   storage.StatusInProgress,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeEpic,
		Assignee: "alice",
	}
	childID, err := s.Create(ctx, child)
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}
	if err := s.AddDependency(ctx, childID, rootID, storage.DepTypeParentChild); err != nil {
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

func TestResolveUser_BDActorEnv(t *testing.T) {
	t.Setenv("BD_ACTOR", "test-actor")
	got := ResolveUser()
	if got != "test-actor" {
		t.Errorf("ResolveUser: got %q, want %q", got, "test-actor")
	}
}

func TestResolveUser_FallsBackToUSER(t *testing.T) {
	t.Setenv("BD_ACTOR", "")
	// We can't easily control git config in tests, but we can verify
	// the function doesn't panic and returns something non-empty.
	got := ResolveUser()
	if got == "" {
		t.Error("ResolveUser returned empty string")
	}
}

func TestResolveUser_USEREnvFallback(t *testing.T) {
	t.Setenv("BD_ACTOR", "")
	// Set PATH to empty to ensure git config fails.
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", oldPath)
	t.Setenv("USER", "fallback-user")

	got := ResolveUser()
	if got != "fallback-user" {
		t.Errorf("ResolveUser: got %q, want %q", got, "fallback-user")
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
