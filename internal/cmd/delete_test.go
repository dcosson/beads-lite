package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/storage"
)

func TestDeleteSoftDeleteDefault(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if !strings.Contains(out.String(), "Tombstoned") {
		t.Errorf("expected 'Tombstoned' in output, got %q", out.String())
	}

	// Verify issue still exists via Get() but has tombstone status
	issue, err := store.Get(context.Background(), issueID)
	if err != nil {
		t.Fatalf("Get after soft-delete should succeed: %v", err)
	}
	if issue.Status != storage.StatusTombstone {
		t.Errorf("expected status tombstone, got %q", issue.Status)
	}
	if issue.DeletedAt == nil {
		t.Error("DeletedAt should be set")
	}
	if issue.DeleteReason != "deleted" {
		t.Errorf("expected default reason 'deleted', got %q", issue.DeleteReason)
	}
}

func TestDeleteHardFlag(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--force", "--hard"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --hard failed: %v", err)
	}

	if !strings.Contains(out.String(), "Deleted") {
		t.Errorf("expected 'Deleted' in output, got %q", out.String())
	}

	// Verify issue is permanently gone
	_, err := store.Get(context.Background(), issueID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after hard delete, got %v", err)
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

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["deleted"] != issueID {
		t.Errorf("expected deleted %q, got %v", issueID, result["deleted"])
	}
	if result["dependencies_removed"] != float64(0) {
		t.Errorf("expected dependencies_removed 0, got %v", result["dependencies_removed"])
	}
	if result["references_updated"] != float64(0) {
		t.Errorf("expected references_updated 0, got %v", result["references_updated"])
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

	issue := &storage.Issue{
		Title: "Test issue for prefix",
		Type:  storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	prefix := id[:len(id)-2]

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete by prefix failed: %v", err)
	}

	// Verify issue is tombstoned (soft delete is default)
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get after soft delete should succeed: %v", err)
	}
	if got.Status != storage.StatusTombstone {
		t.Errorf("expected tombstone status, got %q", got.Status)
	}
}

func TestDeleteAmbiguousPrefix(t *testing.T) {
	app, store := setupTestApp(t)

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

	// Verify issue is tombstoned
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get after soft delete should succeed: %v", err)
	}
	if got.Status != storage.StatusTombstone {
		t.Errorf("expected tombstone status, got %q", got.Status)
	}
}

func TestDeleteByPrefixClosedIssue(t *testing.T) {
	app, store := setupTestApp(t)

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

	prefix := id[:len(id)-2]

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete by prefix for closed issue failed: %v", err)
	}

	// Verify issue is tombstoned
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get after soft delete should succeed: %v", err)
	}
	if got.Status != storage.StatusTombstone {
		t.Errorf("expected tombstone status, got %q", got.Status)
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

	// Verify issue is tombstoned
	got, err := store.Get(context.Background(), issueID)
	if err != nil {
		t.Fatalf("Get after soft delete should succeed: %v", err)
	}
	if got.Status != storage.StatusTombstone {
		t.Errorf("expected tombstone status, got %q", got.Status)
	}
}

func TestDeleteDryRun(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--dry-run", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --dry-run failed: %v", err)
	}

	if !strings.Contains(out.String(), "[dry-run]") {
		t.Errorf("expected '[dry-run]' in output, got %q", out.String())
	}

	// Verify issue still exists unchanged
	got, err := store.Get(context.Background(), issueID)
	if err != nil {
		t.Fatalf("Get after dry-run should succeed: %v", err)
	}
	if got.Status == storage.StatusTombstone {
		t.Error("Issue should not be tombstoned after dry-run")
	}
}

func TestDeleteWithReason(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--force", "--reason", "duplicate of other-id"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --reason failed: %v", err)
	}

	got, err := store.Get(context.Background(), issueID)
	if err != nil {
		t.Fatalf("Get after soft delete should succeed: %v", err)
	}
	if got.DeleteReason != "duplicate of other-id" {
		t.Errorf("expected reason 'duplicate of other-id', got %q", got.DeleteReason)
	}
}

