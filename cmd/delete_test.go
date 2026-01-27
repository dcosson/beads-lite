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

func TestDeleteCmd_Force(t *testing.T) {
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

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{id, "--force"})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "deleted "+id) {
		t.Errorf("expected output to contain 'deleted %s', got %q", id, out.String())
	}

	// Verify the issue is gone
	_, err = store.Get(ctx, id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteCmd_PrefixMatch(t *testing.T) {
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

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{prefix, "--force"})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "deleted "+id) {
		t.Errorf("expected output to contain 'deleted %s', got %q", id, out.String())
	}
}

func TestDeleteCmd_ConfirmYes(t *testing.T) {
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
	in := strings.NewReader("y\n")
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
		In:      in,
	}

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{id})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "deleted "+id) {
		t.Errorf("expected output to contain 'deleted %s', got %q", id, out.String())
	}

	// Verify the issue is gone
	_, err = store.Get(ctx, id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteCmd_ConfirmNo(t *testing.T) {
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
	in := strings.NewReader("n\n")
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
		In:      in,
	}

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{id})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "cancelled") {
		t.Errorf("expected output to contain 'cancelled', got %q", out.String())
	}

	// Verify the issue still exists
	_, err = store.Get(ctx, id)
	if err != nil {
		t.Errorf("expected issue to still exist, got error: %v", err)
	}
}

func TestDeleteCmd_JSON(t *testing.T) {
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

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{id, "--force"})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var result DeleteResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ID != id {
		t.Errorf("expected ID %q, got %q", id, result.ID)
	}
	if result.Status != "deleted" {
		t.Errorf("expected status 'deleted', got %q", result.Status)
	}
}

func TestDeleteCmd_NotFound(t *testing.T) {
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

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{"bd-9999", "--force"})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
}

func TestDeleteCmd_ClosedIssue(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create and close an issue
	issue := &storage.Issue{Title: "Test issue"}
	id, err := store.Create(ctx, issue)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(ctx, id); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		Err:     &errOut,
	}

	cmd := NewDeleteCmd(app)
	cmd.SetArgs([]string{id, "--force"})
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "deleted "+id) {
		t.Errorf("expected output to contain 'deleted %s', got %q", id, out.String())
	}

	// Verify the issue is gone
	_, err = store.Get(ctx, id)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
