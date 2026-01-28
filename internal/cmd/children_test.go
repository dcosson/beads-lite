package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
	"beads-lite/internal/storage/filesystem"
)

func TestChildrenCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create parent issue
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent Issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create parent issue: %v", err)
	}

	// Create child issues
	child1ID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child One",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create child 1: %v", err)
	}

	child2ID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child Two",
		Priority: storage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create child 2: %v", err)
	}

	// Set parent relationships
	if err := store.SetParent(ctx, child1ID, parentID); err != nil {
		t.Fatalf("failed to set parent for child 1: %v", err)
	}
	if err := store.SetParent(ctx, child2ID, parentID); err != nil {
		t.Fatalf("failed to set parent for child 2: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test children command
	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, child1ID) {
		t.Errorf("expected output to contain child 1 ID %s, got: %s", child1ID, output)
	}
	if !strings.Contains(output, child2ID) {
		t.Errorf("expected output to contain child 2 ID %s, got: %s", child2ID, output)
	}
	if !strings.Contains(output, "Child One") {
		t.Errorf("expected output to contain child 1 title, got: %s", output)
	}
	if !strings.Contains(output, "Child Two") {
		t.Errorf("expected output to contain child 2 title, got: %s", output)
	}
	if !strings.Contains(output, "(2)") {
		t.Errorf("expected output to show count (2), got: %s", output)
	}
}

func TestChildrenNoChildren(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issue with no children
	id, err := store.Create(ctx, &storage.Issue{
		Title:    "Childless Issue",
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

	// Test children command
	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No children") {
		t.Errorf("expected output to say 'No children', got: %s", output)
	}
}

func TestChildrenTreeFlag(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create hierarchy: parent -> child -> grandchild
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	grandchildID, err := store.Create(ctx, &storage.Issue{
		Title:    "Grandchild",
		Priority: storage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create grandchild: %v", err)
	}

	// Set parent relationships
	if err := store.SetParent(ctx, childID, parentID); err != nil {
		t.Fatalf("failed to set parent for child: %v", err)
	}
	if err := store.SetParent(ctx, grandchildID, childID); err != nil {
		t.Fatalf("failed to set parent for grandchild: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test children command with --tree flag
	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID, "--tree"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command with --tree failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, childID) {
		t.Errorf("expected output to contain child ID %s, got: %s", childID, output)
	}
	if !strings.Contains(output, grandchildID) {
		t.Errorf("expected output to contain grandchild ID %s, got: %s", grandchildID, output)
	}
	if !strings.Contains(output, "Subtree of") {
		t.Errorf("expected output to say 'Subtree of', got: %s", output)
	}
	// Check for tree formatting characters
	if !strings.Contains(output, "└──") && !strings.Contains(output, "├──") {
		t.Errorf("expected output to contain tree formatting, got: %s", output)
	}
}

func TestChildrenJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create parent and child
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	if err := store.SetParent(ctx, childID, parentID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create app with JSON output
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command JSON failed: %v", err)
	}

	// Verify output is valid JSON
	var result []ChildInfo
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 child, got %d", len(result))
	}
	if result[0].ID != childID {
		t.Errorf("expected child ID %s, got %s", childID, result[0].ID)
	}
}

func TestChildrenTreeJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create hierarchy: parent -> child -> grandchild
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	grandchildID, err := store.Create(ctx, &storage.Issue{
		Title:    "Grandchild",
		Priority: storage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create grandchild: %v", err)
	}

	if err := store.SetParent(ctx, childID, parentID); err != nil {
		t.Fatalf("failed to set parent for child: %v", err)
	}
	if err := store.SetParent(ctx, grandchildID, childID); err != nil {
		t.Fatalf("failed to set parent for grandchild: %v", err)
	}

	// Create app with JSON output
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{parentID, "--tree"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command tree JSON failed: %v", err)
	}

	// Verify output is valid JSON with nested structure
	var result []ChildInfo
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 child at top level, got %d", len(result))
	}
	if result[0].ID != childID {
		t.Errorf("expected child ID %s, got %s", childID, result[0].ID)
	}
	if len(result[0].Children) != 1 {
		t.Errorf("expected 1 grandchild, got %d", len(result[0].Children))
	}
	if result[0].Children[0].ID != grandchildID {
		t.Errorf("expected grandchild ID %s, got %s", grandchildID, result[0].Children[0].ID)
	}
}

func TestChildrenPrefixMatch(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create parent and child
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	if err := store.SetParent(ctx, childID, parentID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test with prefix match (first 4 characters of parent ID)
	prefix := parentID[:4]
	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command with prefix failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, childID) {
		t.Errorf("expected output to contain child ID %s, got: %s", childID, output)
	}
}

func TestChildrenNotFound(t *testing.T) {
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

	// Test with non-existent ID
	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent issue, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "no issue found") {
		t.Errorf("expected error message to say 'no issue found', got: %s", errMsg)
	}
}

func TestChildrenEmptyJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issue with no children
	id, err := store.Create(ctx, &storage.Issue{
		Title:    "Childless",
		Priority: storage.PriorityMedium,
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

	cmd := newChildrenCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("children command failed: %v", err)
	}

	// Verify empty JSON array
	var result []ChildInfo
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}
