package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"beads-lite/internal/issuestorage"
)

func TestCompactDryRun(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create and close some issues
	issue1 := &issuestorage.Issue{Title: "Closed issue 1", Type: issuestorage.TypeTask}
	id1, _ := store.Create(context.Background(), issue1)
	store.Modify(context.Background(), id1, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	issue2 := &issuestorage.Issue{Title: "Closed issue 2", Type: issuestorage.TypeTask}
	id2, _ := store.Create(context.Background(), issue2)
	store.Modify(context.Background(), id2, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	// Keep one open
	issue3 := &issuestorage.Issue{Title: "Open issue", Type: issuestorage.TypeTask}
	store.Create(context.Background(), issue3)

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Would delete 2") {
		t.Errorf("expected 'Would delete 2' in output, got: %s", output)
	}
	if !strings.Contains(output, id1) {
		t.Errorf("expected id1 %s in output, got: %s", id1, output)
	}
	if !strings.Contains(output, id2) {
		t.Errorf("expected id2 %s in output, got: %s", id2, output)
	}

	// Verify issues still exist (dry run shouldn't delete)
	_, err := store.Get(context.Background(), id1)
	if err != nil {
		t.Errorf("issue 1 should still exist after dry run")
	}
	_, err = store.Get(context.Background(), id2)
	if err != nil {
		t.Errorf("issue 2 should still exist after dry run")
	}
}

func TestCompactWithForce(t *testing.T) {
	app, store := setupTestApp(t)

	// Create and close an issue
	issue := &issuestorage.Issue{Title: "Closed issue", Type: issuestorage.TypeTask}
	id, _ := store.Create(context.Background(), issue)
	store.Modify(context.Background(), id, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	// Verify issue is deleted
	_, err := store.Get(context.Background(), id)
	if err != issuestorage.ErrNotFound {
		t.Errorf("expected ErrNotFound after compact, got %v", err)
	}
}

func TestCompactNoClosedIssues(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create only open issues
	issue := &issuestorage.Issue{Title: "Open issue", Type: issuestorage.TypeTask}
	store.Create(context.Background(), issue)

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	if !strings.Contains(out.String(), "No closed issues match") {
		t.Errorf("expected 'No closed issues match' in output, got: %s", out.String())
	}
}

func TestCompactOlderThan(t *testing.T) {
	app, store := setupTestApp(t)
	ctx := context.Background()

	// Create and close an issue, then manually set its ClosedAt to be old
	issue1 := &issuestorage.Issue{Title: "Old closed issue", Type: issuestorage.TypeTask}
	id1, _ := store.Create(ctx, issue1)
	store.Modify(ctx, id1, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	// Update ClosedAt to 60 days ago
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	store.Modify(ctx, id1, func(i *issuestorage.Issue) error { i.ClosedAt = &oldTime; return nil })

	// Create a recently closed issue
	issue2 := &issuestorage.Issue{Title: "Recent closed issue", Type: issuestorage.TypeTask}
	id2, _ := store.Create(ctx, issue2)
	store.Modify(ctx, id2, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--older-than", "30d", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	// Old issue should be deleted
	_, err := store.Get(ctx, id1)
	if err != issuestorage.ErrNotFound {
		t.Errorf("old issue should be deleted, got %v", err)
	}

	// Recent issue should still exist
	_, err = store.Get(ctx, id2)
	if err != nil {
		t.Errorf("recent issue should still exist, got %v", err)
	}
}

func TestCompactBefore(t *testing.T) {
	app, store := setupTestApp(t)
	ctx := context.Background()

	// Create and close an issue with old ClosedAt
	issue1 := &issuestorage.Issue{Title: "Old closed issue", Type: issuestorage.TypeTask}
	id1, _ := store.Create(ctx, issue1)
	store.Modify(ctx, id1, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	oldTime := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	store.Modify(ctx, id1, func(i *issuestorage.Issue) error { i.ClosedAt = &oldTime; return nil })

	// Create a recently closed issue
	issue2 := &issuestorage.Issue{Title: "Recent closed issue", Type: issuestorage.TypeTask}
	id2, _ := store.Create(ctx, issue2)
	store.Modify(ctx, id2, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--before", "2024-01-01", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	// Old issue should be deleted
	_, err := store.Get(ctx, id1)
	if err != issuestorage.ErrNotFound {
		t.Errorf("old issue should be deleted, got %v", err)
	}

	// Recent issue should still exist
	_, err = store.Get(ctx, id2)
	if err != nil {
		t.Errorf("recent issue should still exist, got %v", err)
	}
}

func TestCompactJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create and close an issue
	issue := &issuestorage.Issue{Title: "Closed issue", Type: issuestorage.TypeTask}
	id, _ := store.Create(context.Background(), issue)
	store.Modify(context.Background(), id, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["count"].(float64) != 1 {
		t.Errorf("expected count 1, got %v", result["count"])
	}

	deleted := result["deleted"].([]interface{})
	if len(deleted) != 1 || deleted[0].(string) != id {
		t.Errorf("expected deleted to contain %s, got %v", id, deleted)
	}
}

func TestCompactDryRunJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create and close an issue
	issue := &issuestorage.Issue{Title: "Closed issue", Type: issuestorage.TypeTask}
	id, _ := store.Create(context.Background(), issue)
	store.Modify(context.Background(), id, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["count"].(float64) != 1 {
		t.Errorf("expected count 1, got %v", result["count"])
	}

	wouldDelete := result["would_delete"].([]interface{})
	if len(wouldDelete) != 1 || wouldDelete[0].(string) != id {
		t.Errorf("expected would_delete to contain %s, got %v", id, wouldDelete)
	}

	// Verify issue still exists
	_, err := store.Get(context.Background(), id)
	if err != nil {
		t.Errorf("issue should still exist after dry run")
	}
}

func TestCompactInvalidBeforeDate(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--before", "invalid-date"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid date")
	}
	if !strings.Contains(err.Error(), "invalid --before date") {
		t.Errorf("expected 'invalid --before date' in error, got: %v", err)
	}
}

func TestCompactInvalidOlderThan(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--older-than", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "invalid --older-than duration") {
		t.Errorf("expected 'invalid --older-than duration' in error, got: %v", err)
	}
}

func TestCompactBothFiltersError(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--before", "2024-01-01", "--older-than", "30d"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when both filters specified")
	}
	if !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("expected 'cannot specify both' in error, got: %v", err)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"6m", 180 * 24 * time.Hour, false},
		{"1y", 365 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"", 0, true},
		{"d", 0, true},
		{"abc", 0, true},
		{"-5d", 0, true},
		{"0d", 0, true},
		{"30x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompactPreservesOpenIssues(t *testing.T) {
	app, store := setupTestApp(t)
	ctx := context.Background()

	// Create open issues
	openIssue := &issuestorage.Issue{Title: "Open issue", Type: issuestorage.TypeTask}
	openID, _ := store.Create(ctx, openIssue)

	// Create closed issue
	closedIssue := &issuestorage.Issue{Title: "Closed issue", Type: issuestorage.TypeTask}
	closedID, _ := store.Create(ctx, closedIssue)
	store.Modify(ctx, closedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	cmd := newCompactCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	// Open issue should still exist
	_, err := store.Get(ctx, openID)
	if err != nil {
		t.Errorf("open issue should still exist, got %v", err)
	}

	// Closed issue should be deleted
	_, err = store.Get(ctx, closedID)
	if err != issuestorage.ErrNotFound {
		t.Errorf("closed issue should be deleted, got %v", err)
	}
}
