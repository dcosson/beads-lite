package graph

import (
	"context"
	"testing"

	"beads-lite/internal/issueservice"
	"beads-lite/internal/issuestorage"
)

// TestTopologicalWavesAcrossParents_NoCascade tests that without cascade,
// leaf tasks under different parents are all in the same wave (no cross-parent deps).
func TestTopologicalWavesAcrossParents_NoCascade(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)
	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	addPC(t, ctx, s, parentA.ID, root.ID)
	addPC(t, ctx, s, parentB.ID, root.ID)

	// ParentA blocked by ParentB (but cascade=false so ignored)
	addBlocks(t, ctx, s, parentA.ID, parentB.ID)

	taskA1 := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	addPC(t, ctx, s, taskA1.ID, parentA.ID)

	taskB1 := createIssue(t, ctx, s, "Task B1", issuestorage.TypeTask)
	addPC(t, ctx, s, taskB1.ID, parentB.ID)

	waves, byID, err := TopologicalWavesAcrossParents(ctx, s, root.ID, false)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave (no cascade), got %d: %v", len(waves), waves)
	}
	if len(waves[0]) != 2 {
		t.Errorf("wave 0: expected 2 tasks, got %d", len(waves[0]))
	}
	if byID == nil {
		t.Error("byID should not be nil")
	}
}

// TestTopologicalWavesAcrossParents_WithCascade tests that cascade injects
// synthetic edges between parent-blocked tasks.
func TestTopologicalWavesAcrossParents_WithCascade(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)
	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	addPC(t, ctx, s, parentA.ID, root.ID)
	addPC(t, ctx, s, parentB.ID, root.ID)

	// ParentA blocked by ParentB
	addBlocks(t, ctx, s, parentA.ID, parentB.ID)

	taskA1 := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	addPC(t, ctx, s, taskA1.ID, parentA.ID)

	taskB1 := createIssue(t, ctx, s, "Task B1", issuestorage.TypeTask)
	addPC(t, ctx, s, taskB1.ID, parentB.ID)

	waves, _, err := TopologicalWavesAcrossParents(ctx, s, root.ID, true)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves (cascade), got %d: %v", len(waves), waves)
	}
	// Wave 0: B1 (unblocked), Wave 1: A1 (blocked by B1 via cascade)
	if len(waves[0]) != 1 || waves[0][0] != taskB1.ID {
		t.Errorf("wave 0: expected [%s], got %v", taskB1.ID, waves[0])
	}
	if len(waves[1]) != 1 || waves[1][0] != taskA1.ID {
		t.Errorf("wave 1: expected [%s], got %v", taskA1.ID, waves[1])
	}
}

// TestTopologicalWavesAcrossParents_BlockingFrontierChain tests that the
// blocking frontier correctly identifies sinks in a chain.
func TestTopologicalWavesAcrossParents_BlockingFrontierChain(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)
	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	addPC(t, ctx, s, parentA.ID, root.ID)
	addPC(t, ctx, s, parentB.ID, root.ID)
	addBlocks(t, ctx, s, parentA.ID, parentB.ID)

	// ParentB has chain: B1 → B2 → B3
	taskB1 := createIssue(t, ctx, s, "B1", issuestorage.TypeTask)
	taskB2 := createIssue(t, ctx, s, "B2", issuestorage.TypeTask)
	taskB3 := createIssue(t, ctx, s, "B3", issuestorage.TypeTask)
	addPC(t, ctx, s, taskB1.ID, parentB.ID)
	addPC(t, ctx, s, taskB2.ID, parentB.ID)
	addPC(t, ctx, s, taskB3.ID, parentB.ID)
	addBlocks(t, ctx, s, taskB2.ID, taskB1.ID)
	addBlocks(t, ctx, s, taskB3.ID, taskB2.ID)

	taskA1 := createIssue(t, ctx, s, "A1", issuestorage.TypeTask)
	addPC(t, ctx, s, taskA1.ID, parentA.ID)

	waves, _, err := TopologicalWavesAcrossParents(ctx, s, root.ID, true)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}

	// B1→B2→B3 (3 waves for B chain), then A1 (wave 4)
	if len(waves) != 4 {
		t.Fatalf("expected 4 waves, got %d: %v", len(waves), waves)
	}
	// Wave 3 should be A1 (after B3 completes)
	if len(waves[3]) != 1 || waves[3][0] != taskA1.ID {
		t.Errorf("wave 3: expected [%s (A1)], got %v", taskA1.ID, waves[3])
	}
}

