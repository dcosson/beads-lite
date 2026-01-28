package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/internal/storage"
)

func TestDeleteWithForce(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if !strings.Contains(out.String(), "Deleted") {
		t.Errorf("expected 'Deleted' in output, got %q", out.String())
	}

	// Verify issue is deleted
	_, err := store.Get(context.Background(), issueID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["id"] != issueID {
		t.Errorf("expected id %q, got %q", issueID, result["id"])
	}
	if result["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %q", result["status"])
	}
}

func TestDeleteNonExistentIssue(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestDeleteNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no issue ID provided")
	}
}

func TestDeleteByPrefix(t *testing.T) {
	app, store := setupTestApp(t)

	// Create an issue
	issue := &storage.Issue{
		Title: "Test issue for prefix",
		Type:  storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Get a prefix of the ID (the first few characters)
	prefix := id[:len(id)-2] // Remove last 2 chars to get prefix

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete by prefix failed: %v", err)
	}

	// Verify issue is deleted
	_, err = store.Get(context.Background(), id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteAmbiguousPrefix(t *testing.T) {
	app, store := setupTestApp(t)

	// Create two issues - their IDs will both start with "bd-"
	issue1 := &storage.Issue{Title: "Issue 1", Type: storage.TypeTask}
	issue2 := &storage.Issue{Title: "Issue 2", Type: storage.TypeTask}

	_, err := store.Create(context.Background(), issue1)
	if err != nil {
		t.Fatalf("failed to create issue 1: %v", err)
	}
	_, err = store.Create(context.Background(), issue2)
	if err != nil {
		t.Fatalf("failed to create issue 2: %v", err)
	}

	// Try to delete with ambiguous prefix "bd-" (matches both)
	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-", "--force"})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for ambiguous prefix")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

func TestDeleteClosedIssue(t *testing.T) {
	app, store := setupTestApp(t)

	// Create and close an issue
	issue := &storage.Issue{
		Title: "Closed issue",
		Type:  storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Close(context.Background(), id); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete closed issue failed: %v", err)
	}

	// Verify issue is deleted
	_, err = store.Get(context.Background(), id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteByPrefixClosedIssue(t *testing.T) {
	app, store := setupTestApp(t)

	// Create and close an issue
	issue := &storage.Issue{
		Title: "Closed issue for prefix test",
		Type:  storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Close(context.Background(), id); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Get a prefix of the ID
	prefix := id[:len(id)-2]

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete by prefix for closed issue failed: %v", err)
	}

	// Verify issue is deleted
	_, err = store.Get(context.Background(), id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteShortFlag(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "-f"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete with -f flag failed: %v", err)
	}

	// Verify issue is deleted
	_, err := store.Get(context.Background(), issueID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
