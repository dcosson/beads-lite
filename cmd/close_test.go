package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

func TestCloseCmd_SingleIssue(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create a test issue
	issue := &storage.Issue{Title: "Test issue"}
	id, err := store.Create(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
	}

	cmd := NewCloseCmd(app)
	cmd.SetArgs([]string{id})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "closed "+id) {
		t.Errorf("expected output to contain 'closed %s', got %q", id, out.String())
	}

	// Verify the issue is now closed
	closed, err := store.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != storage.StatusClosed {
		t.Errorf("expected status %q, got %q", storage.StatusClosed, closed.Status)
	}
}

func TestCloseCmd_MultipleIssues(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create test issues
	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		issue := &storage.Issue{Title: "Test issue"}
		id, err := store.Create(ctx, issue)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}

	var out, errOut bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
	}

	cmd := NewCloseCmd(app)
	cmd.SetArgs(ids)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify all issues are closed
	for _, id := range ids {
		if !strings.Contains(out.String(), "closed "+id) {
			t.Errorf("expected output to contain 'closed %s', got %q", id, out.String())
		}

		closed, err := store.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if closed.Status != storage.StatusClosed {
			t.Errorf("expected status %q, got %q", storage.StatusClosed, closed.Status)
		}
	}
}

func TestCloseCmd_PrefixMatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create a test issue
	issue := &storage.Issue{Title: "Test issue"}
	id, err := store.Create(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}

	// Use a prefix of the ID
	prefix := id[:len(id)-1]

	var out, errOut bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
	}

	cmd := NewCloseCmd(app)
	cmd.SetArgs([]string{prefix})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "closed "+id) {
		t.Errorf("expected output to contain 'closed %s', got %q", id, out.String())
	}
}

func TestCloseCmd_JSON(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create a test issue
	issue := &storage.Issue{Title: "Test issue"}
	id, err := store.Create(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
		JSON:    true,
	}

	cmd := NewCloseCmd(app)
	cmd.SetArgs([]string{id})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var results []CloseResult
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ID != id {
		t.Errorf("expected ID %q, got %q", id, results[0].ID)
	}
	if results[0].Status != "closed" {
		t.Errorf("expected status 'closed', got %q", results[0].Status)
	}
}

func TestCloseCmd_NotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
	}

	cmd := NewCloseCmd(app)
	cmd.SetArgs([]string{"bd-9999"})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	// Should not return an error, but print error to stderr
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(errOut.String(), "error") {
		t.Errorf("expected error output, got %q", errOut.String())
	}
}
