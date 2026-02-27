package graph

import (
	"context"
	"testing"

	"beads-lite/internal/issuestorage"
)

// TestEffectiveBlockers_DirectOnly tests that direct blockers are returned
// when cascade is false (no parent walking).
func TestEffectiveBlockers_DirectOnly(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Create a parent with two children, B blocked by A
	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B"},
		map[string][]string{
			"B": {"A"},
		})

	byTitle := make(map[string]*issuestorage.Issue)
	for _, c := range children {
		byTitle[c.Title] = c
	}

	// B should have A as direct blocker
	result, err := EffectiveBlockers(ctx, s, byTitle["B"], map[string]bool{}, false)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}

	if len(result.Direct) != 1 {
		t.Fatalf("expected 1 direct blocker, got %d", len(result.Direct))
	}
	if result.Direct[0] != byTitle["A"].ID {
		t.Errorf("direct blocker = %s, want %s", result.Direct[0], byTitle["A"].ID)
	}
	if len(result.Inherited) != 0 {
		t.Errorf("expected 0 inherited blockers with cascade=false, got %d", len(result.Inherited))
	}
	if !result.HasBlockers() {
		t.Error("HasBlockers() should be true")
	}

	// A should have no blockers
	result, err = EffectiveBlockers(ctx, s, byTitle["A"], map[string]bool{}, false)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}
	if len(result.Direct) != 0 {
		t.Errorf("A should have 0 direct blockers, got %d", len(result.Direct))
	}
	if result.HasBlockers() {
		t.Error("A should not have any blockers")
	}
}

// TestEffectiveBlockers_ClosedBlockerFiltered tests that closed blockers
// are excluded from the result.
func TestEffectiveBlockers_ClosedBlockerFiltered(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	_, children := buildMolecule(t, ctx, s, "Root", []string{"A", "B"},
		map[string][]string{
			"B": {"A"},
		})

	byTitle := make(map[string]*issuestorage.Issue)
	for _, c := range children {
		byTitle[c.Title] = c
	}

	// With A in closedSet, B should have no blockers
	closedSet := map[string]bool{byTitle["A"].ID: true}
	result, err := EffectiveBlockers(ctx, s, byTitle["B"], closedSet, false)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}
	if len(result.Direct) != 0 {
		t.Errorf("expected 0 direct blockers after closing A, got %d", len(result.Direct))
	}
	if result.HasBlockers() {
		t.Error("B should not be blocked after A is closed")
	}
}

// TestEffectiveBlockers_InheritedFromParent tests that blocking deps on a
// parent issue cascade down to its children when cascade=true.
func TestEffectiveBlockers_InheritedFromParent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Create two parents: ParentA and ParentB
	// ParentB blocks ParentA
	// Each parent has a child task
	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	// ParentA depends on (is blocked by) ParentB
	if err := s.AddDependency(ctx, parentA.ID, parentB.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency blocks: %v", err)
	}

	// Create children
	childA := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, childA.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency parent-child: %v", err)
	}
	childB := createIssue(t, ctx, s, "Task B1", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, childB.ID, parentB.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency parent-child: %v", err)
	}

	// Re-read to get updated deps
	childA, _ = s.Get(ctx, childA.ID)
	childB, _ = s.Get(ctx, childB.ID)

	// With cascade=false, childA should have no blockers (it has no direct blocking deps)
	result, err := EffectiveBlockers(ctx, s, childA, map[string]bool{}, false)
	if err != nil {
		t.Fatalf("EffectiveBlockers cascade=false: %v", err)
	}
	if result.HasBlockers() {
		t.Error("childA should have no blockers when cascade=false")
	}

	// With cascade=true, childA should inherit the blocker from parentA
	result, err = EffectiveBlockers(ctx, s, childA, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("EffectiveBlockers cascade=true: %v", err)
	}
	if len(result.Direct) != 0 {
		t.Errorf("expected 0 direct blockers, got %d", len(result.Direct))
	}
	if len(result.Inherited) != 1 {
		t.Fatalf("expected 1 inherited blocker, got %d", len(result.Inherited))
	}
	if result.Inherited[0].AncestorID != parentA.ID {
		t.Errorf("inherited ancestor = %s, want %s", result.Inherited[0].AncestorID, parentA.ID)
	}
	if result.Inherited[0].BlockerID != parentB.ID {
		t.Errorf("inherited blocker = %s, want %s", result.Inherited[0].BlockerID, parentB.ID)
	}
	if !result.HasBlockers() {
		t.Error("childA should be blocked with cascade=true")
	}

	// childB should have no inherited blockers (parentB has no blocking deps)
	result, err = EffectiveBlockers(ctx, s, childB, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("EffectiveBlockers for childB: %v", err)
	}
	if result.HasBlockers() {
		t.Error("childB should not have any blockers")
	}
}

