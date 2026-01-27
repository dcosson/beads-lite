package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"beads2/filesystem"
	"beads2/storage"
)

func setupTestStorage(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	return store
}

func createTestIssue(t *testing.T, store storage.Storage) string {
	t.Helper()
	ctx := context.Background()
	issue := &storage.Issue{
		Title:    "Test issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(ctx, issue)
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	return id
}

func TestCommentAdd(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	err := CommentAdd(ctx, store, CommentAddOptions{
		IssueID: issueID,
		Body:    "This is a test comment",
		Author:  "alice",
	})
	if err != nil {
		t.Fatalf("CommentAdd: %v", err)
	}

	// Verify comment was added
	issue, err := store.Get(ctx, issueID)
	if err != nil {
		t.Fatalf("Get issue: %v", err)
	}
	if len(issue.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(issue.Comments))
	}
	c := issue.Comments[0]
	if c.Author != "alice" {
		t.Errorf("expected author 'alice', got %q", c.Author)
	}
	if c.Body != "This is a test comment" {
		t.Errorf("expected body 'This is a test comment', got %q", c.Body)
	}
	if c.ID == "" {
		t.Error("expected comment ID to be generated")
	}
}

func TestCommentAddFromStdin(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	stdin := strings.NewReader("Comment from stdin")
	err := CommentAdd(ctx, store, CommentAddOptions{
		IssueID: issueID,
		Body:    "-",
		Author:  "bob",
		Stdin:   stdin,
	})
	if err != nil {
		t.Fatalf("CommentAdd: %v", err)
	}

	issue, err := store.Get(ctx, issueID)
	if err != nil {
		t.Fatalf("Get issue: %v", err)
	}
	if len(issue.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(issue.Comments))
	}
	if issue.Comments[0].Body != "Comment from stdin" {
		t.Errorf("expected body from stdin, got %q", issue.Comments[0].Body)
	}
}

func TestCommentAddDefaultAuthor(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	// Don't set Author, should fall back to $USER
	expectedAuthor := os.Getenv("USER")
	if expectedAuthor == "" {
		expectedAuthor = "unknown"
	}

	err := CommentAdd(ctx, store, CommentAddOptions{
		IssueID: issueID,
		Body:    "Comment with default author",
	})
	if err != nil {
		t.Fatalf("CommentAdd: %v", err)
	}

	issue, err := store.Get(ctx, issueID)
	if err != nil {
		t.Fatalf("Get issue: %v", err)
	}
	if issue.Comments[0].Author != expectedAuthor {
		t.Errorf("expected author %q, got %q", expectedAuthor, issue.Comments[0].Author)
	}
}

func TestCommentAddMissingIssueID(t *testing.T) {
	store := setupTestStorage(t)
	ctx := context.Background()

	err := CommentAdd(ctx, store, CommentAddOptions{
		Body:   "Some comment",
		Author: "alice",
	})
	if err == nil {
		t.Fatal("expected error for missing issue ID")
	}
	if !strings.Contains(err.Error(), "issue ID is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCommentAddEmptyBody(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	err := CommentAdd(ctx, store, CommentAddOptions{
		IssueID: issueID,
		Body:    "",
		Author:  "alice",
	})
	if err == nil {
		t.Fatal("expected error for empty body")
	}
	if !strings.Contains(err.Error(), "comment body is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCommentAddIssueNotFound(t *testing.T) {
	store := setupTestStorage(t)
	ctx := context.Background()

	err := CommentAdd(ctx, store, CommentAddOptions{
		IssueID: "bd-xxxx",
		Body:    "Some comment",
		Author:  "alice",
	})
	if err == nil {
		t.Fatal("expected error for non-existent issue")
	}
}

