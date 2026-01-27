package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

func setupTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	beadsDir := dir + "/.beads"

	store := filesystem.New(beadsDir)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	return &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
		JSON:    false,
	}
}

func createTestIssue(t *testing.T, store storage.Storage, title string) string {
	t.Helper()
	id, err := store.Create(context.Background(), &storage.Issue{
		Title:    title,
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	return id
}

func TestDepAdd(t *testing.T) {
	testApp := setupTestApp(t)
	ctx := context.Background()

	// Create two issues
	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")

	// Call the storage method directly (this is what the command does)
	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Verify the dependency was created
	issueA, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A failed: %v", err)
	}
	if !contains(issueA.DependsOn, idB) {
		t.Errorf("A.DependsOn should contain B; got %v", issueA.DependsOn)
	}

	issueB, err := testApp.Storage.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B failed: %v", err)
	}
	if !contains(issueB.Dependents, idA) {
		t.Errorf("B.Dependents should contain A; got %v", issueB.Dependents)
	}
}

func TestDepAddCycle(t *testing.T) {
	testApp := setupTestApp(t)
	ctx := context.Background()

	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")

	// Create A depends on B
	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Try to create B depends on A (would create cycle)
	err := testApp.Storage.AddDependency(ctx, idB, idA)
	if err != storage.ErrCycle {
		t.Errorf("Expected ErrCycle, got: %v", err)
	}
}

func TestDepRemove(t *testing.T) {
	testApp := setupTestApp(t)
	ctx := context.Background()

	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")

	// Create dependency first
	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Remove it
	if err := testApp.Storage.RemoveDependency(ctx, idA, idB); err != nil {
		t.Fatalf("RemoveDependency failed: %v", err)
	}

	// Verify the dependency was removed
	issueA, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A failed: %v", err)
	}
	if contains(issueA.DependsOn, idB) {
		t.Errorf("A.DependsOn should not contain B; got %v", issueA.DependsOn)
	}

	issueB, err := testApp.Storage.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B failed: %v", err)
	}
	if contains(issueB.Dependents, idA) {
		t.Errorf("B.Dependents should not contain A; got %v", issueB.Dependents)
	}
}

func TestDepListOutput(t *testing.T) {
	testApp := setupTestApp(t)
	ctx := context.Background()

	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")
	idC := createTestIssue(t, testApp.Storage, "Issue C")

	// A depends on B, A blocks C
	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}
	if err := testApp.Storage.AddBlock(ctx, idA, idC); err != nil {
		t.Fatalf("AddBlock failed: %v", err)
	}

	issue, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Test the output function directly
	if err := printDependencyList(ctx, testApp, issue); err != nil {
		t.Fatalf("printDependencyList failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, "Issue A") {
		t.Errorf("Output should contain Issue A title, got:\n%s", output)
	}
	if !strings.Contains(output, "Depends on") {
		t.Errorf("Output should contain 'Depends on' section, got:\n%s", output)
	}
	if !strings.Contains(output, "Issue B") {
		t.Errorf("Output should show Issue B as dependency, got:\n%s", output)
	}
	if !strings.Contains(output, "Blocks") {
		t.Errorf("Output should contain 'Blocks' section, got:\n%s", output)
	}
	if !strings.Contains(output, "Issue C") {
		t.Errorf("Output should show Issue C as blocked, got:\n%s", output)
	}
}

func TestDepListJSON(t *testing.T) {
	testApp := setupTestApp(t)
	testApp.JSON = true
	ctx := context.Background()

	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")

	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	issue, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if err := printDependencyList(ctx, testApp, issue); err != nil {
		t.Fatalf("printDependencyList failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	var info DepInfo
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}
	if info.ID != idA {
		t.Errorf("Expected ID=%s, got %s", idA, info.ID)
	}
	if len(info.DependsOn) != 1 || info.DependsOn[0] != idB {
		t.Errorf("Expected DependsOn=[%s], got %v", idB, info.DependsOn)
	}
}

func TestDepListTree(t *testing.T) {
	testApp := setupTestApp(t)
	ctx := context.Background()

	// Create a chain: A -> B -> C
	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")
	idC := createTestIssue(t, testApp.Storage, "Issue C")

	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency A->B failed: %v", err)
	}
	if err := testApp.Storage.AddDependency(ctx, idB, idC); err != nil {
		t.Fatalf("AddDependency B->C failed: %v", err)
	}

	issue, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if err := printDependencyTree(ctx, testApp, issue); err != nil {
		t.Fatalf("printDependencyTree failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	// Should show tree structure
	if !strings.Contains(output, "Issue A") {
		t.Errorf("Output should contain Issue A title, got:\n%s", output)
	}
	if !strings.Contains(output, "Issue B") {
		t.Errorf("Output should show Issue B in tree, got:\n%s", output)
	}
	if !strings.Contains(output, "Issue C") {
		t.Errorf("Output should show Issue C in tree, got:\n%s", output)
	}
}

func TestDepListTreeJSON(t *testing.T) {
	testApp := setupTestApp(t)
	testApp.JSON = true
	ctx := context.Background()

	// Create a chain: A -> B
	idA := createTestIssue(t, testApp.Storage, "Issue A")
	idB := createTestIssue(t, testApp.Storage, "Issue B")

	if err := testApp.Storage.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency A->B failed: %v", err)
	}

	issue, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if err := printDependencyTree(ctx, testApp, issue); err != nil {
		t.Fatalf("printDependencyTree failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	var tree DepTreeNode
	if err := json.Unmarshal([]byte(output), &tree); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}
	if tree.ID != idA {
		t.Errorf("Expected root ID=%s, got %s", idA, tree.ID)
	}
	if len(tree.Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(tree.Children))
	} else if tree.Children[0].ID != idB {
		t.Errorf("Expected child ID=%s, got %s", idB, tree.Children[0].ID)
	}
}

func TestDepListNoDeps(t *testing.T) {
	testApp := setupTestApp(t)
	ctx := context.Background()

	idA := createTestIssue(t, testApp.Storage, "Issue A")

	issue, err := testApp.Storage.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if err := printDependencyList(ctx, testApp, issue); err != nil {
		t.Fatalf("printDependencyList failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, "(none)") {
		t.Errorf("Output should show (none) for empty deps, got:\n%s", output)
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   storage.Status
		expected string
	}{
		{storage.StatusClosed, "[x]"},
		{storage.StatusInProgress, "[~]"},
		{storage.StatusBlocked, "[!]"},
		{storage.StatusDeferred, "[-]"},
		{storage.StatusOpen, "[ ]"},
	}

	for _, tc := range tests {
		got := statusIcon(tc.status)
		if got != tc.expected {
			t.Errorf("statusIcon(%s) = %s, want %s", tc.status, got, tc.expected)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
