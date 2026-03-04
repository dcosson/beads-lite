package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"beads-lite/internal/issueservice"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
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
		id, err := rs.Create(ctx, &issueCopy)
		if err != nil {
			t.Fatalf("failed to create issue: %v", err)
		}
		// Update status after create (create sets to open by default)
		if issue.Status != issuestorage.StatusOpen {
			if err := rs.Modify(ctx, id, func(i *issuestorage.Issue) error { i.Status = issue.Status; return nil }); err != nil {
				t.Fatalf("failed to update issue: %v", err)
			}
		}
	}

	// Create and close an issue
	closeID, err := rs.Create(ctx, &issuestorage.Issue{Title: "To be closed"})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := rs.Modify(ctx, closeID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil }); err != nil {
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
	rs.Create(ctx, &issuestorage.Issue{Title: "Issue 1"})
	rs.Create(ctx, &issuestorage.Issue{Title: "Issue 2"})
	id, _ := rs.Create(ctx, &issuestorage.Issue{Title: "Issue 3"})
	rs.Modify(ctx, id, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

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

func TestStatsCmd_IDsIncludesDescendantsRecursively(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, s)

	parentID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic})
	if err != nil {
		t.Fatalf("failed to create parent issue: %v", err)
	}
	childID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Child", Parent: parentID})
	if err != nil {
		t.Fatalf("failed to create child issue: %v", err)
	}
	grandChildID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Grandchild", Parent: childID})
	if err != nil {
		t.Fatalf("failed to create grandchild issue: %v", err)
	}
	if err := rs.Modify(ctx, grandChildID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("failed to close grandchild issue: %v", err)
	}
	if _, err := rs.Create(ctx, &issuestorage.Issue{Title: "Unrelated"}); err != nil {
		t.Fatalf("failed to create unrelated issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{Storage: rs, Out: &out, JSON: true}
	cmd := newStatsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--ids", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	var result StatsResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result.Summary.TotalIssues != 3 {
		t.Fatalf("expected total_issues=3 for parent subtree, got %d", result.Summary.TotalIssues)
	}
	if result.Summary.OpenIssues != 2 {
		t.Fatalf("expected open_issues=2 for parent+child, got %d", result.Summary.OpenIssues)
	}
	if result.Summary.ClosedIssues != 1 {
		t.Fatalf("expected closed_issues=1 for grandchild, got %d", result.Summary.ClosedIssues)
	}
}

func TestStatsCmd_IDsCombinedWithCreatedRangeFiltersExpandedSubtree(t *testing.T) {
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, s)

	parentID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic})
	if err != nil {
		t.Fatalf("failed to create parent issue: %v", err)
	}
	childInRangeID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Child in range", Parent: parentID})
	if err != nil {
		t.Fatalf("failed to create in-range child issue: %v", err)
	}
	childOutOfRangeID, err := rs.Create(ctx, &issuestorage.Issue{Title: "Child out of range", Parent: parentID})
	if err != nil {
		t.Fatalf("failed to create out-of-range child issue: %v", err)
	}

	if err := rs.Modify(ctx, parentID, func(i *issuestorage.Issue) error {
		i.CreatedAt = time.Date(2026, 2, 1, 10, 0, 0, 0, time.Local)
		return nil
	}); err != nil {
		t.Fatalf("failed to set parent created_at: %v", err)
	}
	if err := rs.Modify(ctx, childInRangeID, func(i *issuestorage.Issue) error {
		i.CreatedAt = time.Date(2026, 3, 15, 10, 0, 0, 0, time.Local)
		return nil
	}); err != nil {
		t.Fatalf("failed to set in-range child created_at: %v", err)
	}
	if err := rs.Modify(ctx, childOutOfRangeID, func(i *issuestorage.Issue) error {
		i.CreatedAt = time.Date(2026, 4, 10, 10, 0, 0, 0, time.Local)
		return nil
	}); err != nil {
		t.Fatalf("failed to set out-of-range child created_at: %v", err)
	}

	var out bytes.Buffer
	app := &App{Storage: rs, Out: &out, JSON: true}
	cmd := newStatsCmd(NewTestProvider(app))
	cmd.SetArgs([]string{
		"--ids", parentID,
		"--created-after", "2026-03-01",
		"--created-before", "2026-03-31",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	var result StatsResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result.Summary.TotalIssues != 1 {
		t.Fatalf("expected only 1 in-range subtree issue, got %d", result.Summary.TotalIssues)
	}
	if result.Summary.OpenIssues != 1 {
		t.Fatalf("expected open_issues=1, got %d", result.Summary.OpenIssues)
	}
	if result.Summary.ClosedIssues != 0 {
		t.Fatalf("expected closed_issues=0, got %d", result.Summary.ClosedIssues)
	}
}