func TestCommentList(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	// Add comments with different timestamps
	now := time.Now()
	comments := []storage.Comment{
		{Author: "alice", Body: "First comment", CreatedAt: now.Add(-2 * time.Hour)},
		{Author: "bob", Body: "Second comment", CreatedAt: now.Add(-1 * time.Hour)},
		{Author: "carol", Body: "Third comment", CreatedAt: now},
	}
	for i := range comments {
		if err := store.AddComment(ctx, issueID, &comments[i]); err != nil {
			t.Fatalf("AddComment: %v", err)
		}
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CommentList(ctx, store, CommentListOptions{
		IssueID: issueID,
	})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CommentList: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Default order is newest first
	aliceIdx := strings.Index(output, "alice")
	bobIdx := strings.Index(output, "bob")
	carolIdx := strings.Index(output, "carol")

	if carolIdx > bobIdx || bobIdx > aliceIdx {
		t.Errorf("expected newest first order (carol, bob, alice), got output:\n%s", output)
	}
}

func TestCommentListReverse(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	// Add comments
	now := time.Now()
	comments := []storage.Comment{
		{Author: "alice", Body: "First comment", CreatedAt: now.Add(-2 * time.Hour)},
		{Author: "bob", Body: "Second comment", CreatedAt: now.Add(-1 * time.Hour)},
		{Author: "carol", Body: "Third comment", CreatedAt: now},
	}
	for i := range comments {
		if err := store.AddComment(ctx, issueID, &comments[i]); err != nil {
			t.Fatalf("AddComment: %v", err)
		}
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CommentList(ctx, store, CommentListOptions{
		IssueID: issueID,
		Reverse: true,
	})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CommentList: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Reverse order is oldest first (chronological)
	aliceIdx := strings.Index(output, "alice")
	bobIdx := strings.Index(output, "bob")
	carolIdx := strings.Index(output, "carol")

	if aliceIdx > bobIdx || bobIdx > carolIdx {
		t.Errorf("expected chronological order (alice, bob, carol), got output:\n%s", output)
	}
}

func TestCommentListEmpty(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CommentList(ctx, store, CommentListOptions{
		IssueID: issueID,
	})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CommentList: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No comments") {
		t.Errorf("expected 'No comments' for empty list, got: %s", output)
	}
}

func TestCommentListMissingIssueID(t *testing.T) {
	store := setupTestStorage(t)
	ctx := context.Background()

	err := CommentList(ctx, store, CommentListOptions{})
	if err == nil {
		t.Fatal("expected error for missing issue ID")
	}
	if !strings.Contains(err.Error(), "issue ID is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCommentListIssueNotFound(t *testing.T) {
	store := setupTestStorage(t)
	ctx := context.Background()

	err := CommentList(ctx, store, CommentListOptions{
		IssueID: "bd-xxxx",
	})
	if err == nil {
		t.Fatal("expected error for non-existent issue")
	}
}

func TestCommentAddJSON(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CommentAdd(ctx, store, CommentAddOptions{
		IssueID: issueID,
		Body:    "JSON test comment",
		Author:  "alice",
		JSON:    true,
	})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CommentAdd: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result CommentAddResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if result.IssueID != issueID {
		t.Errorf("expected issue_id %q, got %q", issueID, result.IssueID)
	}
	if result.CommentID == "" {
		t.Error("expected comment_id to be set")
	}
	if !strings.HasPrefix(result.CommentID, "c-") {
		t.Errorf("expected comment_id to start with 'c-', got %q", result.CommentID)
	}
}

func TestCommentListJSON(t *testing.T) {
	store := setupTestStorage(t)
	issueID := createTestIssue(t, store)
	ctx := context.Background()

	// Add comments
	now := time.Now()
	comments := []storage.Comment{
		{Author: "alice", Body: "First comment", CreatedAt: now.Add(-2 * time.Hour)},
		{Author: "bob", Body: "Second comment", CreatedAt: now.Add(-1 * time.Hour)},
	}
	for i := range comments {
		if err := store.AddComment(ctx, issueID, &comments[i]); err != nil {
			t.Fatalf("AddComment: %v", err)
		}
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := CommentList(ctx, store, CommentListOptions{
		IssueID: issueID,
		JSON:    true,
	})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("CommentList: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result CommentListResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}

	// Default order is newest first
	if result.Comments[0].Author != "bob" {
		t.Errorf("expected first comment by bob (newest), got %q", result.Comments[0].Author)
	}
	if result.Comments[1].Author != "alice" {
		t.Errorf("expected second comment by alice (oldest), got %q", result.Comments[1].Author)
	}
}
