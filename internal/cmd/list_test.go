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

func TestListCommand_DefaultListsOpenIssues(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create open and closed issues
	openID, err := store.Create(ctx, &storage.Issue{
		Title:    "Open issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	closedID, err := store.Create(ctx, &storage.Issue{
		Title:    "Closed issue",
		Priority: storage.PriorityMedium,
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

	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, openID) {
		t.Errorf("expected output to contain open issue %s, got: %s", openID, output)
	}
	if strings.Contains(output, closedID) {
		t.Errorf("expected output NOT to contain closed issue %s, got: %s", closedID, output)
	}
}

func TestListCommand_AllFlag(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create open and closed issues
	openID, err := store.Create(ctx, &storage.Issue{
		Title:    "Open issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	closedID, err := store.Create(ctx, &storage.Issue{
		Title:    "Closed issue",
		Priority: storage.PriorityMedium,
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

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, openID) {
		t.Errorf("expected output to contain open issue %s, got: %s", openID, output)
	}
	if !strings.Contains(output, closedID) {
		t.Errorf("expected output to contain closed issue %s, got: %s", closedID, output)
	}
}

func TestListCommand_ClosedFlag(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create open and closed issues
	openID, err := store.Create(ctx, &storage.Issue{
		Title:    "Open issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	closedID, err := store.Create(ctx, &storage.Issue{
		Title:    "Closed issue",
		Priority: storage.PriorityMedium,
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

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--closed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if strings.Contains(output, openID) {
		t.Errorf("expected output NOT to contain open issue %s, got: %s", openID, output)
	}
	if !strings.Contains(output, closedID) {
		t.Errorf("expected output to contain closed issue %s, got: %s", closedID, output)
	}
}

func TestListCommand_StatusFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create issues with different statuses
	openID, err := store.Create(ctx, &storage.Issue{
		Title:    "Open issue",
		Priority: storage.PriorityHigh,
		Status:   storage.StatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	inProgressID, err := store.Create(ctx, &storage.Issue{
		Title:    "In-progress issue",
		Priority: storage.PriorityHigh,
		Status:   storage.StatusInProgress,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Update status to in-progress
	issue, _ := store.Get(ctx, inProgressID)
	issue.Status = storage.StatusInProgress
	store.Update(ctx, issue)

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--status=in-progress"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if strings.Contains(output, openID) {
		t.Errorf("expected output NOT to contain open issue %s, got: %s", openID, output)
	}
	if !strings.Contains(output, inProgressID) {
		t.Errorf("expected output to contain in-progress issue %s, got: %s", inProgressID, output)
	}
}

func TestListCommand_PriorityFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	highID, err := store.Create(ctx, &storage.Issue{
		Title:    "High priority",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	lowID, err := store.Create(ctx, &storage.Issue{
		Title:    "Low priority",
		Priority: storage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--priority=high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, highID) {
		t.Errorf("expected output to contain high priority issue %s, got: %s", highID, output)
	}
	if strings.Contains(output, lowID) {
		t.Errorf("expected output NOT to contain low priority issue %s, got: %s", lowID, output)
	}
}

func TestListCommand_TypeFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	bugID, err := store.Create(ctx, &storage.Issue{
		Title:    "Bug issue",
		Priority: storage.PriorityHigh,
		Type:     storage.TypeBug,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	taskID, err := store.Create(ctx, &storage.Issue{
		Title:    "Task issue",
		Priority: storage.PriorityHigh,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--type=bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, bugID) {
		t.Errorf("expected output to contain bug %s, got: %s", bugID, output)
	}
	if strings.Contains(output, taskID) {
		t.Errorf("expected output NOT to contain task %s, got: %s", taskID, output)
	}
}

func TestListCommand_LabelsFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	labeledID, err := store.Create(ctx, &storage.Issue{
		Title:    "Labeled issue",
		Priority: storage.PriorityHigh,
		Labels:   []string{"urgent", "v2"},
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	unlabeledID, err := store.Create(ctx, &storage.Issue{
		Title:    "Unlabeled issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--labels=urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, labeledID) {
		t.Errorf("expected output to contain labeled issue %s, got: %s", labeledID, output)
	}
	if strings.Contains(output, unlabeledID) {
		t.Errorf("expected output NOT to contain unlabeled issue %s, got: %s", unlabeledID, output)
	}
}

func TestListCommand_AssigneeFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	aliceID, err := store.Create(ctx, &storage.Issue{
		Title:    "Alice's issue",
		Priority: storage.PriorityHigh,
		Assignee: "alice",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	bobID, err := store.Create(ctx, &storage.Issue{
		Title:    "Bob's issue",
		Priority: storage.PriorityHigh,
		Assignee: "bob",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--assignee=alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, aliceID) {
		t.Errorf("expected output to contain Alice's issue %s, got: %s", aliceID, output)
	}
	if strings.Contains(output, bobID) {
		t.Errorf("expected output NOT to contain Bob's issue %s, got: %s", bobID, output)
	}
}

func TestListCommand_ParentFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create parent issue
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent issue",
		Priority: storage.PriorityHigh,
		Type:     storage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create child issue
	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child issue",
		Priority: storage.PriorityMedium,
		Parent:   parentID,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create another root issue
	rootID, err := store.Create(ctx, &storage.Issue{
		Title:    "Another root",
		Priority: storage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--parent=" + parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, childID) {
		t.Errorf("expected output to contain child issue %s, got: %s", childID, output)
	}
	if strings.Contains(output, rootID) {
		t.Errorf("expected output NOT to contain root issue %s, got: %s", rootID, output)
	}
}

func TestListCommand_RootsFlag(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create parent issue
	parentID, err := store.Create(ctx, &storage.Issue{
		Title:    "Parent issue",
		Priority: storage.PriorityHigh,
		Type:     storage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create child issue
	childID, err := store.Create(ctx, &storage.Issue{
		Title:    "Child issue",
		Priority: storage.PriorityMedium,
		Parent:   parentID,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--roots"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, parentID) {
		t.Errorf("expected output to contain root issue %s, got: %s", parentID, output)
	}
	if strings.Contains(output, childID) {
		t.Errorf("expected output NOT to contain child issue %s, got: %s", childID, output)
	}
}

func TestListCommand_FormatIds(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id1, err := store.Create(ctx, &storage.Issue{
		Title:    "Issue 1",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	id2, err := store.Create(ctx, &storage.Issue{
		Title:    "Issue 2",
		Priority: storage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--format=ids"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines for ids format, got %d: %s", len(lines), output)
	}
	// Check that output contains only IDs
	if !strings.Contains(output, id1) || !strings.Contains(output, id2) {
		t.Errorf("expected output to contain both IDs, got: %s", output)
	}
	// Check that titles are NOT in output
	if strings.Contains(output, "Issue 1") || strings.Contains(output, "Issue 2") {
		t.Errorf("expected ids format NOT to include titles, got: %s", output)
	}
}

func TestListCommand_FormatShort(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &storage.Issue{
		Title:    "Test issue",
		Priority: storage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--format=short"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, id) {
		t.Errorf("expected output to contain ID %s, got: %s", id, output)
	}
	if !strings.Contains(output, "Test issue") {
		t.Errorf("expected output to contain title, got: %s", output)
	}
	// Short format should NOT include status/type/priority brackets
	if strings.Contains(output, "[open]") {
		t.Errorf("expected short format NOT to include status, got: %s", output)
	}
}

func TestListCommand_JSON(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	_, err := store.Create(ctx, &storage.Issue{
		Title:    "Test issue",
		Priority: storage.PriorityHigh,
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
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	var issues []*storage.Issue
	if err := json.Unmarshal([]byte(output), &issues); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v, output: %s", err, output)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue in JSON output, got %d", len(issues))
	}
	if issues[0].Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got '%s'", issues[0].Title)
	}
}

func TestListCommand_NoIssues(t *testing.T) {
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
