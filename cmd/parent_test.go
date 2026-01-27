package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

func setupTestApp(t *testing.T) (storage.Storage, func()) {
	tmpDir, err := os.MkdirTemp("", "beads-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	beadsDir := filepath.Join(tmpDir, ".beads")
	store := filesystem.New(beadsDir)
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestParentSetCommand(t *testing.T) {
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create parent and child issues
	parent := &storage.Issue{
		Title:    "Parent Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeEpic,
	}
	parentID, err := store.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Failed to create parent: %v", err)
	}

	child := &storage.Issue{
		Title:    "Child Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	childID, err := store.Create(ctx, child)
	if err != nil {
		t.Fatalf("Failed to create child: %v", err)
	}

	// Test parent set command
	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	// Call RunE directly with args
	err = parentSetCmd.RunE(parentSetCmd, []string{childID, parentID})
	if err != nil {
		t.Fatalf("parent set failed: %v", err)
	}

	if !strings.Contains(out.String(), "Set parent") {
		t.Errorf("Expected success message, got: %s", out.String())
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
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	parentID, _ := store.Create(ctx, &storage.Issue{Title: "P", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "C", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    true,
	}

	err := parentSetCmd.RunE(parentSetCmd, []string{childID, parentID})
	if err != nil {
		t.Fatalf("parent set failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
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
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create A -> B hierarchy
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "A", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "B", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	store.SetParent(ctx, childID, parentID)

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

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
	store, cleanup := setupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create parent-child relationship
	parentID, _ := store.Create(ctx, &storage.Issue{Title: "P", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeEpic})
	childID, _ := store.Create(ctx, &storage.Issue{Title: "C", Status: storage.StatusOpen, Priority: storage.PriorityMedium, Type: storage.TypeTask})
	store.SetParent(ctx, childID, parentID)

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	err := parentRemoveCmd.RunE(parentRemoveCmd, []string{childID})
	if err != nil {
		t.Fatalf("parent remove failed: %v", err)
	}

	if !strings.Contains(out.String(), "Removed parent") {
		t.Errorf("Expected success message, got: %s", out.String())
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
	store, cleanup := setupTestApp(t)
	defer cleanup()

	var out bytes.Buffer
	app = &App{
		Storage: store,
		Out:     &out,
		Err:     os.Stderr,
		JSON:    false,
	}

	err := parentRemoveCmd.RunE(parentRemoveCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("Expected not found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected not found error, got: %v", err)
	}
}
