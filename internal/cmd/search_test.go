package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/internal/storage/filesystem"
	"beads2/internal/storage"
)

func TestSearchCmd_NoArgs(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing query argument")
	}
}

func TestSearchCmd_NoMatches(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{Title: "First issue"})
	s.Create(ctx, &storage.Issue{Title: "Second issue"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"nonexistent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	if !strings.Contains(out.String(), "No matches found") {
		t.Errorf("expected 'No matches found', got: %s", out.String())
	}
}

func TestSearchCmd_MatchTitle(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{Title: "Fix authentication bug"})
	s.Create(ctx, &storage.Issue{Title: "Add login feature"})
	s.Create(ctx, &storage.Issue{Title: "Update docs"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"authentication"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Found 1 matches") {
		t.Errorf("expected 'Found 1 matches', got: %s", output)
	}
	if !strings.Contains(output, "Fix authentication bug") {
		t.Errorf("expected to find 'Fix authentication bug', got: %s", output)
	}
}

func TestSearchCmd_MatchDescription(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{
		Title:       "Generic issue",
		Description: "This issue involves authentication changes",
	})
	s.Create(ctx, &storage.Issue{Title: "Other issue"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"authentication"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Found 1 matches") {
		t.Errorf("expected 'Found 1 matches', got: %s", output)
	}
	if !strings.Contains(output, "Generic issue") {
		t.Errorf("expected to find 'Generic issue', got: %s", output)
	}
}

func TestSearchCmd_TitleOnly(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{
		Title:       "Generic issue",
		Description: "This issue involves authentication changes",
	})
	s.Create(ctx, &storage.Issue{Title: "Authentication fix"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"authentication", "--title-only"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Found 1 matches") {
		t.Errorf("expected 'Found 1 matches', got: %s", output)
	}
	if !strings.Contains(output, "Authentication fix") {
		t.Errorf("expected to find 'Authentication fix', got: %s", output)
	}
	// Should NOT find the one with authentication only in description
	if strings.Contains(output, "Generic issue") {
		t.Errorf("should not find 'Generic issue' with --title-only, got: %s", output)
	}
}

func TestSearchCmd_IncludeClosed(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{Title: "Open auth issue"})
	closedID, _ := s.Create(ctx, &storage.Issue{Title: "Closed auth issue"})
	s.Close(ctx, closedID)

	// First, search without --all (should not find closed)
	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Found 1 matches") {
		t.Errorf("expected 'Found 1 matches', got: %s", output)
	}
	if strings.Contains(output, "Closed auth issue") {
		t.Errorf("should not find closed issue without --all, got: %s", output)
	}

	// Now search with --all
	out.Reset()
	cmd = NewSearchCmd(app)
	cmd.SetArgs([]string{"auth", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output = out.String()
	if !strings.Contains(output, "Found 2 matches") {
		t.Errorf("expected 'Found 2 matches', got: %s", output)
	}
	if !strings.Contains(output, "Closed auth issue") {
		t.Errorf("expected to find 'Closed auth issue' with --all, got: %s", output)
	}
}

func TestSearchCmd_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{Title: "Fix AUTHENTICATION bug"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"authentication"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	if !strings.Contains(out.String(), "Found 1 matches") {
		t.Errorf("expected case-insensitive match, got: %s", out.String())
	}
}

func TestSearchCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &storage.Issue{Title: "Auth issue 1"})
	s.Create(ctx, &storage.Issue{Title: "Auth issue 2"})
	s.Create(ctx, &storage.Issue{Title: "Other issue"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
		JSON:    true,
	}

	cmd := NewSearchCmd(app)
	cmd.SetArgs([]string{"auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	var results []SearchResult
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
