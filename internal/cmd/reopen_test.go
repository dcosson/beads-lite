package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/routing"
)

func TestReopenCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := routing.NewIssueStore(nil, store)

	// Create and close an issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Issue to reopen",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	if err := store.Modify(ctx, id, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Verify it's closed
	issue, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get closed issue: %v", err)
	}
	if issue.Status != issuestorage.StatusClosed {
		t.Fatalf("expected status closed, got: %s", issue.Status)
	}
	if issue.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	// Run reopen command
	cmd := newReopenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reopen command failed: %v", err)
	}

	// Verify output
	output := out.String()
	if !bytes.Contains([]byte(output), []byte("Reopened")) {
		t.Errorf("expected output to contain 'Reopened', got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte(id)) {
		t.Errorf("expected output to contain issue ID %s, got: %s", id, output)
	}

	// Verify the issue is now open
	issue, err = store.Get(ctx, id)
	if err != nil {
		t.Fatalf("failed to get reopened issue: %v", err)
	}
	if issue.Status != issuestorage.StatusOpen {
		t.Errorf("expected status open, got: %s", issue.Status)
	}
	if issue.ClosedAt != nil {
		t.Errorf("expected closed_at to be nil, got: %v", issue.ClosedAt)
	}
}

func TestReopenNonExistent(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := routing.NewIssueStore(nil, store)

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	// Try to reopen non-existent issue
	cmd := newReopenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"non-existent-id"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when reopening non-existent issue")
	}
}

func TestReopenJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := routing.NewIssueStore(nil, store)

	// Create and close an issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Issue to reopen",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	if err := store.Modify(ctx, id, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Create app with JSON output
	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    true,
	}

	// Run reopen command
	cmd := newReopenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reopen command JSON failed: %v", err)
	}

	// Parse and verify JSON output
	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result))
	}
	if result[0]["id"].(string) != id {
		t.Errorf("expected id %s, got: %s", id, result[0]["id"])
	}
	if result[0]["status"].(string) != "open" {
		t.Errorf("expected status 'open', got: %s", result[0]["status"])
	}
}
