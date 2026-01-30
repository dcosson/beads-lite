package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
)

func createTestIssue(t *testing.T, store storage.Storage) string {
	t.Helper()
	issue := &storage.Issue{
		Title:       "Original title",
		Description: "Original description",
		Type:        storage.TypeTask,
		Priority:    storage.PriorityMedium,
		Status:      storage.StatusOpen,
		Assignee:    "original-assignee",
		Labels:      []string{"backend", "api"},
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}
	return id
}

func TestUpdateTitle(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--title", "New title"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !strings.Contains(out.String(), "Updated") {
		t.Errorf("expected 'Updated' in output, got %q", out.String())
	}

	issue, _ := store.Get(context.Background(), issueID)
	if issue.Title != "New title" {
		t.Errorf("expected title %q, got %q", "New title", issue.Title)
	}
	// Verify other fields unchanged
	if issue.Description != "Original description" {
		t.Errorf("description should not change, got %q", issue.Description)
	}
}

func TestUpdateOutputFormat(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--title", "New title"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	output := out.String()
	// Check for the checkmark (with ANSI color codes) and the message
	if !strings.Contains(output, "✓") {
		t.Errorf("expected output to contain checkmark, got %q", output)
	}
	if !strings.Contains(output, "Updated issue: "+issueID) {
		t.Errorf("expected output to contain 'Updated issue: %s', got %q", issueID, output)
	}
}

func TestUpdateDescription(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--description", "New description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if issue.Description != "New description" {
		t.Errorf("expected description %q, got %q", "New description", issue.Description)
	}
}

func TestUpdatePriority(t *testing.T) {
	tests := []struct {
		priority string
		expected storage.Priority
	}{
		{"0", storage.PriorityCritical},
		{"p0", storage.PriorityCritical},
		{"P0", storage.PriorityCritical}, // test case insensitivity
		{"1", storage.PriorityHigh},
		{"p1", storage.PriorityHigh},
		{"2", storage.PriorityMedium},
		{"p2", storage.PriorityMedium},
		{"3", storage.PriorityLow},
		{"p3", storage.PriorityLow},
		{"4", storage.PriorityBacklog},
		{"p4", storage.PriorityBacklog},
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			app, store := setupTestApp(t)
			issueID := createTestIssue(t, store)

			cmd := newUpdateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{issueID, "--priority", tt.priority})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("update failed: %v", err)
			}

			issue, _ := store.Get(context.Background(), issueID)
			if issue.Priority != tt.expected {
				t.Errorf("expected priority %q, got %q", tt.expected, issue.Priority)
			}
		})
	}
}

func TestUpdateType(t *testing.T) {
	tests := []struct {
		typeFlag string
		expected storage.IssueType
	}{
		{"task", storage.TypeTask},
		{"bug", storage.TypeBug},
		{"feature", storage.TypeFeature},
		{"epic", storage.TypeEpic},
		{"chore", storage.TypeChore},
		{"FEATURE", storage.TypeFeature}, // test case insensitivity
	}

	for _, tt := range tests {
		t.Run(tt.typeFlag, func(t *testing.T) {
			app, store := setupTestApp(t)
			issueID := createTestIssue(t, store)

			cmd := newUpdateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{issueID, "--type", tt.typeFlag})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("update failed: %v", err)
			}

			issue, _ := store.Get(context.Background(), issueID)
			if issue.Type != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, issue.Type)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected storage.Status
	}{
		{"open", storage.StatusOpen},
		{"in-progress", storage.StatusInProgress},
		{"in_progress", storage.StatusInProgress}, // alternative format
		{"blocked", storage.StatusBlocked},
		{"deferred", storage.StatusDeferred},
		{"closed", storage.StatusClosed},
		{"BLOCKED", storage.StatusBlocked}, // test case insensitivity
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			app, store := setupTestApp(t)
			issueID := createTestIssue(t, store)

			cmd := newUpdateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{issueID, "--status", tt.status})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("update failed: %v", err)
			}

			issue, _ := store.Get(context.Background(), issueID)
			if issue.Status != tt.expected {
				t.Errorf("expected status %q, got %q", tt.expected, issue.Status)
			}
		})
	}
}

