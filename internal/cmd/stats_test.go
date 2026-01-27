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

func TestStatsCmd_Empty(t *testing.T) {
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
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issues with various statuses
	issues := []storage.Issue{
		{Title: "Open 1", Status: storage.StatusOpen},
		{Title: "Open 2", Status: storage.StatusOpen},
		{Title: "In Progress", Status: storage.StatusInProgress},
		{Title: "Blocked", Status: storage.StatusBlocked},
		{Title: "Deferred", Status: storage.StatusDeferred},
	}

	for _, issue := range issues {
		issueCopy := issue
		id, err := s.Create(ctx, &issueCopy)
		if err != nil {
			t.Fatalf("failed to create issue: %v", err)
		}
		// Update status after create (create sets to open by default)
		if issue.Status != storage.StatusOpen {
			issueCopy.ID = id
			if err := s.Update(ctx, &issueCopy); err != nil {
				t.Fatalf("failed to update issue: %v", err)
			}
		}
	}

	// Create and close an issue
	closeID, err := s.Create(ctx, &storage.Issue{Title: "To be closed"})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := s.Close(ctx, closeID); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: s,
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
	s := filesystem.New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a few issues
	s.Create(ctx, &storage.Issue{Title: "Issue 1"})
	s.Create(ctx, &storage.Issue{Title: "Issue 2"})
	id, _ := s.Create(ctx, &storage.Issue{Title: "Issue 3"})
	s.Close(ctx, id)

	var out bytes.Buffer
	app := &App{
		Storage: s,
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

	if result.Open != 2 {
		t.Errorf("expected open=2, got %d", result.Open)
	}
	if result.Closed != 1 {
		t.Errorf("expected closed=1, got %d", result.Closed)
	}
	if result.Total != 3 {
		t.Errorf("expected total=3, got %d", result.Total)
	}
}
