package cmd

import (
	"bytes"
	"context"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
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
	depID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Dependency issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create a blocked issue (depends on open issue)
	blockedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Blocked issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Add dependency: blockedID depends on depID
	if err := store.AddDependency(ctx, blockedID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create an unblocked issue
	unblockedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Unblocked issue",
		Priority: issuestorage.PriorityLow,
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
	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
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

	// Create a blocker issue (open)
	blockerID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Blocker issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create an issue that is blocked by the blocker
	blockedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Blocked issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Add dependency: blockedID depends on blockerID (blockerID blocks blockedID)
	if err := store.AddDependency(ctx, blockedID, blockerID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test blocked command
	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	if !containsString(output, blockedID) {
		t.Errorf("expected output to contain blocked issue %s, got: %s", blockedID, output)
	}
	if !containsString(output, blockerID) {
		t.Errorf("expected output to show blocker %s, got: %s", blockerID, output)
	}
}

func TestBlockedNoBlockedIssues(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create only unblocked issues
	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Unblocked issue 1",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	_, err = store.Create(ctx, &issuestorage.Issue{
		Title:    "Unblocked issue 2",
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

	// Test blocked command
	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	if !containsString(output, "No blocked issues found") {
		t.Errorf("expected 'No blocked issues found' message, got: %s", output)
	}
}

func TestBlockedClosedDependency(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create and close a dependency issue
	depID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Closed dependency",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Close(ctx, depID); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Create an issue that depends on the closed issue
	issueID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Issue with closed dependency",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Add dependency: issueID depends on depID
	if err := store.AddDependency(ctx, issueID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test blocked command - issue should NOT be blocked since dependency is closed
	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	if containsString(output, issueID) {
		t.Errorf("expected output NOT to contain issue %s (dependency is closed), got: %s", issueID, output)
	}
}

func TestBlockedJSONOutput(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a dependency issue (open)
	depID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Dependency issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create a blocked issue
	blockedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Blocked issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Add dependency: blockedID depends on depID
	if err := store.AddDependency(ctx, blockedID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create app for testing with JSON output
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	// Test blocked command
	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	// Basic JSON structure check
	if !containsString(output, blockedID) {
		t.Errorf("expected JSON output to contain blocked issue ID %s, got: %s", blockedID, output)
	}
	if !containsString(output, depID) {
		t.Errorf("expected JSON output to contain dependency ID %s, got: %s", depID, output)
	}
	if !containsString(output, "blocked_by") {
		t.Errorf("expected JSON output to contain 'blocked_by' field, got: %s", output)
	}
}

func TestBlockedMultipleDependencies(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create two open dependency issues
	dep1ID, _ := store.Create(ctx, &issuestorage.Issue{Title: "Dep 1"})
	dep2ID, _ := store.Create(ctx, &issuestorage.Issue{Title: "Dep 2"})

	// Create an issue blocked by both
	blockedID, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "Multiply blocked",
	})
	// Add dependencies: blockedID depends on both dep1ID and dep2ID
	if err := store.AddDependency(ctx, blockedID, dep1ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency on dep1: %v", err)
	}
	if err := store.AddDependency(ctx, blockedID, dep2ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency on dep2: %v", err)
	}

	// Create app for testing
	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Test blocked command
	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	if !containsString(output, blockedID) {
		t.Errorf("expected output to contain blocked issue %s", blockedID)
	}
	if !containsString(output, dep1ID) || !containsString(output, dep2ID) {
		t.Errorf("expected output to show both dependencies, got: %s", output)
	}
}

func TestBlockedExcludesEphemeralIssues(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an open dependency issue
	depID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open dependency",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create dep issue: %v", err)
	}

	// Create an ephemeral blocked issue — should be excluded
	ephID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Ephemeral blocked issue",
		Priority:  issuestorage.PriorityMedium,
		Ephemeral: true,
	})
	if err != nil {
		t.Fatalf("failed to create ephemeral issue: %v", err)
	}
	if err := store.AddDependency(ctx, ephID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Create a persistent blocked issue — should be included
	persistID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Persistent blocked issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create persistent issue: %v", err)
	}
	if err := store.AddDependency(ctx, persistID, depID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	if containsString(output, ephID) {
		t.Errorf("expected ephemeral issue %s to be excluded from blocked output, got: %s", ephID, output)
	}
	if !containsString(output, persistID) {
		t.Errorf("expected persistent issue %s to be included in blocked output, got: %s", persistID, output)
	}
}

func TestBlockedPersistentBlockedIssuesStillShown(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create two open dependency issues
	dep1ID, _ := store.Create(ctx, &issuestorage.Issue{Title: "Dep A", Priority: issuestorage.PriorityHigh})
	dep2ID, _ := store.Create(ctx, &issuestorage.Issue{Title: "Dep B", Priority: issuestorage.PriorityHigh})

	// Create two persistent blocked issues
	blocked1ID, _ := store.Create(ctx, &issuestorage.Issue{
		Title:    "Persistent blocked 1",
		Priority: issuestorage.PriorityMedium,
	})
	if err := store.AddDependency(ctx, blocked1ID, dep1ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	blocked2ID, _ := store.Create(ctx, &issuestorage.Issue{
		Title:    "Persistent blocked 2",
		Priority: issuestorage.PriorityLow,
	})
	if err := store.AddDependency(ctx, blocked2ID, dep2ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newBlockedCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blocked command failed: %v", err)
	}

	output := out.String()
	if !containsString(output, blocked1ID) {
		t.Errorf("expected persistent blocked issue %s in output, got: %s", blocked1ID, output)
	}
	if !containsString(output, blocked2ID) {
		t.Errorf("expected persistent blocked issue %s in output, got: %s", blocked2ID, output)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
