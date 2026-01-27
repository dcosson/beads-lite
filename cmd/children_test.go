package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"beads2/storage"
)

func TestChildrenCommand(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create parent with two children
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "Parent", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	child1ID, _ := store.Create(ctx, &storage.Issue{Title: "Child 1", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	child2ID, _ := store.Create(ctx, &storage.Issue{Title: "Child 2", Status: storage.StatusClosed, Priority: storage.PriorityMedium, Type: storage.TypeTask})

	store.SetParent(ctx, child1ID, parentID)
	store.SetParent(ctx, child2ID, parentID)

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	// Reset the tree flag
	childrenTree = false
	err := childrenCmd.RunE(childrenCmd, []string{parentID})
	if err != nil {
		t.Fatalf("children command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, child1ID) {
		t.Errorf("Expected output to contain child1 ID %s, got: %s", child1ID, output)
	}
	if !strings.Contains(output, child2ID) {
		t.Errorf("Expected output to contain child2 ID %s, got: %s", child2ID, output)
	}
	if !strings.Contains(output, "Child 1") {
		t.Errorf("Expected output to contain 'Child 1', got: %s", output)
	}
}

func TestChildrenCommandNoChildren(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create issue with no children
	issueID, _ := store.Create(ctx, &storage.Issue{Title: "Lonely", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	childrenTree = false
	err := childrenCmd.RunE(childrenCmd, []string{issueID})
	if err != nil {
		t.Fatalf("children command failed: %v", err)
	}

	if !strings.Contains(out.String(), "No children") {
		t.Errorf("Expected 'No children' message, got: %s", out.String())
	}
}

func TestChildrenCommandJSON(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	parentID, _ := store.Create(ctx, &storage.Issue{Title: "Parent", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "Child", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	store.SetParent(ctx, childID, parentID)

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    true,
	}

	childrenTree = false
	err := childrenCmd.RunE(childrenCmd, []string{parentID})
	if err != nil {
		t.Fatalf("children command failed: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 child, got %d", len(result))
	}
	if result[0]["id"] != childID {
		t.Errorf("Expected child ID %s, got %v", childID, result[0]["id"])
	}
}

func TestChildrenCommandTree(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create a tree: grandparent -> parent -> child
	grandparentID, _ := store.Create(ctx, &storage.Issue{Title: "Grandparent", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "Parent", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "Child", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})

	store.SetParent(ctx, parentID, grandparentID)
	store.SetParent(ctx, childID, parentID)

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	childrenTree = true
	err := childrenCmd.RunE(childrenCmd, []string{grandparentID})
	if err != nil {
		t.Fatalf("children --tree command failed: %v", err)
	}

	output := out.String()
	// Should contain all three levels
	if !strings.Contains(output, "Grandparent") {
		t.Errorf("Expected output to contain 'Grandparent', got: %s", output)
	}
	if !strings.Contains(output, "Parent") {
		t.Errorf("Expected output to contain 'Parent', got: %s", output)
	}
	if !strings.Contains(output, "Child") {
		t.Errorf("Expected output to contain 'Child', got: %s", output)
	}
	// Should have tree structure with indentation
	if !strings.Contains(output, "└─") {
		t.Errorf("Expected tree structure with └─, got: %s", output)
	}
}

func TestChildrenCommandTreeJSON(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create a tree: grandparent -> parent -> child
	grandparentID, _ := store.Create(ctx, &storage.Issue{Title: "Grandparent", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "Parent", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "Child", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})

	store.SetParent(ctx, parentID, grandparentID)
	store.SetParent(ctx, childID, parentID)

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    true,
	}

	childrenTree = true
	err := childrenCmd.RunE(childrenCmd, []string{grandparentID})
	if err != nil {
		t.Fatalf("children --tree --json command failed: %v", err)
	}

	var result TreeNode
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify nested structure
	if result.ID != grandparentID {
		t.Errorf("Expected root ID %s, got %s", grandparentID, result.ID)
	}
	if len(result.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(result.Children))
	}
	if result.Children[0].ID != parentID {
		t.Errorf("Expected child ID %s, got %s", parentID, result.Children[0].ID)
	}
	if len(result.Children[0].Children) != 1 {
		t.Fatalf("Expected 1 grandchild, got %d", len(result.Children[0].Children))
	}
	if result.Children[0].Children[0].ID != childID {
		t.Errorf("Expected grandchild ID %s, got %s", childID, result.Children[0].Children[0].ID)
	}
}

func TestChildrenCommandNotFound(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	childrenTree = false
	err := childrenCmd.RunE(childrenCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("Expected not found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestStatusSymbol(t *testing.T) {
	tests := []struct {
		status storage.Status
		want   string
	}{
		{storage.StatusOpen, "○"},
		{storage.StatusInProgress, "◐"},
		{storage.StatusBlocked, "●"},
		{storage.StatusDeferred, "◇"},
		{storage.StatusClosed, "✓"},
		{storage.Status("unknown"), "?"},
	}

	for _, tc := range tests {
		got := statusSymbol(tc.status)
		if got != tc.want {
			t.Errorf("statusSymbol(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}