// TestEffectiveBlockers_InheritedClosedFiltered tests that inherited blockers
// are filtered when the blocker is in the closedSet.
func TestEffectiveBlockers_InheritedClosedFiltered(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	if err := s.AddDependency(ctx, parentA.ID, parentB.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	childA := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, childA.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	childA, _ = s.Get(ctx, childA.ID)

	// ParentB is closed — childA should not inherit the blocker
	closedSet := map[string]bool{parentB.ID: true}
	result, err := EffectiveBlockers(ctx, s, childA, closedSet, true)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}
	if result.HasBlockers() {
		t.Error("childA should not be blocked when parentB is closed")
	}
}

// TestEffectiveBlockers_GrandparentInheritance tests that blockers cascade
// through multiple levels of parent hierarchy.
func TestEffectiveBlockers_GrandparentInheritance(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Grandparent -> Parent -> Child
	// External blocker blocks Grandparent
	grandparent := createIssue(t, ctx, s, "Grandparent", issuestorage.TypeEpic)
	blocker := createIssue(t, ctx, s, "External Blocker", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, grandparent.ID, blocker.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	parent := createIssue(t, ctx, s, "Parent", issuestorage.TypeEpic)
	if err := s.AddDependency(ctx, parent.ID, grandparent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}

	child := createIssue(t, ctx, s, "Child Task", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, child.ID, parent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}

	child, _ = s.Get(ctx, child.ID)

	// Child should inherit blocker from grandparent via parent chain
	result, err := EffectiveBlockers(ctx, s, child, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}
	if len(result.Inherited) != 1 {
		t.Fatalf("expected 1 inherited blocker, got %d", len(result.Inherited))
	}
	if result.Inherited[0].AncestorID != grandparent.ID {
		t.Errorf("ancestor = %s, want %s (grandparent)", result.Inherited[0].AncestorID, grandparent.ID)
	}
	if result.Inherited[0].BlockerID != blocker.ID {
		t.Errorf("blocker = %s, want %s", result.Inherited[0].BlockerID, blocker.ID)
	}
}

// TestEffectiveBlockers_DirectAndInherited tests that both direct and inherited
// blockers are returned together.
func TestEffectiveBlockers_DirectAndInherited(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	if err := s.AddDependency(ctx, parentA.ID, parentB.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	childA1 := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	childA2 := createIssue(t, ctx, s, "Task A2", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, childA1.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDependency(ctx, childA2.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	// A2 is directly blocked by A1
	if err := s.AddDependency(ctx, childA2.ID, childA1.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	childA2, _ = s.Get(ctx, childA2.ID)

	result, err := EffectiveBlockers(ctx, s, childA2, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}

	// Should have A1 as direct blocker and parentB as inherited blocker
	if len(result.Direct) != 1 {
		t.Fatalf("expected 1 direct blocker, got %d", len(result.Direct))
	}
	if result.Direct[0] != childA1.ID {
		t.Errorf("direct blocker = %s, want %s", result.Direct[0], childA1.ID)
	}
	if len(result.Inherited) != 1 {
		t.Fatalf("expected 1 inherited blocker, got %d", len(result.Inherited))
	}
	if result.Inherited[0].BlockerID != parentB.ID {
		t.Errorf("inherited blocker = %s, want %s", result.Inherited[0].BlockerID, parentB.ID)
	}

	// AllBlockerIDs should contain both
	allIDs := result.AllBlockerIDs()
	if len(allIDs) != 2 {
		t.Fatalf("AllBlockerIDs() = %v, want 2 entries", allIDs)
	}
}

// TestEffectiveBlockers_NoParent tests that an issue with no parent
// returns only direct blockers even when cascade=true.
func TestEffectiveBlockers_NoParent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	a := createIssue(t, ctx, s, "A", issuestorage.TypeTask)
	b := createIssue(t, ctx, s, "B", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, b.ID, a.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}
	b, _ = s.Get(ctx, b.ID)

	result, err := EffectiveBlockers(ctx, s, b, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("EffectiveBlockers: %v", err)
	}
	if len(result.Direct) != 1 {
		t.Fatalf("expected 1 direct blocker, got %d", len(result.Direct))
	}
	if len(result.Inherited) != 0 {
		t.Errorf("expected 0 inherited (no parent), got %d", len(result.Inherited))
	}
}

// TestIsEffectivelyBlocked tests the convenience wrapper.
func TestIsEffectivelyBlocked(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	if err := s.AddDependency(ctx, parentA.ID, parentB.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	child := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, child.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	child, _ = s.Get(ctx, child.ID)

	// cascade=false: not blocked
	blocked, err := IsEffectivelyBlocked(ctx, s, child, map[string]bool{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Error("should not be blocked with cascade=false")
	}

	// cascade=true: blocked via parent
	blocked, err = IsEffectivelyBlocked(ctx, s, child, map[string]bool{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Error("should be blocked with cascade=true")
	}

	// cascade=true but parentB closed: not blocked
	blocked, err = IsEffectivelyBlocked(ctx, s, child, map[string]bool{parentB.ID: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Error("should not be blocked when parentB is closed")
	}
}

// TestEffectiveBlockersBatch tests the batch version with ancestor caching.
func TestEffectiveBlockersBatch(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	if err := s.AddDependency(ctx, parentA.ID, parentB.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	childA1 := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	childA2 := createIssue(t, ctx, s, "Task A2", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, childA1.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDependency(ctx, childA2.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}

	childA1, _ = s.Get(ctx, childA1.ID)
	childA2, _ = s.Get(ctx, childA2.ID)

	issues := []*issuestorage.Issue{childA1, childA2}

	// Batch with cascade=true
	results, err := EffectiveBlockersBatch(ctx, s, issues, map[string]bool{}, true)
	if err != nil {
		t.Fatalf("EffectiveBlockersBatch: %v", err)
	}

	for _, child := range issues {
		r, ok := results[child.ID]
		if !ok {
			t.Errorf("missing result for %s", child.ID)
			continue
		}
		if len(r.Inherited) != 1 {
			t.Errorf("child %s: expected 1 inherited blocker, got %d", child.ID, len(r.Inherited))
		}
	}

	// Batch with cascade=false
	results, err = EffectiveBlockersBatch(ctx, s, issues, map[string]bool{}, false)
	if err != nil {
		t.Fatalf("EffectiveBlockersBatch cascade=false: %v", err)
	}

	for _, child := range issues {
		r := results[child.ID]
		if r.HasBlockers() {
			t.Errorf("child %s: should have no blockers with cascade=false", child.ID)
		}
	}
}

// TestEffectiveBlockers_TypeAgnostic tests that cascade works with non-epic
// parent types (any issue type can be a parent).
func TestEffectiveBlockers_TypeAgnostic(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Create a feature (not epic) as parent
	parentFeature := createIssue(t, ctx, s, "Feature Parent", issuestorage.TypeFeature)
	blocker := createIssue(t, ctx, s, "Blocker Bug", issuestorage.TypeBug)

	if err := s.AddDependency(ctx, parentFeature.ID, blocker.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	child := createIssue(t, ctx, s, "Child Task", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, child.ID, parentFeature.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	child, _ = s.Get(ctx, child.ID)

	result, err := EffectiveBlockers(ctx, s, child, map[string]bool{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Inherited) != 1 {
		t.Fatalf("expected 1 inherited blocker from feature parent, got %d", len(result.Inherited))
	}
	if result.Inherited[0].BlockerID != blocker.ID {
		t.Errorf("blocker = %s, want %s", result.Inherited[0].BlockerID, blocker.ID)
	}
}

// TestEffectiveBlockers_MultipleBlockersOnAncestor tests that multiple blocking
// deps on a single ancestor are all inherited.
func TestEffectiveBlockers_MultipleBlockersOnAncestor(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	parent := createIssue(t, ctx, s, "Parent", issuestorage.TypeEpic)
	blockerX := createIssue(t, ctx, s, "Blocker X", issuestorage.TypeTask)
	blockerY := createIssue(t, ctx, s, "Blocker Y", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, parent.ID, blockerX.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}
	if err := s.AddDependency(ctx, parent.ID, blockerY.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	child := createIssue(t, ctx, s, "Child", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, child.ID, parent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	child, _ = s.Get(ctx, child.ID)

	result, err := EffectiveBlockers(ctx, s, child, map[string]bool{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Inherited) != 2 {
		t.Fatalf("expected 2 inherited blockers, got %d", len(result.Inherited))
	}

	blockerIDs := make(map[string]bool)
	for _, ib := range result.Inherited {
		blockerIDs[ib.BlockerID] = true
	}
	if !blockerIDs[blockerX.ID] || !blockerIDs[blockerY.ID] {
		t.Errorf("expected both blockers X and Y, got %v", blockerIDs)
	}
}

// TestFindReadyStepsWithCascade tests the cascade-aware ready steps finder.
func TestFindReadyStepsWithCascade(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// ParentA blocked by ParentB
	parentA := createIssue(t, ctx, s, "Parent A", issuestorage.TypeEpic)
	parentB := createIssue(t, ctx, s, "Parent B", issuestorage.TypeEpic)

	if err := s.AddDependency(ctx, parentA.ID, parentB.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	// childA1 under parentA, childB1 under parentB
	childA1 := createIssue(t, ctx, s, "Task A1", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, childA1.ID, parentA.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}
	childB1 := createIssue(t, ctx, s, "Task B1", issuestorage.TypeTask)
	if err := s.AddDependency(ctx, childB1.ID, parentB.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}

	childA1, _ = s.Get(ctx, childA1.ID)
	childB1, _ = s.Get(ctx, childB1.ID)
	children := []*issuestorage.Issue{childA1, childB1}

	// With cascade=false, both should be ready (no direct blocking deps between them)
	ready, err := FindReadyStepsWithCascade(ctx, s, children, map[string]bool{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 2 {
		t.Errorf("cascade=false: expected 2 ready, got %d (%v)", len(ready), titlesOf(ready))
	}

	// With cascade=true, only childB1 should be ready (childA1 inherits parentA's block)
	ready, err = FindReadyStepsWithCascade(ctx, s, children, map[string]bool{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 {
		t.Fatalf("cascade=true: expected 1 ready, got %d (%v)", len(ready), titlesOf(ready))
	}
	if ready[0].ID != childB1.ID {
		t.Errorf("ready task = %s (%s), want %s (Task B1)", ready[0].ID, ready[0].Title, childB1.ID)
	}

	// With cascade=true and parentB closed, both should be ready
	ready, err = FindReadyStepsWithCascade(ctx, s, children, map[string]bool{parentB.ID: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 2 {
		t.Errorf("cascade=true, parentB closed: expected 2 ready, got %d (%v)", len(ready), titlesOf(ready))
	}
}