func TestUpdateAssignee(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--assignee", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if issue.Assignee != "alice" {
		t.Errorf("expected assignee %q, got %q", "alice", issue.Assignee)
	}
}

func TestUpdateUnassign(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	// Verify issue has an assignee initially
	issue, _ := store.Get(context.Background(), issueID)
	if issue.Assignee == "" {
		t.Fatal("test issue should have an assignee initially")
	}

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--assignee", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ = store.Get(context.Background(), issueID)
	if issue.Assignee != "" {
		t.Errorf("expected empty assignee, got %q", issue.Assignee)
	}
}

func TestUpdateAddLabel(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--add-label", "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if !contains(issue.Labels, "urgent") {
		t.Errorf("expected labels to contain 'urgent', got %v", issue.Labels)
	}
	// Should still have original labels
	if !contains(issue.Labels, "backend") || !contains(issue.Labels, "api") {
		t.Errorf("original labels should be preserved, got %v", issue.Labels)
	}
}

func TestUpdateAddMultipleLabels(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--add-label", "urgent", "--add-label", "frontend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if !contains(issue.Labels, "urgent") || !contains(issue.Labels, "frontend") {
		t.Errorf("expected labels to contain 'urgent' and 'frontend', got %v", issue.Labels)
	}
}

func TestUpdateAddDuplicateLabel(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	// Add a label that already exists
	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--add-label", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	// Count occurrences of "backend"
	count := 0
	for _, l := range issue.Labels {
		if l == "backend" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected only one 'backend' label, got %d in %v", count, issue.Labels)
	}
}

func TestUpdateRemoveLabel(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--remove-label", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if contains(issue.Labels, "backend") {
		t.Errorf("expected 'backend' label to be removed, got %v", issue.Labels)
	}
	// api label should still be there
	if !contains(issue.Labels, "api") {
		t.Errorf("'api' label should be preserved, got %v", issue.Labels)
	}
}

func TestUpdateAddAndRemoveLabels(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--add-label", "urgent", "--remove-label", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if contains(issue.Labels, "backend") {
		t.Errorf("expected 'backend' label to be removed")
	}
	if !contains(issue.Labels, "urgent") {
		t.Errorf("expected 'urgent' label to be added")
	}
	if !contains(issue.Labels, "api") {
		t.Errorf("'api' label should be preserved")
	}
}

func TestUpdateMultipleFields(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{
		issueID,
		"--title", "New title",
		"--priority", "1",
		"--status", "in-progress",
		"--assignee", "bob",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), issueID)
	if issue.Title != "New title" {
		t.Errorf("title mismatch")
	}
	if issue.Priority != storage.PriorityHigh {
		t.Errorf("priority mismatch")
	}
	if issue.Status != storage.StatusInProgress {
		t.Errorf("status mismatch")
	}
	if issue.Assignee != "bob" {
		t.Errorf("assignee mismatch")
	}
}

func TestUpdateWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--title", "New title"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Update now returns array of full issue objects to match original beads
	var results []IssueJSON
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 issue in result, got %d", len(results))
	}

	if results[0].ID != issueID {
		t.Errorf("expected id %q, got %q", issueID, results[0].ID)
	}
	if results[0].Title != "New title" {
		t.Errorf("expected title 'New title', got %q", results[0].Title)
	}
}

func TestUpdateNoChanges(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no changes specified")
	}
	if !strings.Contains(err.Error(), "no changes") {
		t.Errorf("expected error about no changes, got: %v", err)
	}
}

