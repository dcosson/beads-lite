package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
)

func TestCommentAddBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create an issue to add a comment to
	issue := &storage.Issue{
		Title:    "Issue for comments",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "This is a test comment"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment add failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Added comment to "+id) {
		t.Errorf("expected output to contain 'Added comment to %s', got %q", id, output)
	}

	// Verify comment was added
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}
	if got.Comments[0].Body != "This is a test comment" {
		t.Errorf("expected comment body %q, got %q", "This is a test comment", got.Comments[0].Body)
	}
}

func TestCommentAddWithAuthor(t *testing.T) {
	app, store := setupTestApp(t)

	// Create an issue
	issue := &storage.Issue{
		Title:    "Issue for author test",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "Comment with author", "--author", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment add failed: %v", err)
	}

	// Verify author was set
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

func TestCommentAddNonExistent(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCommentAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent", "Comment"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got %q", err.Error())
	}
}

func TestCommentAddEmptyMessage(t *testing.T) {
	app, store := setupTestApp(t)

	// Create an issue
	issue := &storage.Issue{
		Title:    "Issue for empty message test",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, ""})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for empty message")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected error to mention 'cannot be empty', got %q", err.Error())
	}
}

func TestCommentAddJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create an issue
	issue := &storage.Issue{
		Title:    "Issue for JSON test",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id, "JSON comment", "--author", "bob"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment add failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["issue_id"] != id {
		t.Errorf("expected issue_id %q, got %q", id, result["issue_id"])
	}

	comment, ok := result["comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected comment to be a map, got %T", result["comment"])
	}
	if comment["body"] != "JSON comment" {
		t.Errorf("expected body %q, got %q", "JSON comment", comment["body"])
	}
	if comment["author"] != "bob" {
		t.Errorf("expected author %q, got %q", "bob", comment["author"])
	}
}

func TestCommentListBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create an issue with comments
	issue := &storage.Issue{
		Title:    "Issue with comments",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Add some comments
	comment1 := &storage.Comment{Author: "alice", Body: "First comment"}
	comment2 := &storage.Comment{Author: "bob", Body: "Second comment"}
	if err := store.AddComment(context.Background(), id, comment1); err != nil {
		t.Fatalf("failed to add comment 1: %v", err)
	}
	if err := store.AddComment(context.Background(), id, comment2); err != nil {
		t.Fatalf("failed to add comment 2: %v", err)
	}

	cmd := newCommentListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment list failed: %v", err)
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

func TestCommentListEmpty(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create an issue without comments
	issue := &storage.Issue{
		Title:    "Issue without comments",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCommentListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No comments") {
		t.Errorf("expected output to contain 'No comments', got %q", output)
	}
}

func TestCommentListNonExistent(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCommentListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got %q", err.Error())
	}
}

func TestCommentListJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create an issue with comments
	issue := &storage.Issue{
		Title:    "Issue for JSON list",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Add a comment
	comment := &storage.Comment{Author: "alice", Body: "Test comment"}
	if err := store.AddComment(context.Background(), id, comment); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	cmd := newCommentListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment list failed: %v", err)
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
	if result[0]["body"] != "Test comment" {
		t.Errorf("expected body %q, got %q", "Test comment", result[0]["body"])
	}
}

func TestCommentListNoAuthor(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create an issue
	issue := &storage.Issue{
		Title:    "Issue for no-author test",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Add a comment without author
	comment := &storage.Comment{Body: "Anonymous comment"}
	if err := store.AddComment(context.Background(), id, comment); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	cmd := newCommentListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("comment list failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Anonymous comment") {
		t.Errorf("expected output to contain 'Anonymous comment', got %q", output)
	}
	// Output should NOT have a colon with no author
	// Format should be "[timestamp] body" not "[timestamp] : body"
	if strings.Contains(output, ": Anonymous") {
		t.Errorf("expected output without colon for anonymous comment, got %q", output)
	}
}

func TestCommentNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCommentAddCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}

	cmd = newCommentListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}
}