// TestTopologicalWavesAcrossParents_ParallelSinks tests that when the blocker
// has multiple independent tasks, all become part of the blocking frontier.
func TestTopologicalWavesAcrossParents_ParallelSinks(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)
	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	addPC(t, ctx, s, parentA.ID, root.ID)
	addPC(t, ctx, s, parentB.ID, root.ID)
	addBlocks(t, ctx, s, parentA.ID, parentB.ID)

	// ParentB has 3 independent tasks (all sinks)
	taskB1 := createIssue(t, ctx, s, "B1", issuestorage.TypeTask)
	taskB2 := createIssue(t, ctx, s, "B2", issuestorage.TypeTask)
	taskB3 := createIssue(t, ctx, s, "B3", issuestorage.TypeTask)
	addPC(t, ctx, s, taskB1.ID, parentB.ID)
	addPC(t, ctx, s, taskB2.ID, parentB.ID)
	addPC(t, ctx, s, taskB3.ID, parentB.ID)

	taskA1 := createIssue(t, ctx, s, "A1", issuestorage.TypeTask)
	addPC(t, ctx, s, taskA1.ID, parentA.ID)

	waves, _, err := TopologicalWavesAcrossParents(ctx, s, root.ID, true)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}

	// Wave 0: [B1, B2, B3], Wave 1: [A1]
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d: %v", len(waves), waves)
	}
	if len(waves[0]) != 3 {
		t.Errorf("wave 0: expected 3 tasks, got %d", len(waves[0]))
	}
	if len(waves[1]) != 1 {
		t.Errorf("wave 1: expected 1 task, got %d", len(waves[1]))
	}
}

