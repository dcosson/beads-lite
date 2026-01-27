package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

func TestResolveID_ExactMatch(t *testing.T) {
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

	app := &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}

	resolved, err := app.ResolveID(ctx, id)
	if err != nil {
		t.Fatalf("ResolveID() error = %v", err)
	}

	if resolved != id {
		t.Errorf("expected %q, got %q", id, resolved)
	}
}

func TestResolveID_PrefixMatch(t *testing.T) {
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

	app := &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}

	// Use a prefix (just "bd-" plus first char)
	prefix := id[:4]
	resolved, err := app.ResolveID(ctx, prefix)
	if err != nil {
		t.Fatalf("ResolveID() error = %v", err)
	}

	if resolved != id {
		t.Errorf("expected %q, got %q", id, resolved)
	}
}

func TestResolveID_NotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	app := &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}

	_, err := app.ResolveID(ctx, "bd-9999")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveID_Ambiguous(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Create multiple issues that could match a short prefix
	// Since IDs are random, we'll create several and hope at least two start with "bd-"
	// which they all do. We use a very short prefix to ensure ambiguity.
	for i := 0; i < 10; i++ {
		issue := &storage.Issue{Title: "Test issue"}
		_, err := store.Create(ctx, issue)
		if err != nil {
			t.Fatal(err)
		}
	}

	app := &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}

	// "bd-" should match all issues
	_, err := app.ResolveID(ctx, "bd-")
	if err == nil {
		t.Error("expected error for ambiguous prefix")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error message, got %v", err)
	}
}

func TestResolveID_ClosedIssue(t *testing.T) {
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

	app := &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}

	// Should still be able to resolve closed issues
	resolved, err := app.ResolveID(ctx, id)
	if err != nil {
		t.Fatalf("ResolveID() error = %v", err)
	}

	if resolved != id {
		t.Errorf("expected %q, got %q", id, resolved)
	}
}

func TestConfirm_Yes(t *testing.T) {
	in := strings.NewReader("y\n")
	app := &App{
		Out: &bytes.Buffer{},
		In:  in,
	}

	if !app.Confirm("Test?") {
		t.Error("expected Confirm to return true for 'y'")
	}
}

func TestConfirm_Yes_Full(t *testing.T) {
	in := strings.NewReader("yes\n")
	app := &App{
		Out: &bytes.Buffer{},
		In:  in,
	}

	if !app.Confirm("Test?") {
		t.Error("expected Confirm to return true for 'yes'")
	}
}

func TestConfirm_No(t *testing.T) {
	in := strings.NewReader("n\n")
	app := &App{
		Out: &bytes.Buffer{},
		In:  in,
	}

	if app.Confirm("Test?") {
		t.Error("expected Confirm to return false for 'n'")
	}
}

func TestConfirm_Empty(t *testing.T) {
	in := strings.NewReader("\n")
	app := &App{
		Out: &bytes.Buffer{},
		In:  in,
	}

	if app.Confirm("Test?") {
		t.Error("expected Confirm to return false for empty input")
	}
}
