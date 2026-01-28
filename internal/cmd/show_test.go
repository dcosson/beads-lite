package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/internal/storage"
	"beads2/internal/storage/filesystem"
)

func TestShowCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a test issue
	id, err := store.Create(ctx, &storage.Issue{
		Title:       "Test Issue",
		Description: "This is a test description",
		Priority:    storage.PriorityHigh,
		Type:        storage.TypeBug,
		Labels:      []string{"urgent", "backend"},
		Assignee:    "alice",
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
	if !strings.Contains(output, id) {
		t.Errorf("expected output to contain ID %s, got: %s", id, output)
	}
	if !strings.Contains(output, "Test Issue") {
		t.Errorf("expected output to contain title, got: %s", output)
	}
	if !strings.Contains(output, "This is a test description") {
		t.Errorf("expected output to contain description, got: %s", output)
	}
	if !strings.Contains(output, "high") {
		t.Errorf("expected output to contain priority, got: %s", output)
	}
	if !strings.Contains(output, "bug") {
		t.Errorf("expected output to contain type, got: %s", output)
	}
	if !strings.Contains(output, "alice") {
		t.Errorf("expected output to contain assignee, got: %s", output)
	}
	if !strings.Contains(output, "urgent") || !strings.Contains(output, "backend") {
		t.Errorf("expected output to contain labels, got: %s", output)
	}
}

func TestShowPrefixMatch(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a test issue
	id, err := store.Create(ctx, &storage.Issue{
		Title:    "Prefix Test Issue",
		Priority: storage.PriorityMedium,
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
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create two issues (both will start with "bd-")
	id1, err := store.Create(ctx, &storage.Issue{
		Title:    "Issue One",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue 1: %v", err)
	}

	id2, err := store.Create(ctx, &storage.Issue{
		Title:    "Issue Two",
		Priority: storage.PriorityMedium,
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
	store := filesystem.New(dir)
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
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a test issue
	id, err := store.Create(ctx, &storage.Issue{
		Title:       "JSON Test Issue",
		Description: "Test description",
		Priority:    storage.PriorityLow,
		Type:        storage.TypeTask,
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

	// Verify output is valid JSON
	var result storage.Issue
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ID != id {
		t.Errorf("expected ID %s, got %s", id, result.ID)
	}
	if result.Title != "JSON Test Issue" {
		t.Errorf("expected title 'JSON Test Issue', got %s", result.Title)
	}
}

func TestShowClosedIssue(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create and close an issue
	id, err := store.Create(ctx, &storage.Issue{
		Title:    "Closed Issue",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	if err := store.Close(ctx, id); err != nil {
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
	if !strings.Contains(output, "closed") {
		t.Errorf("expected output to show closed status, got: %s", output)
	}
}

func TestShowWithDependencies(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issues with dependencies
	depID, err := store.Create(ctx, &storage.Issue{
		Title:    "Dependency Issue",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create dependency issue: %v", err)
	}

	mainID, err := store.Create(ctx, &storage.Issue{
		Title:    "Main Issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create main issue: %v", err)
	}

	// Add dependency relationship
	if err := store.AddDependency(ctx, mainID, depID); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test show main issue
	cmd := newShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{mainID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Depends On") {
		t.Errorf("expected output to contain 'Depends On' section, got: %s", output)
	}
	if !strings.Contains(output, depID) {
		t.Errorf("expected output to contain dependency ID %s, got: %s", depID, output)
	}
}

func TestShowWithComments(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an issue
	id, err := store.Create(ctx, &storage.Issue{
		Title:    "Issue with Comments",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Add a comment
	if err := store.AddComment(ctx, id, &storage.Comment{
		Author: "bob",
		Body:   "This is a test comment",
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
		t.Errorf("expected output to contain 'Comments (1)' section, got: %s", output)
	}
	if !strings.Contains(output, "bob") {
		t.Errorf("expected output to contain comment author, got: %s", output)
	}
	if !strings.Contains(output, "This is a test comment") {
		t.Errorf("expected output to contain comment body, got: %s", output)
	}
}
