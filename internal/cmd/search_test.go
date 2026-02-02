package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func TestSearchCmd_NoArgs(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing query argument")
	}
}

func TestSearchCmd_NoMatches(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{Title: "First issue"})
	s.Create(ctx, &issuestorage.Issue{Title: "Second issue"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
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
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{Title: "Fix authentication bug"})
	s.Create(ctx, &issuestorage.Issue{Title: "Add login feature"})
	s.Create(ctx, &issuestorage.Issue{Title: "Update docs"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
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
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{
		Title:       "Generic issue",
		Description: "This issue involves authentication changes",
	})
	s.Create(ctx, &issuestorage.Issue{Title: "Other issue"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
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
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{
		Title:       "Generic issue",
		Description: "This issue involves authentication changes",
	})
	s.Create(ctx, &issuestorage.Issue{Title: "Authentication fix"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
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

func TestSearchCmd_StatusFilter(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{Title: "Open auth issue"})
	closedID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Closed auth issue"})
	s.Modify(ctx, closedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	// Default search includes closed issues.
	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Found 2 matches") {
		t.Errorf("expected 'Found 2 matches', got: %s", output)
	}
	if !strings.Contains(output, "Closed auth issue") {
		t.Errorf("expected to find closed issue by default, got: %s", output)
	}

	// Now search with --status open
	out.Reset()
	cmd = newSearchCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"auth", "--status", "open"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output = out.String()
	if !strings.Contains(output, "Found 1 matches") {
		t.Errorf("expected 'Found 1 matches', got: %s", output)
	}
	if strings.Contains(output, "Closed auth issue") {
		t.Errorf("should not find closed issue with --status open, got: %s", output)
	}

	// Search with --status closed
	out.Reset()
	cmd = newSearchCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"auth", "--status", "closed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output = out.String()
	if !strings.Contains(output, "Found 1 matches") {
		t.Errorf("expected 'Found 1 matches', got: %s", output)
	}
	if !strings.Contains(output, "Closed auth issue") {
		t.Errorf("expected to find 'Closed auth issue' with --status closed, got: %s", output)
	}
}

func TestSearchCmd_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{Title: "Fix AUTHENTICATION bug"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
	}

	cmd := newSearchCmd(NewTestProvider(app))
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
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	s.Create(ctx, &issuestorage.Issue{Title: "Auth issue 1"})
	s.Create(ctx, &issuestorage.Issue{Title: "Auth issue 2"})
	s.Create(ctx, &issuestorage.Issue{Title: "Other issue"})

	var out bytes.Buffer
	app := &App{
		Storage: s,
		Out:     &out,
		JSON:    true,
	}

	cmd := newSearchCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	var results []IssueListJSON
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