func TestUpdateInvalidPriority(t *testing.T) {
	// Test that word priorities are rejected (must use 0-4 or P0-P4)
	invalidPriorities := []string{"invalid", "medium", "high", "low", "critical"}

	for _, priority := range invalidPriorities {
		t.Run(priority, func(t *testing.T) {
			app, store := setupTestApp(t)
			issueID := createTestIssue(t, store)

			cmd := newUpdateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{issueID, "--priority", priority})
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for priority %q", priority)
			}
			if !strings.Contains(err.Error(), "invalid priority") {
				t.Errorf("expected error about invalid priority, got: %v", err)
			}
			if !strings.Contains(err.Error(), "not words like high/medium/low") {
				t.Errorf("expected error message to mention word restriction, got: %v", err)
			}
		})
	}
}

func TestUpdateInvalidType(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--type", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("expected error about invalid type, got: %v", err)
	}
}

func TestUpdateInvalidStatus(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--status", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("expected error about invalid status, got: %v", err)
	}
}

func TestUpdateNonExistentIssue(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent", "--title", "New title"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
}

func TestUpdateNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no issue ID provided")
	}
}

func TestUpdateRemoveNonExistentLabel(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	// Removing a non-existent label should not cause an error
	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--remove-label", "nonexistent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Original labels should be preserved
	issue, _ := store.Get(context.Background(), issueID)
	if !contains(issue.Labels, "backend") || !contains(issue.Labels, "api") {
		t.Errorf("original labels should be preserved, got %v", issue.Labels)
	}
}

func TestUpdateIssueWithNoLabels(t *testing.T) {
	app, store := setupTestApp(t)

	// Create an issue without labels
	issue := &storage.Issue{
		Title: "No labels",
		Type:  storage.TypeTask,
	}
	id, _ := store.Create(context.Background(), issue)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--add-label", "new-label"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	updated, _ := store.Get(context.Background(), id)
	if len(updated.Labels) != 1 || updated.Labels[0] != "new-label" {
		t.Errorf("expected labels [new-label], got %v", updated.Labels)
	}
}

func TestUpdateSetParent(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, _ := store.Create(context.Background(), parent)
	childID, _ := store.Create(context.Background(), child)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{childID, "--parent", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update --parent failed: %v", err)
	}

	if !strings.Contains(out.String(), "Updated") {
		t.Errorf("expected 'Updated' in output, got %q", out.String())
	}

	// Verify child has parent
	gotChild, _ := store.Get(context.Background(), childID)
	if gotChild.Parent != parentID {
		t.Errorf("expected child parent %q, got %q", parentID, gotChild.Parent)
	}

	// Verify parent has child in children list
	gotParent, _ := store.Get(context.Background(), parentID)
	found := false
	for _, c := range gotParent.Children() {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected parent to have child %q in children list", childID)
	}
}

