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

func TestShowCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a test issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "Test Issue",
		Description: "This is a test description",
		Priority:    issuestorage.PriorityHigh,
		Type:        issuestorage.TypeBug,
		Labels:      []string{"urgent", "backend"},
		Assignee:    "alice",
		Owner:       "bob",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show with exact ID
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := out.String()

	// Header line: icon + id + · + title + status bracket
	if !strings.Contains(output, "○") {
		t.Errorf("expected status icon ○, got: %s", output)
	}
	if !strings.Contains(output, id+" · Test Issue") {
		t.Errorf("expected 'id · Test Issue' in header, got: %s", output)
	}
	if !strings.Contains(output, "P1") {
		t.Errorf("expected P1 in priority bracket, got: %s", output)
	}
	if !strings.Contains(output, "OPEN") {
		t.Errorf("expected OPEN in status bracket, got: %s", output)
	}

	// Metadata line
	if !strings.Contains(output, "Owner: bob") {
		t.Errorf("expected 'Owner: bob' in metadata, got: %s", output)
	}
	if !strings.Contains(output, "Assignee: alice") {
		t.Errorf("expected 'Assignee: alice' in metadata, got: %s", output)
	}
	if !strings.Contains(output, "Type: bug") {
		t.Errorf("expected 'Type: bug' in metadata, got: %s", output)
	}

	// Dates line
	if !strings.Contains(output, "Created:") || !strings.Contains(output, "Updated:") {
		t.Errorf("expected dates line, got: %s", output)
	}

	// Description section
	if !strings.Contains(output, "Description") {
		t.Errorf("expected Description section, got: %s", output)
	}
	if !strings.Contains(output, "  This is a test description") {
		t.Errorf("expected indented description, got: %s", output)
	}

	// Labels section
	if !strings.Contains(output, "Labels: urgent, backend") {
		t.Errorf("expected labels line, got: %s", output)
	}
}

func TestShowPrefixMatch(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a test issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Prefix Test Issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show with prefix match (first 4 characters of ID)
	prefix := id[:4]
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command with prefix failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, id) {
		t.Errorf("expected output to contain full ID %s, got: %s", id, output)
	}
	if !strings.Contains(output, "Prefix Test Issue") {
		t.Errorf("expected output to contain title, got: %s", output)
	}
}

func TestShowAmbiguousPrefix(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create two issues (both will start with "bd-")
	id1, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Issue One",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue 1: %v", err)
	}

	id2, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Issue Two",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue 2: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show with ambiguous prefix "bd-" which matches both
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for ambiguous prefix, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "ambiguous") {
		t.Errorf("expected 'ambiguous' in error message, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, id1) || !strings.Contains(errMsg, id2) {
		t.Errorf("expected error to list matching IDs, got: %s", errMsg)
	}
}

func TestShowNotFound(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show with non-existent ID
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent issue, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "no issue found") && !strings.Contains(errMsg, "bd-nonexistent") {
		t.Errorf("expected error message to reference the query, got: %s", errMsg)
	}
}

func TestShowJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a test issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:       "JSON Test Issue",
		Description: "Test description",
		Priority:    issuestorage.PriorityLow,
		Type:        issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create app with JSON output
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command JSON failed: %v", err)
	}

	// Verify output is valid JSON array (show returns array to match original beads)
	var results []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 issue in output, got %d", len(results))
	}
	result := results[0]

	if result["id"] != id {
		t.Errorf("expected ID %s, got %v", id, result["id"])
	}
	if result["title"] != "JSON Test Issue" {
		t.Errorf("expected title 'JSON Test Issue', got %v", result["title"])
	}
}

