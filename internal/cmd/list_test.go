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

func TestListCommand(t *testing.T) {
	// Setup test storage
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create some test issues
	id1, err := store.Create(ctx, &storage.Issue{
		Title:    "Open issue 1",
		Type:     storage.TypeBug,
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	id2, err := store.Create(ctx, &storage.Issue{
		Title:    "Open issue 2",
		Type:     storage.TypeFeature,
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

	// Test default list (open issues only)
	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, id1) {
		t.Errorf("expected output to contain %s, got: %s", id1, output)
	}
	if !strings.Contains(output, id2) {
		t.Errorf("expected output to contain %s, got: %s", id2, output)
	}
}

func TestListWithClosedIssues(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create open issue
	openID, err := store.Create(ctx, &storage.Issue{
		Title: "Open issue",
		Type:  storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create and close an issue
	closedID, err := store.Create(ctx, &storage.Issue{
		Title: "Closed issue",
		Type:  storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Close(ctx, closedID); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Default: only open issues
	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, openID) {
		t.Errorf("expected output to contain %s (open), got: %s", openID, output)
	}
	if strings.Contains(output, closedID) {
		t.Errorf("expected output NOT to contain %s (closed), got: %s", closedID, output)
	}

	// --all: both open and closed
	out.Reset()
	cmd = newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --all command failed: %v", err)
	}

	output = out.String()
	if !strings.Contains(output, openID) {
		t.Errorf("expected --all output to contain %s (open), got: %s", openID, output)
	}
	if !strings.Contains(output, closedID) {
		t.Errorf("expected --all output to contain %s (closed), got: %s", closedID, output)
	}

	// --closed: only closed issues
	out.Reset()
	cmd = newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--closed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --closed command failed: %v", err)
	}

	output = out.String()
	if strings.Contains(output, openID) {
		t.Errorf("expected --closed output NOT to contain %s (open), got: %s", openID, output)
	}
	if !strings.Contains(output, closedID) {
		t.Errorf("expected --closed output to contain %s (closed), got: %s", closedID, output)
	}
}

func TestListTypeFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issues of different types
	bugID, _ := store.Create(ctx, &storage.Issue{
		Title: "Bug issue",
		Type:  storage.TypeBug,
	})
	featureID, _ := store.Create(ctx, &storage.Issue{
		Title: "Feature issue",
		Type:  storage.TypeFeature,
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	// Filter by type bug
	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--type", "bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --type bug command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, bugID) {
		t.Errorf("expected output to contain %s (bug), got: %s", bugID, output)
	}
	if strings.Contains(output, featureID) {
		t.Errorf("expected output NOT to contain %s (feature), got: %s", featureID, output)
	}
}

func TestListPriorityFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	highID, _ := store.Create(ctx, &storage.Issue{
		Title:    "High priority",
		Priority: storage.PriorityHigh,
	})
	lowID, _ := store.Create(ctx, &storage.Issue{
		Title:    "Low priority",
		Priority: storage.PriorityLow,
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--priority", "high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --priority high command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, highID) {
		t.Errorf("expected output to contain %s (high priority), got: %s", highID, output)
	}
	if strings.Contains(output, lowID) {
		t.Errorf("expected output NOT to contain %s (low priority), got: %s", lowID, output)
	}
}

func TestListLabelFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	backendID, _ := store.Create(ctx, &storage.Issue{
		Title:  "Backend issue",
		Labels: []string{"backend", "urgent"},
	})
	frontendID, _ := store.Create(ctx, &storage.Issue{
		Title:  "Frontend issue",
		Labels: []string{"frontend"},
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--label", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --label backend command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, backendID) {
		t.Errorf("expected output to contain %s (has backend label), got: %s", backendID, output)
	}
	if strings.Contains(output, frontendID) {
		t.Errorf("expected output NOT to contain %s (no backend label), got: %s", frontendID, output)
	}
}

func TestListRootsFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a root issue
	rootID, _ := store.Create(ctx, &storage.Issue{
		Title: "Root issue",
	})

	// Create a child issue
	childID, _ := store.Create(ctx, &storage.Issue{
		Title:  "Child issue",
		Parent: rootID,
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--roots"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --roots command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, rootID) {
		t.Errorf("expected output to contain %s (root), got: %s", rootID, output)
	}
	if strings.Contains(output, childID) {
		t.Errorf("expected output NOT to contain %s (has parent), got: %s", childID, output)
	}
}

func TestListParentFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a parent issue
	parentID, _ := store.Create(ctx, &storage.Issue{
		Title: "Parent issue",
	})

	// Create children of the parent
	child1ID, _ := store.Create(ctx, &storage.Issue{
		Title:  "Child 1",
		Parent: parentID,
	})
	child2ID, _ := store.Create(ctx, &storage.Issue{
		Title:  "Child 2",
		Parent: parentID,
	})

	// Create an unrelated root issue
	unrelatedID, _ := store.Create(ctx, &storage.Issue{
		Title: "Unrelated issue",
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--parent", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --parent command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, child1ID) {
		t.Errorf("expected output to contain %s (child), got: %s", child1ID, output)
	}
	if !strings.Contains(output, child2ID) {
		t.Errorf("expected output to contain %s (child), got: %s", child2ID, output)
	}
	if strings.Contains(output, parentID) {
		t.Errorf("expected output NOT to contain %s (parent itself), got: %s", parentID, output)
	}
	if strings.Contains(output, unrelatedID) {
		t.Errorf("expected output NOT to contain %s (unrelated), got: %s", unrelatedID, output)
	}
}

func TestListAssigneeFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	aliceID, _ := store.Create(ctx, &storage.Issue{
		Title:    "Alice's issue",
		Assignee: "alice",
	})
	bobID, _ := store.Create(ctx, &storage.Issue{
		Title:    "Bob's issue",
		Assignee: "bob",
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--assignee", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --assignee command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, aliceID) {
		t.Errorf("expected output to contain %s (alice), got: %s", aliceID, output)
	}
	if strings.Contains(output, bobID) {
		t.Errorf("expected output NOT to contain %s (bob), got: %s", bobID, output)
	}
}

func TestListFormatIds(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id1, _ := store.Create(ctx, &storage.Issue{Title: "Issue 1"})
	id2, _ := store.Create(ctx, &storage.Issue{Title: "Issue 2"})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--format", "ids"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --format ids command failed: %v", err)
	}

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %s", len(lines), output)
	}
	// Should only contain IDs, not titles
	if strings.Contains(output, "Issue 1") {
		t.Errorf("expected ids format to NOT contain title, got: %s", output)
	}
	if !strings.Contains(output, id1) || !strings.Contains(output, id2) {
		t.Errorf("expected ids format to contain both IDs, got: %s", output)
	}
}

func TestListFormatLong(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, _ := store.Create(ctx, &storage.Issue{
		Title:       "Test issue",
		Description: "A detailed description",
		Type:        storage.TypeBug,
		Priority:    storage.PriorityHigh,
		Labels:      []string{"urgent", "backend"},
		Assignee:    "alice",
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--format", "long"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --format long command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, id) {
		t.Errorf("expected long format to contain ID, got: %s", output)
	}
	if !strings.Contains(output, "Test issue") {
		t.Errorf("expected long format to contain title, got: %s", output)
	}
	if !strings.Contains(output, "A detailed description") {
		t.Errorf("expected long format to contain description, got: %s", output)
	}
	if !strings.Contains(output, "bug") {
		t.Errorf("expected long format to contain type, got: %s", output)
	}
	if !strings.Contains(output, "high") {
		t.Errorf("expected long format to contain priority, got: %s", output)
	}
	if !strings.Contains(output, "alice") {
		t.Errorf("expected long format to contain assignee, got: %s", output)
	}
	if !strings.Contains(output, "urgent") || !strings.Contains(output, "backend") {
		t.Errorf("expected long format to contain labels, got: %s", output)
	}
}

func TestListJSON(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	_, err := store.Create(ctx, &storage.Issue{
		Title: "Test issue",
		Type:  storage.TypeBug,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list JSON command failed: %v", err)
	}

	output := out.String()

	// Should be valid JSON array
	var issues []storage.Issue
	if err := json.Unmarshal([]byte(output), &issues); err != nil {
		t.Errorf("expected valid JSON, got parse error: %v, output: %s", err, output)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue in JSON, got %d", len(issues))
	}
	if issues[0].Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got '%s'", issues[0].Title)
	}
}

func TestListNoIssues(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No issues found") {
		t.Errorf("expected 'No issues found' message, got: %s", output)
	}
}

func TestListMultipleFilters(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issues with various combinations
	matchID, _ := store.Create(ctx, &storage.Issue{
		Title:    "Match: high priority bug",
		Type:     storage.TypeBug,
		Priority: storage.PriorityHigh,
	})
	noMatchType, _ := store.Create(ctx, &storage.Issue{
		Title:    "No match: high priority feature",
		Type:     storage.TypeFeature,
		Priority: storage.PriorityHigh,
	})
	noMatchPriority, _ := store.Create(ctx, &storage.Issue{
		Title:    "No match: low priority bug",
		Type:     storage.TypeBug,
		Priority: storage.PriorityLow,
	})

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--type", "bug", "--priority", "high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list with multiple filters failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, matchID) {
		t.Errorf("expected output to contain %s (matches both filters), got: %s", matchID, output)
	}
	if strings.Contains(output, noMatchType) {
		t.Errorf("expected output NOT to contain %s (wrong type), got: %s", noMatchType, output)
	}
	if strings.Contains(output, noMatchPriority) {
		t.Errorf("expected output NOT to contain %s (wrong priority), got: %s", noMatchPriority, output)
	}
}