func TestUpdateReparent(t *testing.T) {
	app, store := setupTestApp(t)

	parent1 := &storage.Issue{Title: "Parent 1", Type: storage.TypeEpic}
	parent2 := &storage.Issue{Title: "Parent 2", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parent1ID, _ := store.Create(context.Background(), parent1)
	parent2ID, _ := store.Create(context.Background(), parent2)
	childID, _ := store.Create(context.Background(), child)

	// Set initial parent
	if err := store.AddDependency(context.Background(), childID, parent1ID, storage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to set initial parent: %v", err)
	}

	// Re-parent via update
	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{childID, "--parent", parent2ID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update --parent (reparent) failed: %v", err)
	}

	gotChild, _ := store.Get(context.Background(), childID)
	if gotChild.Parent != parent2ID {
		t.Errorf("expected child parent %q, got %q", parent2ID, gotChild.Parent)
	}

	// Old parent should not have child
	gotOldParent, _ := store.Get(context.Background(), parent1ID)
	for _, c := range gotOldParent.Children() {
		if c == childID {
			t.Errorf("old parent should not have child in children list")
		}
	}

	// New parent should have child
	gotNewParent, _ := store.Get(context.Background(), parent2ID)
	found := false
	for _, c := range gotNewParent.Children() {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("new parent should have child in children list")
	}
}

func TestUpdateRemoveParent(t *testing.T) {
	app, store := setupTestApp(t)

	parent := &storage.Issue{Title: "Parent", Type: storage.TypeEpic}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, _ := store.Create(context.Background(), parent)
	childID, _ := store.Create(context.Background(), child)

	if err := store.AddDependency(context.Background(), childID, parentID, storage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Remove parent via update --parent ""
	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{childID, "--parent", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update --parent '' failed: %v", err)
	}

	gotChild, _ := store.Get(context.Background(), childID)
	if gotChild.Parent != "" {
		t.Errorf("expected child to have no parent, got %q", gotChild.Parent)
	}

	gotParent, _ := store.Get(context.Background(), parentID)
	for _, c := range gotParent.Children() {
		if c == childID {
			t.Errorf("parent should not have child in children list after removal")
		}
	}
}

func TestUpdateParentNotFound(t *testing.T) {
	app, store := setupTestApp(t)

	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}
	childID, _ := store.Create(context.Background(), child)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{childID, "--parent", "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent parent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestUpdateParentCycle(t *testing.T) {
	app, store := setupTestApp(t)

	issueA := &storage.Issue{Title: "A", Type: storage.TypeTask}
	issueB := &storage.Issue{Title: "B", Type: storage.TypeTask}

	idA, _ := store.Create(context.Background(), issueA)
	idB, _ := store.Create(context.Background(), issueB)

	// Make A parent of B
	if err := store.AddDependency(context.Background(), idB, idA, storage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to set initial parent: %v", err)
	}

	// Try to make B parent of A (would create cycle)
	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{idA, "--parent", idB})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

func TestUpdateClaimUnassigned(t *testing.T) {
	app, store := setupTestApp(t)
	issue := &storage.Issue{
		Title:  "Unassigned issue",
		Type:   storage.TypeTask,
		Status: storage.StatusOpen,
	}
	id, _ := store.Create(context.Background(), issue)

	t.Setenv("BD_ACTOR", "test-actor")

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	got, _ := store.Get(context.Background(), id)
	if got.Assignee != "test-actor" {
		t.Errorf("expected assignee %q, got %q", "test-actor", got.Assignee)
	}
	if got.Status != storage.StatusInProgress {
		t.Errorf("expected status %q, got %q", storage.StatusInProgress, got.Status)
	}
}

func TestUpdateClaimAlreadyAssigned(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store) // has Assignee: "original-assignee"

	t.Setenv("BD_ACTOR", "test-actor")

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--claim"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when claiming already-assigned issue")
	}
	if !strings.Contains(err.Error(), "already assigned") {
		t.Errorf("expected 'already assigned' in error, got: %v", err)
	}
}

func TestUpdateClaimResolvesFromBDActor(t *testing.T) {
	app, store := setupTestApp(t)
	issue := &storage.Issue{
		Title:  "Unassigned",
		Type:   storage.TypeTask,
		Status: storage.StatusOpen,
	}
	id, _ := store.Create(context.Background(), issue)

	t.Setenv("BD_ACTOR", "env-actor")

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	got, _ := store.Get(context.Background(), id)
	if got.Assignee != "env-actor" {
		t.Errorf("expected assignee %q from BD_ACTOR, got %q", "env-actor", got.Assignee)
	}
}

func TestUpdateClaimResolvesFromGitConfig(t *testing.T) {
	app, store := setupTestApp(t)
	issue := &storage.Issue{
		Title:  "Unassigned",
		Type:   storage.TypeTask,
		Status: storage.StatusOpen,
	}
	id, _ := store.Create(context.Background(), issue)

	// Clear BD_ACTOR so resolution falls through to git config user.name or $USER
	t.Setenv("BD_ACTOR", "")

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	got, _ := store.Get(context.Background(), id)
	if got.Assignee == "" {
		t.Error("expected non-empty assignee from git config or $USER fallback")
	}
	if got.Status != storage.StatusInProgress {
		t.Errorf("expected status %q, got %q", storage.StatusInProgress, got.Status)
	}
}
