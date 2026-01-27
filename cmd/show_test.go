package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"beads2/filesystem"
	"beads2/storage"
)

func setupTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	fs := filesystem.New(dir)
	if err := fs.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	return &App{
		Storage: fs,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}
}

func TestShowCmd_ExactMatch(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create a test issue
	issue := &storage.Issue{
		Title:       "Test Issue",
		Description: "A test description",
		Priority:    storage.PriorityHigh,
		Type:        storage.TypeTask,
	}
	id, err := app.Storage.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Test exact match
	cmd := NewShowCmd(app)
	cmd.SetArgs([]string{id})
	cmd.SetOut(app.Out.(*bytes.Buffer))

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := app.Out.(*bytes.Buffer).String()
	if !strings.Contains(output, "Test Issue") {
		t.Errorf("output should contain title, got: %s", output)
	}
	if !strings.Contains(output, id) {
		t.Errorf("output should contain ID, got: %s", output)
	}
}

func TestShowCmd_PrefixMatch(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create a test issue
	issue := &storage.Issue{
		Title:    "Prefix Test Issue",
		Priority: storage.PriorityMedium,
		Type:     storage.TypeBug,
	}
	id, err := app.Storage.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Test prefix match (use first 4 chars of ID)
	prefix := id[:4]
	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{prefix})
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("show command with prefix failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Prefix Test Issue") {
		t.Errorf("output should contain title, got: %s", output)
	}
}

func TestShowCmd_JSONOutput(t *testing.T) {
	app := setupTestApp(t)
	app.JSON = true
	ctx := context.Background()

	// Create a test issue with all fields
	issue := &storage.Issue{
		Title:       "JSON Test Issue",
		Description: "Testing JSON output",
		Priority:    storage.PriorityCritical,
		Type:        storage.TypeFeature,
		Labels:      []string{"backend", "urgent"},
		Assignee:    "alice",
	}
	id, err := app.Storage.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{id})
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	// Parse JSON output
	var result storage.Issue
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Title != "JSON Test Issue" {
		t.Errorf("expected title 'JSON Test Issue', got '%s'", result.Title)
	}
	if result.Priority != storage.PriorityCritical {
		t.Errorf("expected priority critical, got %s", result.Priority)
	}
	if len(result.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(result.Labels))
	}
}

func TestShowCmd_NotFound(t *testing.T) {
	app := setupTestApp(t)

	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{"bd-nonexistent"})
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent issue")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestShowCmd_AmbiguousPrefix(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create multiple issues - they all start with "bd-"
	for i := 0; i < 5; i++ {
		issue := &storage.Issue{
			Title:    "Ambiguous Test",
			Priority: storage.PriorityLow,
			Type:     storage.TypeTask,
		}
		_, err := app.Storage.Create(ctx, issue)
		if err != nil {
			t.Fatalf("failed to create issue: %v", err)
		}
	}

	// Try to match with just "bd-" prefix (should be ambiguous)
	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{"bd-"})
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for ambiguous prefix")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' error, got: %v", err)
	}
}

func TestShowCmd_ClosedIssue(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create and close an issue
	issue := &storage.Issue{
		Title:    "Closed Issue Test",
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := app.Storage.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	if err := app.Storage.Close(ctx, id); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Should still be able to show closed issues
	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{id})
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("show command failed for closed issue: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Closed Issue Test") {
		t.Errorf("output should contain title, got: %s", output)
	}
	if !strings.Contains(output, "closed") {
		t.Errorf("output should indicate closed status, got: %s", output)
	}
}

func TestShowCmd_WithRelationships(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create parent issue
	parent := &storage.Issue{
		Title:    "Parent Issue",
		Priority: storage.PriorityHigh,
		Type:     storage.TypeEpic,
	}
	parentID, err := app.Storage.Create(ctx, parent)
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child issue
	child := &storage.Issue{
		Title:    "Child Issue",
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	childID, err := app.Storage.Create(ctx, child)
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Set parent relationship
	if err := app.Storage.SetParent(ctx, childID, parentID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create dependency
	dep := &storage.Issue{
		Title:    "Dependency Issue",
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	depID, err := app.Storage.Create(ctx, dep)
	if err != nil {
		t.Fatalf("failed to create dependency: %v", err)
	}

	if err := app.Storage.AddDependency(ctx, childID, depID); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Show child issue (should show parent, depends_on)
	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{childID})
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "PARENT") {
		t.Errorf("output should show PARENT section, got: %s", output)
	}
	if !strings.Contains(output, parentID) {
		t.Errorf("output should contain parent ID, got: %s", output)
	}
	if !strings.Contains(output, "DEPENDS ON") {
		t.Errorf("output should show DEPENDS ON section, got: %s", output)
	}
	if !strings.Contains(output, depID) {
		t.Errorf("output should contain dependency ID, got: %s", output)
	}
}

func TestShowCmd_WithComments(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create issue
	issue := &storage.Issue{
		Title:    "Issue with Comments",
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := app.Storage.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Add comment
	comment := &storage.Comment{
		Author:    "bob",
		Body:      "This is a test comment\nWith multiple lines",
		CreatedAt: time.Now(),
	}
	if err := app.Storage.AddComment(ctx, id, comment); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	cmd := NewShowCmd(app)
	buf := &bytes.Buffer{}
	app.Out = buf
	cmd.SetArgs([]string{id})
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "COMMENTS") {
		t.Errorf("output should show COMMENTS section, got: %s", output)
	}
	if !strings.Contains(output, "bob") {
		t.Errorf("output should contain comment author, got: %s", output)
	}
	if !strings.Contains(output, "test comment") {
		t.Errorf("output should contain comment body, got: %s", output)
	}
}

func TestResolveID(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// Create test issue
	issue := &storage.Issue{
		Title:    "Resolve Test",
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := app.Storage.Create(ctx, issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Test exact match
	resolved, err := app.ResolveID(ctx, id)
	if err != nil {
		t.Fatalf("ResolveID exact match failed: %v", err)
	}
	if resolved.ID != id {
		t.Errorf("expected ID %s, got %s", id, resolved.ID)
	}

	// Test prefix match
	prefix := id[:5]
	resolved, err = app.ResolveID(ctx, prefix)
	if err != nil {
		t.Fatalf("ResolveID prefix match failed: %v", err)
	}
	if resolved.ID != id {
		t.Errorf("expected ID %s, got %s", id, resolved.ID)
	}

	// Test not found
	_, err = app.ResolveID(ctx, "bd-xxxx")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
