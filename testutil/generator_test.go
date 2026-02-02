package testutil

import (
	"context"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func setupTestStorage(t *testing.T) issuestorage.IssueStore {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	return store
}

func TestNewIssueGenerator(t *testing.T) {
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	if gen == nil {
		t.Fatal("NewIssueGenerator returned nil")
	}
	if gen.storage != store {
		t.Error("generator storage not set correctly")
	}
	if len(gen.IDs()) != 0 {
		t.Error("new generator should have no IDs")
	}
}

func TestIssueGenerator_GenerateDependencyChain(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	// Generate a chain of 3 issues
	ids, err := gen.GenerateDependencyChain(ctx, 3)
	if err != nil {
		t.Fatalf("GenerateDependencyChain failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("expected 3 IDs, got %d", len(ids))
	}

	// Verify generator tracks all IDs
	if len(gen.IDs()) != 3 {
		t.Errorf("generator should track 3 IDs, got %d", len(gen.IDs()))
	}

	// Verify dependency chain: ids[1] depends on ids[0], ids[2] depends on ids[1]
	issue0, _ := store.Get(ctx, ids[0])
	issue1, _ := store.Get(ctx, ids[1])
	issue2, _ := store.Get(ctx, ids[2])

	// First issue has no dependencies
	if len(issue0.Dependencies) != 0 {
		t.Errorf("first issue should have no dependencies, got %d", len(issue0.Dependencies))
	}
	// First issue should have one dependent (issue 1)
	if len(issue0.Dependents) != 1 {
		t.Errorf("first issue should have 1 dependent, got %d", len(issue0.Dependents))
	}

	// Second issue depends on first
	if !issue1.HasDependency(ids[0]) {
		t.Error("second issue should depend on first")
	}
	// Second issue should have one dependent (issue 2)
	if len(issue1.Dependents) != 1 {
		t.Errorf("second issue should have 1 dependent, got %d", len(issue1.Dependents))
	}

	// Third issue depends on second
	if !issue2.HasDependency(ids[1]) {
		t.Error("third issue should depend on second")
	}
	// Third issue should have no dependents
	if len(issue2.Dependents) != 0 {
		t.Errorf("third issue should have no dependents, got %d", len(issue2.Dependents))
	}
}

func TestIssueGenerator_GenerateDependencyChain_Empty(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	ids, err := gen.GenerateDependencyChain(ctx, 0)
	if err != nil {
		t.Fatalf("GenerateDependencyChain(0) failed: %v", err)
	}
	if ids != nil && len(ids) != 0 {
		t.Errorf("expected nil or empty slice for length 0, got %v", ids)
	}
}

func TestIssueGenerator_GenerateTree(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	// Generate tree with depth=2, breadth=2
	// This creates: 2 root nodes, each with 2 children = 2 + 4 = 6 issues
	err := gen.GenerateTree(ctx, 2, 2)
	if err != nil {
		t.Fatalf("GenerateTree failed: %v", err)
	}

	// Depth 2, breadth 2: total = 2^1 + 2^2 = 2 + 4 = 6... no wait
	// Actually: depth=2 means 2 levels, breadth=2 means 2 children per node
	// Level 1: 2 nodes, Level 2: 2*2 = 4 nodes, Total = 6... let me verify
	expectedCount := 2 + 2*2 // depth 2, breadth 2: first level has 2, second level has 4
	if len(gen.IDs()) != expectedCount {
		t.Errorf("expected %d issues, got %d", expectedCount, len(gen.IDs()))
	}
}

func TestIssueGenerator_GenerateTree_SingleNode(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	// Generate tree with depth=1, breadth=1 - should create exactly 1 issue
	err := gen.GenerateTree(ctx, 1, 1)
	if err != nil {
		t.Fatalf("GenerateTree failed: %v", err)
	}

	if len(gen.IDs()) != 1 {
		t.Errorf("expected 1 issue, got %d", len(gen.IDs()))
	}
}

func TestIssueGenerator_GenerateDependencyDAG(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	// Generate a DAG with 5 nodes and up to 4 edges
	err := gen.GenerateDependencyDAG(ctx, 5, 4)
	if err != nil {
		t.Fatalf("GenerateDependencyDAG failed: %v", err)
	}

	// Should have created 5 nodes
	if len(gen.IDs()) != 5 {
		t.Errorf("expected 5 issues, got %d", len(gen.IDs()))
	}

	// Verify all issues exist
	for _, id := range gen.IDs() {
		_, err := store.Get(ctx, id)
		if err != nil {
			t.Errorf("issue %s should exist: %v", id, err)
		}
	}
}

func TestIssueGenerator_GenerateDependencyDAG_Empty(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	err := gen.GenerateDependencyDAG(ctx, 0, 0)
	if err != nil {
		t.Fatalf("GenerateDependencyDAG(0, 0) failed: %v", err)
	}
	if len(gen.IDs()) != 0 {
		t.Errorf("expected 0 issues for nodes=0, got %d", len(gen.IDs()))
	}
}

func TestIssueGenerator_Cleanup(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	// Generate some issues
	ids, err := gen.GenerateDependencyChain(ctx, 3)
	if err != nil {
		t.Fatalf("GenerateDependencyChain failed: %v", err)
	}

	// Verify they exist
	for _, id := range ids {
		_, err := store.Get(ctx, id)
		if err != nil {
			t.Errorf("issue %s should exist before cleanup", id)
		}
	}

	// Cleanup
	if err := gen.Cleanup(ctx); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify generator IDs are cleared
	if len(gen.IDs()) != 0 {
		t.Errorf("generator should have no IDs after cleanup, got %d", len(gen.IDs()))
	}

	// Verify issues are deleted
	for _, id := range ids {
		_, err := store.Get(ctx, id)
		if err == nil {
			t.Errorf("issue %s should not exist after cleanup", id)
		}
	}
}

func TestIssueGenerator_MultipleGenerations(t *testing.T) {
	ctx := context.Background()
	store := setupTestStorage(t)
	gen := NewIssueGenerator(store)

	// Generate a chain
	chain1, _ := gen.GenerateDependencyChain(ctx, 2)

	// Generate another chain
	chain2, _ := gen.GenerateDependencyChain(ctx, 2)

	// Generator should track all 4 IDs
	if len(gen.IDs()) != 4 {
		t.Errorf("expected 4 total IDs, got %d", len(gen.IDs()))
	}

	// All IDs should be unique
	idSet := make(map[string]bool)
	for _, id := range gen.IDs() {
		if idSet[id] {
			t.Errorf("duplicate ID: %s", id)
		}
		idSet[id] = true
	}

	// Chains should be independent
	if chain1[0] == chain2[0] || chain1[1] == chain2[1] {
		t.Error("chains should have different IDs")
	}
}
