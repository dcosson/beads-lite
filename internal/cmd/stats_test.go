package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/issueservice"
)

func TestStatsCmd_Empty(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, s)

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
	}

	cmd := newStatsCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Open issues:     0") {
		t.Errorf("expected 'Open issues:     0', got: %s", output)
	}
	if !strings.Contains(output, "Closed issues:   0") {
		t.Errorf("expected 'Closed issues:   0', got: %s", output)
	}
	if !strings.Contains(output, "Total:           0") {
		t.Errorf("expected 'Total:           0', got: %s", output)
	}
}

func TestStatsCmd_WithIssues(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, s)

	// Create issues with various statuses
	issues := []issuestorage.Issue{
		{Title: "Open 1", Status: issuestorage.StatusOpen},
		{Title: "Open 2", Status: issuestorage.StatusOpen},
		{Title: "In Progress", Status: issuestorage.StatusInProgress},
		{Title: "Blocked", Status: issuestorage.StatusBlocked},
		{Title: "Deferred", Status: issuestorage.StatusDeferred},
	}

	for _, issue := range issues {
		issueCopy := issue
		id, err := s.Create(ctx, &issueCopy)
		if err != nil {
			t.Fatalf("failed to create issue: %v", err)
		}
		// Update status after create (create sets to open by default)
		if issue.Status != issuestorage.StatusOpen {
			if err := s.Modify(ctx, id, func(i *issuestorage.Issue) error { i.Status = issue.Status; return nil }); err != nil {
				t.Fatalf("failed to update issue: %v", err)
			}
		}
	}

	// Create and close an issue
	closeID, err := s.Create(ctx, &issuestorage.Issue{Title: "To be closed"})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := s.Modify(ctx, closeID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil }); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
	}

	cmd := newStatsCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Open issues:     5") {
		t.Errorf("expected 'Open issues:     5', got: %s", output)
	}
	if !strings.Contains(output, "In progress:   1") {
		t.Errorf("expected 'In progress:   1', got: %s", output)
	}
	if !strings.Contains(output, "Blocked:       1") {
		t.Errorf("expected 'Blocked:       1', got: %s", output)
	}
	if !strings.Contains(output, "Deferred:      1") {
		t.Errorf("expected 'Deferred:      1', got: %s", output)
	}
	if !strings.Contains(output, "Closed issues:   1") {
		t.Errorf("expected 'Closed issues:   1', got: %s", output)
	}
	if !strings.Contains(output, "Total:           6") {
		t.Errorf("expected 'Total:           6', got: %s", output)
	}
}

func TestStatsCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, s)

	// Create a few issues
	s.Create(ctx, &issuestorage.Issue{Title: "Issue 1"})
	s.Create(ctx, &issuestorage.Issue{Title: "Issue 2"})
	id, _ := s.Create(ctx, &issuestorage.Issue{Title: "Issue 3"})
	s.Modify(ctx, id, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    true,
	}

	cmd := newStatsCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	var result StatsResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Summary.OpenIssues != 2 {
		t.Errorf("expected open_issues=2, got %d", result.Summary.OpenIssues)
	}
	if result.Summary.ClosedIssues != 1 {
		t.Errorf("expected closed_issues=1, got %d", result.Summary.ClosedIssues)
	}
	if result.Summary.TotalIssues != 3 {
		t.Errorf("expected total_issues=3, got %d", result.Summary.TotalIssues)
	}
}