func TestShowClosedIssue(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create and close an issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Closed Issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	if err := store.Modify(ctx, id, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil }); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show closed issue with exact ID
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show closed issue failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, id) {
		t.Errorf("expected output to contain ID %s, got: %s", id, output)
	}
	if !strings.Contains(output, "CLOSED") {
		t.Errorf("expected output to show CLOSED status, got: %s", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("expected closed status icon ✓, got: %s", output)
	}
	// Closed issues should NOT have ● in the priority bracket
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Closed Issue") && strings.Contains(line, "●") {
			t.Errorf("closed issue should not have ● in priority bracket, got: %s", line)
		}
	}
}

func TestShowWithDependencies(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issues with dependencies
	depID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Dependency Issue",
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create dependency issue: %v", err)
	}

	mainID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Main Issue",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create main issue: %v", err)
	}

	// Add dependency relationship (mainID depends on depID)
	if err := store.AddDependency(ctx, mainID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show main issue — should have "Depends On" with enriched dep line
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{mainID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Depends On") {
		t.Errorf("expected 'Depends On' section, got: %s", output)
	}
	if !strings.Contains(output, "→") {
		t.Errorf("expected → prefix in depends on section, got: %s", output)
	}
	if !strings.Contains(output, depID) {
		t.Errorf("expected dependency ID %s, got: %s", depID, output)
	}
	if !strings.Contains(output, "Dependency Issue") {
		t.Errorf("expected dependency title in output, got: %s", output)
	}

	// Test show dependency issue — should have "Blocks" section
	out.Reset()
	cmd = newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{depID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show dep issue failed: %v", err)
	}

	output = out.String()
	if !strings.Contains(output, "Blocks") {
		t.Errorf("expected 'Blocks' section, got: %s", output)
	}
	if !strings.Contains(output, "←") {
		t.Errorf("expected ← prefix in blocks section, got: %s", output)
	}
	if !strings.Contains(output, mainID) {
		t.Errorf("expected blocker ID %s, got: %s", mainID, output)
	}
}

func TestShowWithChildren(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create parent epic
	parentID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Parent Epic",
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child task
	childID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Child Task",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Add parent-child relationship
	if err := store.AddDependency(ctx, childID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Show parent — should have Children section, NOT Blocks
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show parent failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "[EPIC]") {
		t.Errorf("expected [EPIC] tag in header, got: %s", output)
	}
	if !strings.Contains(output, "Children") {
		t.Errorf("expected Children section, got: %s", output)
	}
	if !strings.Contains(output, "↳") {
		t.Errorf("expected ↳ prefix in children section, got: %s", output)
	}
	if !strings.Contains(output, childID) {
		t.Errorf("expected child ID %s in children section, got: %s", childID, output)
	}
	if !strings.Contains(output, "Child Task") {
		t.Errorf("expected child title in children section, got: %s", output)
	}
	// Parent-child deps should NOT appear in Blocks
	if strings.Contains(output, "Blocks") {
		t.Errorf("parent-child dep should not appear in Blocks section, got: %s", output)
	}

	// Show child — should have Parent section, NOT Depends On
	out.Reset()
	cmd = newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{childID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show child failed: %v", err)
	}

	output = out.String()
	if !strings.Contains(output, "Parent") {
		t.Errorf("expected Parent section, got: %s", output)
	}
	if !strings.Contains(output, parentID) {
		t.Errorf("expected parent ID %s, got: %s", parentID, output)
	}
	// Parent-child deps should NOT appear in Depends On
	if strings.Contains(output, "Depends On") {
		t.Errorf("parent-child dep should not appear in Depends On section, got: %s", output)
	}
}

func TestShowWithComments(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an issue
	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Issue with Comments",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Add a comment
	if err := store.AddComment(ctx, id, &issuestorage.Comment{
		Author: "bob",
		Text:   "This is a test comment",
	}); err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show issue
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Comments (1)") {
		t.Errorf("expected 'Comments (1)' section, got: %s", output)
	}
	if !strings.Contains(output, "bob") {
		t.Errorf("expected comment author, got: %s", output)
	}
	if !strings.Contains(output, "This is a test comment") {
		t.Errorf("expected comment body, got: %s", output)
	}
}
