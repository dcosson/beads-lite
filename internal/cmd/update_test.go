package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

func createTestIssue(t *testing.T, store issuestorage.IssueStore) string {
	t.Helper()
	issue := &issuestorage.Issue{
		Title:       "Original title",
		Description: "Original description",
		Type:        issuestorage.TypeTask,
		Priority:    issuestorage.PriorityMedium,
		Status:      issuestorage.StatusOpen,
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
		expected issuestorage.Priority
	}{
		{"0", issuestorage.PriorityCritical},
		{"p0", issuestorage.PriorityCritical},
		{"P0", issuestorage.PriorityCritical}, // test case insensitivity
		{"1", issuestorage.PriorityHigh},
		{"p1", issuestorage.PriorityHigh},
		{"2", issuestorage.PriorityMedium},
		{"p2", issuestorage.PriorityMedium},
		{"3", issuestorage.PriorityLow},
		{"p3", issuestorage.PriorityLow},
		{"4", issuestorage.PriorityBacklog},
		{"p4", issuestorage.PriorityBacklog},
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
		expected issuestorage.IssueType
	}{
		{"task", issuestorage.TypeTask},
		{"bug", issuestorage.TypeBug},
		{"feature", issuestorage.TypeFeature},
		{"epic", issuestorage.TypeEpic},
		{"chore", issuestorage.TypeChore},
		{"FEATURE", issuestorage.TypeFeature}, // test case insensitivity
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
		expected issuestorage.Status
	}{
		{"open", issuestorage.StatusOpen},
		{"in-progress", issuestorage.StatusInProgress},
		{"in_progress", issuestorage.StatusInProgress}, // alternative format
		{"blocked", issuestorage.StatusBlocked},
		{"deferred", issuestorage.StatusDeferred},
		{"closed", issuestorage.StatusClosed},
		{"BLOCKED", issuestorage.StatusBlocked}, // test case insensitivity
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
	if issue.Priority != issuestorage.PriorityHigh {
		t.Errorf("priority mismatch")
	}
	if issue.Status != issuestorage.StatusInProgress {
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
	issue := &issuestorage.Issue{
		Title: "No labels",
		Type:  issuestorage.TypeTask,
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

	parent := &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic}
	child := &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask}

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

	parent1 := &issuestorage.Issue{Title: "Parent 1", Type: issuestorage.TypeEpic}
	parent2 := &issuestorage.Issue{Title: "Parent 2", Type: issuestorage.TypeEpic}
	child := &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask}

	parent1ID, _ := store.Create(context.Background(), parent1)
	parent2ID, _ := store.Create(context.Background(), parent2)
	childID, _ := store.Create(context.Background(), child)

	// Set initial parent
	if err := store.AddDependency(context.Background(), childID, parent1ID, issuestorage.DepTypeParentChild); err != nil {
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

	parent := &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic}
	child := &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask}

	parentID, _ := store.Create(context.Background(), parent)
	childID, _ := store.Create(context.Background(), child)

	if err := store.AddDependency(context.Background(), childID, parentID, issuestorage.DepTypeParentChild); err != nil {
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

	child := &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask}
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

	issueA := &issuestorage.Issue{Title: "A", Type: issuestorage.TypeTask}
	issueB := &issuestorage.Issue{Title: "B", Type: issuestorage.TypeTask}

	idA, _ := store.Create(context.Background(), issueA)
	idB, _ := store.Create(context.Background(), issueB)

	// Make A parent of B
	if err := store.AddDependency(context.Background(), idB, idA, issuestorage.DepTypeParentChild); err != nil {
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
	app.ConfigStore = &mapConfigStore{data: map[string]string{"actor": "test-agent"}}

	issue := &issuestorage.Issue{
		Title:  "Unassigned task",
		Type:   issuestorage.TypeTask,
		Status: issuestorage.StatusOpen,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	got, _ := store.Get(context.Background(), id)
	if got.Assignee != "test-agent" {
		t.Errorf("expected assignee %q, got %q", "test-agent", got.Assignee)
	}
	if got.Status != issuestorage.StatusInProgress {
		t.Errorf("expected status %q, got %q", issuestorage.StatusInProgress, got.Status)
	}
}

func TestUpdateClaimAlreadyAssigned(t *testing.T) {
	app, store := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{"actor": "test-agent"}}

	issue := &issuestorage.Issue{
		Title:    "Assigned task",
		Type:     issuestorage.TypeTask,
		Status:   issuestorage.StatusOpen,
		Assignee: "someone-else",
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when claiming already-assigned issue")
	}
	if !strings.Contains(err.Error(), "already assigned") {
		t.Errorf("expected 'already assigned' in error, got: %v", err)
	}
}

func TestUpdateClaimResolvesBDActorEnv(t *testing.T) {
	app, store := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{"actor": "env-actor"}}

	issue := &issuestorage.Issue{
		Title:  "Unassigned task",
		Type:   issuestorage.TypeTask,
		Status: issuestorage.StatusOpen,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	got, _ := store.Get(context.Background(), id)
	if got.Assignee != "env-actor" {
		t.Errorf("expected assignee from BD_ACTOR %q, got %q", "env-actor", got.Assignee)
	}
}

func TestUpdateClaimResolvesGitConfig(t *testing.T) {
	// When BD_ACTOR is not set (config has default "${USER}"),
	// actor should resolve from git config user.name.
	name, err := resolveActor(&App{})
	if err != nil || name == "" || name == "unknown" {
		t.Skip("cannot resolve actor name, skipping git config resolution test")
	}

	app, store := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{"actor": "${USER}"}}

	issue := &issuestorage.Issue{
		Title:  "Unassigned task",
		Type:   issuestorage.TypeTask,
		Status: issuestorage.StatusOpen,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--claim"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	got, _ := store.Get(context.Background(), id)
	if got.Assignee != name {
		t.Errorf("expected assignee from git config %q, got %q", name, got.Assignee)
	}
}
