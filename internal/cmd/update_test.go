package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/internal/storage"
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

	cmd := NewUpdateCmd(app)
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

func TestUpdateDescription(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := NewUpdateCmd(app)
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
		{"critical", storage.PriorityCritical},
		{"high", storage.PriorityHigh},
		{"medium", storage.PriorityMedium},
		{"low", storage.PriorityLow},
		{"HIGH", storage.PriorityHigh}, // test case insensitivity
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			app, store := setupTestApp(t)
			issueID := createTestIssue(t, store)

			cmd := NewUpdateCmd(app)
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

			cmd := NewUpdateCmd(app)
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

			cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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
	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
	cmd.SetArgs([]string{
		issueID,
		"--title", "New title",
		"--priority", "high",
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

	cmd := NewUpdateCmd(app)
	cmd.SetArgs([]string{issueID, "--title", "New title"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["id"] != issueID {
		t.Errorf("expected id %q, got %q", issueID, result["id"])
	}
	if result["status"] != "updated" {
		t.Errorf("expected status 'updated', got %q", result["status"])
	}
}

func TestUpdateNoChanges(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := NewUpdateCmd(app)
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
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := NewUpdateCmd(app)
	cmd.SetArgs([]string{issueID, "--priority", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid priority")
	}
	if !strings.Contains(err.Error(), "invalid priority") {
		t.Errorf("expected error about invalid priority, got: %v", err)
	}
}

func TestUpdateInvalidType(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
	cmd.SetArgs([]string{"bd-nonexistent", "--title", "New title"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
}

func TestUpdateNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := NewUpdateCmd(app)
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
	cmd := NewUpdateCmd(app)
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

	cmd := NewUpdateCmd(app)
	cmd.SetArgs([]string{id, "--add-label", "new-label"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	updated, _ := store.Get(context.Background(), id)
	if len(updated.Labels) != 1 || updated.Labels[0] != "new-label" {
		t.Errorf("expected labels [new-label], got %v", updated.Labels)
	}
}
