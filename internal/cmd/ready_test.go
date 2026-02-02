package cmd

import (
	"bytes"
	"context"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func TestReadyCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create some test issues
	// Issue 1: ready (no dependencies)
	id1, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Ready issue 1",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Issue 2: ready (no dependencies)
	id2, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Ready issue 2",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Issue 3: blocked (depends on issue 2)
	id3, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Blocked issue",
		Priority: issuestorage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Add dependency: id3 depends on id2 (id2 blocks id3)
	if err := store.AddDependency(ctx, id3, id2, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a dependency issue
	depID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Dependency",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create an issue that depends on it
	mainID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Main issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Add dependency: mainID depends on depID (depID blocks mainID)
	if err := store.AddDependency(ctx, mainID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
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

func TestReadyExcludesEphemeral(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a normal issue (should appear)
	normalID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Normal issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create an ephemeral issue (should NOT appear)
	ephID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Ephemeral issue",
		Priority:  issuestorage.PriorityHigh,
		Ephemeral: true,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	cmd := newReadyCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte(normalID)) {
		t.Errorf("expected output to contain normal issue %s, got: %s", normalID, output)
	}
	if bytes.Contains([]byte(output), []byte(ephID)) {
		t.Errorf("expected output NOT to contain ephemeral issue %s, got: %s", ephID, output)
	}
}

func TestReadyMolShowsOnlyMoleculeSteps(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a molecule root
	molRootID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Molecule root",
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create mol root: %v", err)
	}

	// Create two molecule steps (children of root)
	stepID1, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Step 1",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create step 1: %v", err)
	}
	if err := store.AddDependency(ctx, stepID1, molRootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child dep: %v", err)
	}

	stepID2, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Step 2",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create step 2: %v", err)
	}
	if err := store.AddDependency(ctx, stepID2, molRootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child dep: %v", err)
	}

	// Create a non-molecule issue
	otherID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Other top-level issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create other issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	// With --mol: should show only molecule steps, not other issues
	cmd := newReadyCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--mol", molRootID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready --mol failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte(stepID1)) {
		t.Errorf("expected --mol output to contain step %s, got: %s", stepID1, output)
	}
	if !bytes.Contains([]byte(output), []byte(stepID2)) {
		t.Errorf("expected --mol output to contain step %s, got: %s", stepID2, output)
	}
	if bytes.Contains([]byte(output), []byte(otherID)) {
		t.Errorf("expected --mol output NOT to contain other issue %s, got: %s", otherID, output)
	}
	// Should show parallel info for multiple ready steps
	if !bytes.Contains([]byte(output), []byte("parallel")) {
		t.Errorf("expected --mol output to mention parallel steps, got: %s", output)
	}
}

func TestReadyWithoutMolExcludesMoleculeSteps(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a molecule root
	molRootID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Molecule root",
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create mol root: %v", err)
	}

	// Create a molecule step (child of root)
	stepID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Molecule step",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create step: %v", err)
	}
	if err := store.AddDependency(ctx, stepID, molRootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child dep: %v", err)
	}

	// Create a top-level issue
	topID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Top-level issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create top-level issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
	}

	// Without --mol: should show top-level issue, exclude molecule step
	cmd := newReadyCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ready command failed: %v", err)
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte(topID)) {
		t.Errorf("expected output to contain top-level issue %s, got: %s", topID, output)
	}
	if bytes.Contains([]byte(output), []byte(stepID)) {
		t.Errorf("expected output NOT to contain molecule step %s, got: %s", stepID, output)
	}
}

func TestReadyJSON(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a ready issue
	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Ready issue",
		Priority: issuestorage.PriorityHigh,
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
