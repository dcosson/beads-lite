package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
)

func TestParentSetBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create parent and child issues
	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, err := store.Create(context.Background(), parent)
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}
	childID, err := store.Create(context.Background(), child)
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", childID, parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent set failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Set parent") {
		t.Errorf("expected 'Set parent' in output, got %q", output)
	}

	// Verify child has parent
	gotChild, err := store.Get(context.Background(), childID)
	if err != nil {
		t.Fatalf("failed to get child: %v", err)
	}
	if gotChild.Parent != parentID {
		t.Errorf("expected child parent %q, got %q", parentID, gotChild.Parent)
	}

	// Verify parent has child
	gotParent, err := store.Get(context.Background(), parentID)
	if err != nil {
		t.Fatalf("failed to get parent: %v", err)
	}
	found := false
	for _, c := range gotParent.Children {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected parent to have child %q in children list", childID)
	}
}

func TestParentSetReparent(t *testing.T) {
	app, store := setupTestApp(t)

	// Create two parents and a child
	parent1 := &storage.Issue{Title: "Parent 1", Type: storage.TypeEpic}
	parent2 := &storage.Issue{Title: "Parent 2", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parent1ID, _ := store.Create(context.Background(), parent1)
	parent2ID, _ := store.Create(context.Background(), parent2)
	childID, _ := store.Create(context.Background(), child)

	// Set initial parent
	if err := store.SetParent(context.Background(), childID, parent1ID); err != nil {
		t.Fatalf("failed to set initial parent: %v", err)
	}

	// Re-parent to new parent
	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", childID, parent2ID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent set (reparent) failed: %v", err)
	}

	// Verify child has new parent
	gotChild, _ := store.Get(context.Background(), childID)
	if gotChild.Parent != parent2ID {
		t.Errorf("expected child parent %q, got %q", parent2ID, gotChild.Parent)
	}

	// Verify old parent no longer has child
	gotOldParent, _ := store.Get(context.Background(), parent1ID)
	for _, c := range gotOldParent.Children {
		if c == childID {
			t.Errorf("old parent should not have child in children list")
		}
	}

	// Verify new parent has child
	gotNewParent, _ := store.Get(context.Background(), parent2ID)
	found := false
	for _, c := range gotNewParent.Children {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("new parent should have child in children list")
	}
}

func TestParentSetWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, _ := store.Create(context.Background(), parent)
	childID, _ := store.Create(context.Background(), child)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", childID, parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent set failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["child"] != childID {
		t.Errorf("expected child %q, got %q", childID, result["child"])
	}
	if result["parent"] != parentID {
		t.Errorf("expected parent %q, got %q", parentID, result["parent"])
	}
	if result["status"] != "updated" {
		t.Errorf("expected status 'updated', got %q", result["status"])
	}
}

func TestParentSetChildNotFound(t *testing.T) {
	app, store := setupTestApp(t)

	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	parentID, _ := store.Create(context.Background(), parent)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", "bd-nonexistent", parentID})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent child")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestParentSetParentNotFound(t *testing.T) {
	app, store := setupTestApp(t)

	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}
	childID, _ := store.Create(context.Background(), child)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", childID, "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent parent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestParentSetCycle(t *testing.T) {
	app, store := setupTestApp(t)

	// Create two issues and set up a parent relationship
	issueA := &storage.Issue{Title: "A", Type: storage.TypeTask}
	issueB := &storage.Issue{Title: "B", Type: storage.TypeTask}

	idA, _ := store.Create(context.Background(), issueA)
	idB, _ := store.Create(context.Background(), issueB)

	// Make A parent of B
	if err := store.SetParent(context.Background(), idB, idA); err != nil {
		t.Fatalf("failed to set initial parent: %v", err)
	}

	// Try to make B parent of A (would create cycle)
	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", idA, idB})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

func TestParentSetNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}
}

func TestParentSetOneArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"set", "bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when only one argument provided")
	}
}

func TestParentRemoveBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create parent and child, set relationship
	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, _ := store.Create(context.Background(), parent)
	childID, _ := store.Create(context.Background(), child)

	if err := store.SetParent(context.Background(), childID, parentID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"remove", childID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent remove failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Removed parent") {
		t.Errorf("expected 'Removed parent' in output, got %q", output)
	}
	if !strings.Contains(output, parentID) {
		t.Errorf("expected old parent ID in output, got %q", output)
	}

	// Verify child has no parent
	gotChild, _ := store.Get(context.Background(), childID)
	if gotChild.Parent != "" {
		t.Errorf("expected child to have no parent, got %q", gotChild.Parent)
	}

	// Verify parent no longer has child
	gotParent, _ := store.Get(context.Background(), parentID)
	for _, c := range gotParent.Children {
		if c == childID {
			t.Errorf("parent should not have child in children list")
		}
	}
}

func TestParentRemoveWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, _ := store.Create(context.Background(), parent)
	childID, _ := store.Create(context.Background(), child)

	if err := store.SetParent(context.Background(), childID, parentID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"remove", childID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("parent remove failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["child"] != childID {
		t.Errorf("expected child %q, got %q", childID, result["child"])
	}
	if result["old_parent"] != parentID {
		t.Errorf("expected old_parent %q, got %q", parentID, result["old_parent"])
	}
	if result["status"] != "removed" {
		t.Errorf("expected status 'removed', got %q", result["status"])
	}
}

func TestParentRemoveIssueNotFound(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"remove", "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestParentRemoveNoParent(t *testing.T) {
	app, store := setupTestApp(t)

	// Create an issue without a parent
	issue := &storage.Issue{Title: "Orphan", Type: storage.TypeTask}
	issueID, _ := store.Create(context.Background(), issue)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"remove", issueID})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when issue has no parent")
	}
	if !strings.Contains(err.Error(), "no parent") {
		t.Errorf("expected 'no parent' in error, got: %v", err)
	}
}

func TestParentRemoveNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newParentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"remove"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}
}
