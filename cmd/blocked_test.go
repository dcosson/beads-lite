package cmd

import (
	"bytes"
	"context"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

func TestBlockedCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a dependency issue (open)
	depID, err := store.Create(ctx, &storage.Issue{
		Title:    "Dependency issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create a blocked issue (depends on open issue)
	blockedID, err := store.Create(ctx, &storage.Issue{
		Title:     "Blocked issue",
		Priority:  storage.PriorityMedium,
		DependsOn: []string{depID},
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create an unblocked issue
	unblockedID, err := store.Create(ctx, &storage.Issue{
		Title:    "Unblocked issue",
		Priority: storage.PriorityLow,
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

	// Test blocked command
	err = runBlocked(ctx, app)
	if err != nil {
		t.Fatalf("runBlocked failed: %v", err)
	}

	output := out.String()
	if !containsString(output, blockedID) {
		t.Errorf("expected output to contain blocked issue %s, got: %s", blockedID, output)
	}
	if !containsString(output, depID) {
		t.Errorf("expected output to show dependency %s as blocker, got: %s", depID, output)
	}
	if containsString(output, unblockedID) {
		t.Errorf("expected output NOT to contain unblocked issue %s, got: %s", unblockedID, output)
	}
}

func TestBlockedByRelationship(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a blocker issue
	blockerID, err := store.Create(ctx, &storage.Issue{
		Title:    "Blocker issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create a blocked issue using blocked_by relationship
	blockedID, err := store.Create(ctx, &storage.Issue{
		Title:     "Blocked by issue",
		Priority:  storage.PriorityMedium,
		BlockedBy: []string{blockerID},
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

	// Test blocked command
	err = runBlocked(ctx, app)
	if err != nil {
		t.Fatalf("runBlocked failed: %v", err)
	}

	output := out.String()
	if !containsString(output, blockedID) {
		t.Errorf("expected output to contain blocked issue %s, got: %s", blockedID, output)
	}
	if !containsString(output, blockerID) {
		t.Errorf("expected output to show blocker %s, got: %s", blockerID, output)
	}
}

func TestBlockedWithClosedDependency(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a dependency issue
	depID, err := store.Create(ctx, &storage.Issue{
		Title:    "Dependency issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create an issue that depends on it
	mainID, err := store.Create(ctx, &storage.Issue{
		Title:     "Main issue",
		Priority:  storage.PriorityMedium,
		DependsOn: []string{depID},
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

	// Initially should be blocked
	err = runBlocked(ctx, app)
	if err != nil {
		t.Fatalf("runBlocked failed: %v", err)
	}

	output := out.String()
	if !containsString(output, mainID) {
		t.Errorf("expected main issue to be blocked, got: %s", output)
	}

	// Close the dependency
	if err := store.Close(ctx, depID); err != nil {
		t.Fatalf("failed to close dependency: %v", err)
	}

	// Now should NOT be blocked
	out.Reset()
	err = runBlocked(ctx, app)
	if err != nil {
		t.Fatalf("runBlocked failed after closing dependency: %v", err)
	}

	output = out.String()
	if containsString(output, mainID) {
		t.Errorf("expected main issue NOT to be blocked (dependency closed), got: %s", output)
	}
}

func TestBlockedJSON(t *testing.T) {
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

	// Create a blocked issue
	_, err = store.Create(ctx, &storage.Issue{
		Title:     "Blocked issue",
		Priority:  storage.PriorityMedium,
		DependsOn: []string{depID},
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

	err = runBlocked(ctx, app)
	if err != nil {
		t.Fatalf("runBlocked JSON failed: %v", err)
	}

	output := out.String()
	if !containsString(output, "[") {
		t.Errorf("expected JSON array output, got: %s", output)
	}
	if !containsString(output, "waiting_on") {
		t.Errorf("expected JSON to contain waiting_on field, got: %s", output)
	}
}

func TestNoBlockedIssues(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an unblocked issue
	_, err := store.Create(ctx, &storage.Issue{
		Title:    "Unblocked issue",
		Priority: storage.PriorityHigh,
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

	// Test blocked command
	err = runBlocked(ctx, app)
	if err != nil {
		t.Fatalf("runBlocked failed: %v", err)
	}

	output := out.String()
	if !containsString(output, "No blocked issues") {
		t.Errorf("expected 'No blocked issues' message, got: %s", output)
	}
}