func TestDeleteFromFile(t *testing.T) {
	app, store := setupTestApp(t)

	// Create multiple issues
	id1 := createTestIssue(t, store)
	id2 := createTestIssue(t, store)
	id3 := createTestIssue(t, store)

	// Write IDs to a temp file
	tmpDir := t.TempDir()
	idFile := filepath.Join(tmpDir, "ids.txt")
	content := id2 + "\n# comment line\n\n" + id3 + "\n"
	if err := os.WriteFile(idFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write ID file: %v", err)
	}

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id1, "--force", "--from-file", idFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --from-file failed: %v", err)
	}

	// Verify all three are tombstoned
	for _, id := range []string{id1, id2, id3} {
		got, err := store.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get %s after delete should succeed: %v", id, err)
		}
		if got.Status != storage.StatusTombstone {
			t.Errorf("expected tombstone for %s, got %q", id, got.Status)
		}
	}
}

func TestDeleteTextReferenceRewriting(t *testing.T) {
	app, store := setupTestApp(t)
	ctx := context.Background()

	// Create two issues
	refIssue := &storage.Issue{Title: "Referencing issue", Type: storage.TypeTask}
	delIssue := &storage.Issue{Title: "Issue to delete", Type: storage.TypeTask}

	refID, err := store.Create(ctx, refIssue)
	if err != nil {
		t.Fatalf("Create ref issue failed: %v", err)
	}
	delID, err := store.Create(ctx, delIssue)
	if err != nil {
		t.Fatalf("Create del issue failed: %v", err)
	}

	// Set description with reference and add dependency
	refIssue, _ = store.Get(ctx, refID)
	refIssue.Description = "See " + delID + " for details"
	store.Update(ctx, refIssue)

	store.AddDependency(ctx, refID, delID, storage.DepTypeBlocks)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{delID, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify description was rewritten
	got, err := store.Get(ctx, refID)
	if err != nil {
		t.Fatalf("Get ref issue failed: %v", err)
	}
	expected := "See [deleted:" + delID + "] for details"
	if got.Description != expected {
		t.Errorf("expected description %q, got %q", expected, got.Description)
	}
}

func TestDeleteCascadeSoftDelete(t *testing.T) {
	app, store := setupTestApp(t)
	ctx := context.Background()

	parent := &storage.Issue{Title: "Parent", Type: storage.TypeTask}
	child := &storage.Issue{Title: "Child", Type: storage.TypeTask}

	parentID, _ := store.Create(ctx, parent)
	childID, _ := store.Create(ctx, child)

	// child depends on parent (so child is a dependent of parent)
	store.AddDependency(ctx, childID, parentID, storage.DepTypeBlocks)

	cmd := newDeleteCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID, "--cascade", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cascade delete failed: %v", err)
	}

	// Both should be tombstoned
	gotParent, err := store.Get(ctx, parentID)
	if err != nil {
		t.Fatalf("Get parent should succeed: %v", err)
	}
	if gotParent.Status != storage.StatusTombstone {
		t.Errorf("parent should be tombstoned, got %q", gotParent.Status)
	}

	gotChild, err := store.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child should succeed: %v", err)
	}
	if gotChild.Status != storage.StatusTombstone {
		t.Errorf("child should be tombstoned, got %q", gotChild.Status)
	}
}

func TestUpdateRejectsTombstoneStatus(t *testing.T) {
	app, store := setupTestApp(t)
	issueID := createTestIssue(t, store)

	cmd := newUpdateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{issueID, "--status", "tombstone"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when setting status to tombstone")
	}
	if !strings.Contains(err.Error(), "cannot set status to tombstone") {
		t.Errorf("expected tombstone rejection error, got: %v", err)
	}
}
