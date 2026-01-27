package cmd

import (
	"bytes"
	"context"
	"testing"

	"beads2/internal/storage"
	"beads2/internal/storage/filesystem"
)

func TestReadyCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create some test issues
	// Issue 1: ready (no dependencies)
	id1, err := store.Create(ctx, &storage.Issue{
		Title:    "Ready issue 1",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Issue 2: ready (no dependencies)
	id2, err := store.Create(ctx, &storage.Issue{
		Title:    "Ready issue 2",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Issue 3: blocked (depends on issue 2)
	id3, err := store.Create(ctx, &storage.Issue{
		Title:     "Blocked issue",
		Priority:  storage.PriorityLow,
		DependsOn: []string{id2},
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Also add the dependent relationship on id2
	issue2, _ := store.Get(ctx, id2)
	issue2.Dependents = append(issue2.Dependents, id3)
	store.Update(ctx, issue2)

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test ready command - should show issues 1 and 2, but not 3
	cmd := newReadyCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte(id1)) {
		t.Errorf("expected output to contain %s, got: %s", id1, output)
	}
	if !bytes.Contains([]byte(output), []byte(id2)) {
		t.Errorf("expected output to contain %s, got: %s", id2, output)
	}
	if bytes.Contains([]byte(output), []byte(id3)) {
		t.Errorf("expected output NOT to contain %s (blocked), got: %s", id3, output)
	}

	// Test with priority filter
	out.Reset()
	cmd = newReadyCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--priority", "high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command with priority failed: %v", err)
	}

	output = out.String()
	if !bytes.Contains([]byte(output), []byte(id1)) {
		t.Errorf("expected high priority output to contain %s, got: %s", id1, output)
	}
	if bytes.Contains([]byte(output), []byte(id2)) {
		t.Errorf("expected high priority output NOT to contain %s (medium priority), got: %s", id2, output)
	}
}

func TestReadyWithClosedDependency(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a dependency issue
	depID, err := store.Create(ctx, &storage.Issue{
		Title:    "Dependency",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create an issue that depends on it
	mainID, err := store.Create(ctx, &storage.Issue{
		Title:     "Main issue",
		Priority:  storage.PriorityHigh,
		DependsOn: []string{depID},
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create app
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Initially, main issue should NOT be ready (dependency not closed)
	cmd := newReadyCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command failed: %v", err)
	}

	output := out.String()
	if bytes.Contains([]byte(output), []byte(mainID)) {
		t.Errorf("expected main issue NOT to be ready (dependency open), got: %s", output)
	}

	// Close the dependency
	if err := store.Close(ctx, depID); err != nil {
		t.Fatalf("failed to close dependency: %v", err)
	}

	// Now main issue should be ready
	out.Reset()
	cmd = newReadyCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command failed after closing dependency: %v", err)
	}

	output = out.String()
	if !bytes.Contains([]byte(output), []byte(mainID)) {
		t.Errorf("expected main issue to be ready (dependency closed), got: %s", output)
	}
}

func TestReadyJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a ready issue
	_, err := store.Create(ctx, &storage.Issue{
		Title:    "Ready issue",
		Priority: storage.PriorityHigh,
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

	cmd := newReadyCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command JSON failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte("[")) {
		t.Errorf("expected JSON array output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Ready issue")) {
		t.Errorf("expected JSON to contain issue title, got: %s", output)
	}
}