// TestTopologicalWavesAcrossParents_DesignDocExample tests the exact example
// from the design doc.
func TestTopologicalWavesAcrossParents_DesignDocExample(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)
	subB := createIssue(t, ctx, s, "Sub-Parent B", issuestorage.TypeEpic)
	subC := createIssue(t, ctx, s, "Sub-Parent C", issuestorage.TypeEpic)

	addPC(t, ctx, s, subB.ID, root.ID)
	addPC(t, ctx, s, subC.ID, root.ID)

	// Sub-Parent B blocked by Sub-Parent C
	addBlocks(t, ctx, s, subB.ID, subC.ID)

	// Sub-Parent B: B1 → B2, B3 (B2 blocked by B1, B3 independent)
	b1 := createIssue(t, ctx, s, "B1", issuestorage.TypeTask)
	b2 := createIssue(t, ctx, s, "B2", issuestorage.TypeTask)
	b3 := createIssue(t, ctx, s, "B3", issuestorage.TypeTask)
	addPC(t, ctx, s, b1.ID, subB.ID)
	addPC(t, ctx, s, b2.ID, subB.ID)
	addPC(t, ctx, s, b3.ID, subB.ID)
	addBlocks(t, ctx, s, b2.ID, b1.ID)

	// Sub-Parent C: C1, C2 (independent)
	c1 := createIssue(t, ctx, s, "C1", issuestorage.TypeTask)
	c2 := createIssue(t, ctx, s, "C2", issuestorage.TypeTask)
	addPC(t, ctx, s, c1.ID, subC.ID)
	addPC(t, ctx, s, c2.ID, subC.ID)

	// Task D: standalone child of Root
	d := createIssue(t, ctx, s, "D", issuestorage.TypeTask)
	addPC(t, ctx, s, d.ID, root.ID)

	waves, _, err := TopologicalWavesAcrossParents(ctx, s, root.ID, true)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}

	// Expected:
	// Wave 0: [C1, C2, D]
	// Wave 1: [B1, B3]   (C done, B's roots unblocked; D already in wave 0)
	// Wave 2: [B2]       (depends on B1)
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d: %v", len(waves), waves)
	}

	wave0IDs := toSet(waves[0])
	wave1IDs := toSet(waves[1])
	wave2IDs := toSet(waves[2])

	// Wave 0: C1, C2, D
	if !wave0IDs[c1.ID] || !wave0IDs[c2.ID] || !wave0IDs[d.ID] {
		t.Errorf("wave 0: expected {C1, C2, D}, got %v", waves[0])
	}
	if len(waves[0]) != 3 {
		t.Errorf("wave 0: expected 3 items, got %d", len(waves[0]))
	}

	// Wave 1: B1, B3
	if !wave1IDs[b1.ID] || !wave1IDs[b3.ID] {
		t.Errorf("wave 1: expected {B1, B3}, got %v", waves[1])
	}
	if len(waves[1]) != 2 {
		t.Errorf("wave 1: expected 2 items, got %d", len(waves[1]))
	}

	// Wave 2: B2
	if !wave2IDs[b2.ID] {
		t.Errorf("wave 2: expected {B2}, got %v", waves[2])
	}
	if len(waves[2]) != 1 {
		t.Errorf("wave 2: expected 1 item, got %d", len(waves[2]))
	}
}

// TestTopologicalWavesAcrossParents_EmptyRoot tests empty results.
func TestTopologicalWavesAcrossParents_EmptyRoot(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)

	waves, _, err := TopologicalWavesAcrossParents(ctx, s, root.ID, true)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}
	if waves != nil {
		t.Errorf("expected nil waves for empty root, got %v", waves)
	}
}

// TestTopologicalWavesAcrossParents_StandaloneTasks tests with no parents.
func TestTopologicalWavesAcrossParents_StandaloneTasks(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	root := createIssue(t, ctx, s, "Root", issuestorage.TypeEpic)
	a := createIssue(t, ctx, s, "A", issuestorage.TypeTask)
	b := createIssue(t, ctx, s, "B", issuestorage.TypeTask)
	c := createIssue(t, ctx, s, "C", issuestorage.TypeTask)

	addPC(t, ctx, s, a.ID, root.ID)
	addPC(t, ctx, s, b.ID, root.ID)
	addPC(t, ctx, s, c.ID, root.ID)

	// B blocked by A
	addBlocks(t, ctx, s, b.ID, a.ID)

	waves, _, err := TopologicalWavesAcrossParents(ctx, s, root.ID, true)
	if err != nil {
		t.Fatalf("TopologicalWavesAcrossParents: %v", err)
	}

	// Wave 0: [A, C], Wave 1: [B]
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if len(waves[0]) != 2 {
		t.Errorf("wave 0: expected 2 tasks, got %d", len(waves[0]))
	}
	if len(waves[1]) != 1 {
		t.Errorf("wave 1: expected 1 task, got %d", len(waves[1]))
	}
}

// helpers

func addPC(t *testing.T, ctx context.Context, s *issueservice.IssueStore, childID, parentID string) {
	t.Helper()
	if err := s.AddDependency(ctx, childID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency parent-child %s->%s: %v", childID, parentID, err)
	}
}

func addBlocks(t *testing.T, ctx context.Context, s *issueservice.IssueStore, blockedID, blockerID string) {
	t.Helper()
	if err := s.AddDependency(ctx, blockedID, blockerID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency blocks %s->%s: %v", blockedID, blockerID, err)
	}
}

func toSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
