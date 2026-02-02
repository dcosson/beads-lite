package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

func TestCommentsAddBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue for comments",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "This is a test comment"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments add failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added comment to "+id) {
		t.Errorf("expected output to contain 'Added comment to %s', got %q", id, output)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}
	if got.Comments[0].Text != "This is a test comment" {
		t.Errorf("expected comment body %q, got %q", "This is a test comment", got.Comments[0].Text)
	}
}

func TestCommentsAddWithAuthor(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Issue for author test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "Comment with author", "--author", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments add failed: %v", err)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}
	if got.Comments[0].Author != "alice" {
		t.Errorf("expected author %q, got %q", "alice", got.Comments[0].Author)
	}
}

func TestCommentsAddFromFile(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Issue for file test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Write a temp file with comment content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "comment.txt")
	if err := os.WriteFile(tmpFile, []byte("Comment from file\n"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "-f", tmpFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments add failed: %v", err)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}
	if got.Comments[0].Text != "Comment from file" {
		t.Errorf("expected comment body %q, got %q", "Comment from file", got.Comments[0].Text)
	}
}

func TestCommentsAddNonExistent(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent", "Comment"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got %q", err.Error())
	}
}

func TestCommentsAddEmptyMessage(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Issue for empty message test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, ""})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for empty message")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected error to mention 'cannot be empty', got %q", err.Error())
	}
}

func TestCommentsAddJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue for JSON test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "JSON comment", "--author", "bob"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments add failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["issue_id"] != id {
		t.Errorf("expected issue_id %q, got %q", id, result["issue_id"])
	}
	if result["text"] != "JSON comment" {
		t.Errorf("expected text %q, got %q", "JSON comment", result["text"])
	}
	if result["author"] != "bob" {
		t.Errorf("expected author %q, got %q", "bob", result["author"])
	}
}

func TestCommentsListBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue with comments",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	comment1 := &issuestorage.Comment{Author: "alice", Text: "First comment"}
	comment2 := &issuestorage.Comment{Author: "bob", Text: "Second comment"}
	if err := store.AddComment(context.Background(), id, comment1); err != nil {
		t.Fatalf("failed to add comment 1: %v", err)
	}
	if err := store.AddComment(context.Background(), id, comment2); err != nil {
		t.Fatalf("failed to add comment 2: %v", err)
	}

	cmd := newCommentsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "alice") {
		t.Errorf("expected output to contain 'alice', got %q", output)
	}
	if !strings.Contains(output, "First comment") {
		t.Errorf("expected output to contain 'First comment', got %q", output)
	}
	if !strings.Contains(output, "bob") {
		t.Errorf("expected output to contain 'bob', got %q", output)
	}
	if !strings.Contains(output, "Second comment") {
		t.Errorf("expected output to contain 'Second comment', got %q", output)
	}
}

func TestCommentsListEmpty(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue without comments",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No comments") {
		t.Errorf("expected output to contain 'No comments', got %q", output)
	}
}

func TestCommentsListNonExistent(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCommentsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got %q", err.Error())
	}
}

func TestCommentsListJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue for JSON list",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	comment := &issuestorage.Comment{Author: "alice", Text: "Test comment"}
	if err := store.AddComment(context.Background(), id, comment); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	cmd := newCommentsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments list failed: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(result))
	}
	if result[0]["author"] != "alice" {
		t.Errorf("expected author %q, got %q", "alice", result[0]["author"])
	}
	if result[0]["text"] != "Test comment" {
		t.Errorf("expected text %q, got %q", "Test comment", result[0]["text"])
	}
}

func TestCommentsListNoAuthor(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Issue for no-author test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	comment := &issuestorage.Comment{Text: "Anonymous comment"}
	if err := store.AddComment(context.Background(), id, comment); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	cmd := newCommentsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comments list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Anonymous comment") {
		t.Errorf("expected output to contain 'Anonymous comment', got %q", output)
	}
	if strings.Contains(output, ": Anonymous") {
		t.Errorf("expected output without colon for anonymous comment, got %q", output)
	}
}

func TestCommentsNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided to add")
	}

	cmd2 := newCommentsCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided to comments")
	}
}

func TestCommentsAddNoMessageArg(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Issue for no-message test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// No message arg and no -f flag should error
	cmd := newCommentsAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when no message provided")
	}
	if !strings.Contains(err.Error(), "comment message required") {
		t.Errorf("expected error about message required, got %q", err.Error())
	}
}
