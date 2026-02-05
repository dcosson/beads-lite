package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

func TestCommentListDeprecationWarning(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	errOut := app.Err.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue for deprecated comment test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	comment := &issuestorage.Comment{Author: "alice", Text: "Test comment"}
	if err := addComment(context.Background(), store, id, comment); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	cmd := newCommentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment list failed: %v", err)
	}

	// Check deprecation warning on stderr
	errOutput := errOut.String()
	if !strings.Contains(errOutput, `"comment" is deprecated`) {
		t.Errorf("expected deprecation warning on stderr, got %q", errOutput)
	}

	// Check that listing still works
	output := out.String()
	if !strings.Contains(output, "alice") {
		t.Errorf("expected output to contain 'alice', got %q", output)
	}
	if !strings.Contains(output, "Test comment") {
		t.Errorf("expected output to contain 'Test comment', got %q", output)
	}
}

func TestCommentAddDeprecationWarning(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	errOut := app.Err.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue for deprecated comment add test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"add", id, "Deprecated add test"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment add failed: %v", err)
	}

	// Check deprecation warning on stderr
	errOutput := errOut.String()
	if !strings.Contains(errOutput, `"comment" is deprecated`) {
		t.Errorf("expected deprecation warning on stderr, got %q", errOutput)
	}

	// Check that add still works
	output := out.String()
	if !strings.Contains(output, "Added comment to "+id) {
		t.Errorf("expected output to contain 'Added comment to %s', got %q", id, output)
	}

	// Verify comment was stored
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}
	if got.Comments[0].Text != "Deprecated add test" {
		t.Errorf("expected comment body %q, got %q", "Deprecated add test", got.Comments[0].Text)
	}
}
