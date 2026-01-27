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

func TestParentSetCommand(t *testing.T) {
	testApp, store := setupTestApp(t)

	ctx := context.Background()

	// Create parent and child issues
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("Failed to create parent: %v", err)
	}

	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("Failed to create child: %v", err)
	}

	// Set up global app for command
	app = testApp

	// Call RunE directly with args
	err = parentSetCmd.RunE(parentSetCmd, []string{childID, parentID})
	if err != nil {
		t.Fatalf("parent set failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, "Set parent") {
		t.Errorf("Expected success message, got: %s", output)
	}

	// Verify the relationship was created
	gotChild, err := store.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Failed to get child: %v", err)
	}
	if gotChild.Parent != parentID {
		t.Errorf("Child.Parent = %q, want %q", gotChild.Parent, parentID)
	}

	gotParent, err := store.Get(ctx, parentID)
	if err != nil {
		t.Fatalf("Failed to get parent: %v", err)
	}
	if len(gotParent.Children) != 1 || gotParent.Children[0] != childID {
		t.Errorf("Parent.Children = %v, want [%s]", gotParent.Children, childID)
	}
}

func TestParentSetCommandJSON(t *testing.T) {
	testApp, store := setupTestApp(t)
	testApp.JSON = true

	ctx := context.Background()

	parentID, _ := store.Create(ctx, &storage.Issue{Title: "P", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "C", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})

	app = testApp

	err := parentSetCmd.RunE(parentSetCmd, []string{childID, parentID})
	if err != nil {
		t.Fatalf("parent set failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(testApp.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", result)
	}
	if result["child_id"] != childID {
		t.Errorf("Expected child_id=%s, got %s", childID, result["child_id"])
	}
}

func TestParentSetCycle(t *testing.T) {
	testApp, store := setupTestApp(t)

	ctx := context.Background()

	// Create A -> B hierarchy
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "A", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "B", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	store.SetParent(ctx, childID, parentID)

	app = testApp

	// Try to make A a child of B (would create cycle)
	err := parentSetCmd.RunE(parentSetCmd, []string{parentID, childID})
	if err == nil {
		t.Fatal("Expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Expected cycle error, got: %v", err)
	}
}

func TestParentRemoveCommand(t *testing.T) {
	testApp, store := setupTestApp(t)

	ctx := context.Background()

	// Create parent-child relationship
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "P", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "C", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	store.SetParent(ctx, childID, parentID)

	app = testApp

	err := parentRemoveCmd.RunE(parentRemoveCmd, []string{childID})
	if err != nil {
		t.Fatalf("parent remove failed: %v", err)
	}

	output := testApp.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, "Removed parent") {
		t.Errorf("Expected success message, got: %s", output)
	}

	// Verify the relationship was removed
	gotChild, _ := store.Get(ctx, childID)
	if gotChild.Parent != "" {
		t.Errorf("Child still has parent: %s", gotChild.Parent)
	}

	gotParent, _ := store.Get(ctx, parentID)
	if len(gotParent.Children) != 0 {
		t.Errorf("Parent still has children: %v", gotParent.Children)
	}
}

func TestParentRemoveNotFound(t *testing.T) {
	testApp, _ := setupTestApp(t)
	app = testApp

	err := parentRemoveCmd.RunE(parentRemoveCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("Expected not found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestParentRemoveJSON(t *testing.T) {
	testApp, store := setupTestApp(t)
	testApp.JSON = true

	ctx := context.Background()

	parentID, _ := store.Create(ctx, &storage.Issue{Title: "P", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "C", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	store.SetParent(ctx, childID, parentID)

	app = testApp

	err := parentRemoveCmd.RunE(parentRemoveCmd, []string{childID})
	if err != nil {
		t.Fatalf("parent remove failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(testApp.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", result)
	}
}

// Ensure global app isn't nil for commands that use GetApp()
var _ = os.Stderr // silence unused import warning
