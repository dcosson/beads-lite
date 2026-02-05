package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/issueservice"
)

func TestListCommand_DefaultListsNonClosedIssues(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create issues in every non-closed status to verify the default list
	// includes all of them, not just "open".
	openID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	inProgressID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "In-progress issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, inProgressID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusInProgress; return nil }); err != nil {
		t.Fatalf("failed to set in-progress: %v", err)
	}

	blockedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Blocked issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, blockedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusBlocked; return nil }); err != nil {
		t.Fatalf("failed to set blocked: %v", err)
	}

	deferredID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Deferred issue",
		Priority: issuestorage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, deferredID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusDeferred; return nil }); err != nil {
		t.Fatalf("failed to set deferred: %v", err)
	}

	closedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Closed issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, closedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil }); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	for _, tc := range []struct {
		id     string
		label  string
		expect bool
	}{
		{openID, "open", true},
		{inProgressID, "in-progress", true},
		{blockedID, "blocked", true},
		{deferredID, "deferred", true},
		{closedID, "closed", false},
	} {
		found := strings.Contains(output, tc.id)
		if tc.expect && !found {
			t.Errorf("expected output to contain %s issue %s, got: %s", tc.label, tc.id, output)
		}
		if !tc.expect && found {
			t.Errorf("expected output NOT to contain %s issue %s, got: %s", tc.label, tc.id, output)
		}
	}
}

func TestListCommand_AllFlag(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create open, in-progress, and closed issues — --all should show all of them
	openID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	inProgressID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "In-progress issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, inProgressID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusInProgress; return nil }); err != nil {
		t.Fatalf("failed to set in-progress: %v", err)
	}

	closedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Closed issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, closedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil }); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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
	if !strings.Contains(output, inProgressID) {
		t.Errorf("expected output to contain in-progress issue %s, got: %s", inProgressID, output)
	}
	if !strings.Contains(output, closedID) {
		t.Errorf("expected output to contain closed issue %s, got: %s", closedID, output)
	}
}

func TestListCommand_ClosedFlag(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create open and closed issues
	openID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	closedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Closed issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(ctx, closedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil }); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create issues with different statuses
	openID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open issue",
		Priority: issuestorage.PriorityHigh,
		Status:   issuestorage.StatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	inProgressID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "In-progress issue",
		Priority: issuestorage.PriorityHigh,
		Status:   issuestorage.StatusInProgress,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	// Update status to in-progress
	store.Modify(ctx, inProgressID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusInProgress; return nil })

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	highID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "High priority",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	lowID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Low priority",
		Priority: issuestorage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--priority=P1"})
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	bugID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Bug issue",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeBug,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	taskID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Task issue",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	labeledID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Labeled issue",
		Priority: issuestorage.PriorityHigh,
		Labels:   []string{"urgent", "v2"},
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	unlabeledID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Unlabeled issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--label=urgent"})
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	aliceID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Alice's issue",
		Priority: issuestorage.PriorityHigh,
		Assignee: "alice",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	bobID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Bob's issue",
		Priority: issuestorage.PriorityHigh,
		Assignee: "bob",
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create parent issue
	parentID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Parent issue",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create child issue
	childID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Child issue",
		Priority: issuestorage.PriorityMedium,
		Parent:   parentID,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create another root issue
	rootID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Another root",
		Priority: issuestorage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create parent issue
	parentID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Parent issue",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeEpic,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	// Create child issue
	childID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Child issue",
		Priority: issuestorage.PriorityMedium,
		Parent:   parentID,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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

func TestListCommand_FormatIsNoop(t *testing.T) {
	// --format flag is accepted but not implemented (matching original beads)
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Test issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	// Test that --format is accepted with any value but produces default output
	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--format=anyvalue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	// Should produce default output with status icon and priority/type brackets
	if !strings.Contains(output, "○") {
		t.Errorf("expected status icon in output, got: %s", output)
	}
	if !strings.Contains(output, "- Test issue") {
		t.Errorf("expected output to contain title, got: %s", output)
	}
}

func TestListCommand_JSON(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Test issue",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    true,
	}

	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	var issues []IssueListJSON
	if err := json.Unmarshal([]byte(output), &issues); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v, output: %s", err, output)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue in JSON output, got %d", len(issues))
	}
	if issues[0].Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got '%s'", issues[0].Title)
	}
	// Verify new format fields
	if issues[0].IssueType != "task" {
		t.Errorf("expected issue_type 'task', got '%s'", issues[0].IssueType)
	}
	if issues[0].Priority != 1 {
		t.Errorf("expected priority 1, got %d", issues[0].Priority)
	}
}

func TestListCommand_NoIssues(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	var out bytes.Buffer
	app := &App{
		Storage: rs,
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

func TestListCommand_StatusAll(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create issues with different statuses
	openID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Open issue",
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	inProgressID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "In-progress issue",
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	store.Modify(ctx, inProgressID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusInProgress; return nil })

	closedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Closed issue",
		Priority: issuestorage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	store.Modify(ctx, closedID, func(i *issuestorage.Issue) error { i.Status = issuestorage.StatusClosed; return nil })

	// Create a deleted issue (tombstone)
	deletedID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Deleted issue",
		Priority: issuestorage.PriorityLow,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	softDelete(ctx, store, deletedID, "test", "test deletion")

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--status=all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, openID) {
		t.Errorf("expected output to contain open issue %s, got: %s", openID, output)
	}
	if !strings.Contains(output, inProgressID) {
		t.Errorf("expected output to contain in-progress issue %s, got: %s", inProgressID, output)
	}
	if !strings.Contains(output, closedID) {
		t.Errorf("expected output to contain closed issue %s, got: %s", closedID, output)
	}
	if strings.Contains(output, deletedID) {
		t.Errorf("expected output NOT to contain deleted issue %s, got: %s", deletedID, output)
	}
}

func TestListCommand_MolTypeFilter(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	swarmID, err := store.Create(ctx, &issuestorage.Issue{
		Title:   "Swarm issue",
		MolType: issuestorage.MolTypeSwarm,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	patrolID, err := store.Create(ctx, &issuestorage.Issue{
		Title:   "Patrol issue",
		MolType: issuestorage.MolTypePatrol,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	workID, err := store.Create(ctx, &issuestorage.Issue{
		Title:   "Work issue",
		MolType: issuestorage.MolTypeWork,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	// Filter by swarm
	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--mol-type=swarm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, swarmID) {
		t.Errorf("expected output to contain swarm issue %s, got: %s", swarmID, output)
	}
	if strings.Contains(output, patrolID) {
		t.Errorf("expected output NOT to contain patrol issue %s, got: %s", patrolID, output)
	}
	if strings.Contains(output, workID) {
		t.Errorf("expected output NOT to contain work issue %s, got: %s", workID, output)
	}
}

func TestListCommand_MolTypeInvalid(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    false,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--mol-type=invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid mol-type")
	}
	if !strings.Contains(err.Error(), "invalid mol-type") {
		t.Errorf("expected error about invalid mol-type, got: %v", err)
	}
}

func TestListCommand_DefaultLimit(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	// Create 55 issues (more than the default limit of 50)
	for i := 0; i < 55; i++ {
		_, err := store.Create(ctx, &issuestorage.Issue{
			Title:    fmt.Sprintf("Issue %d", i),
			Priority: issuestorage.PriorityMedium,
		})
		if err != nil {
			t.Fatalf("failed to create issue %d: %v", i, err)
		}
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    true,
	}

	cmd := newListCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	var issues []IssueListJSON
	if err := json.Unmarshal(out.Bytes(), &issues); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(issues) != 50 {
		t.Errorf("expected 50 issues (default limit), got %d", len(issues))
	}
}

func TestListCommand_LimitZeroReturnsAll(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	for i := 0; i < 5; i++ {
		_, err := store.Create(ctx, &issuestorage.Issue{
			Title:    fmt.Sprintf("Issue %d", i),
			Priority: issuestorage.PriorityMedium,
		})
		if err != nil {
			t.Fatalf("failed to create issue %d: %v", i, err)
		}
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    true,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--limit", "0"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	var issues []IssueListJSON
	if err := json.Unmarshal(out.Bytes(), &issues); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(issues) != 5 {
		t.Errorf("expected 5 issues (no limit), got %d", len(issues))
	}
}

func TestListCommand_CustomLimit(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)

	for i := 0; i < 5; i++ {
		_, err := store.Create(ctx, &issuestorage.Issue{
			Title:    fmt.Sprintf("Issue %d", i),
			Priority: issuestorage.PriorityMedium,
		})
		if err != nil {
			t.Fatalf("failed to create issue %d: %v", i, err)
		}
	}

	var out bytes.Buffer
	app := &App{
		Storage: rs,
		Out:     &out,
		JSON:    true,
	}

	cmd := newListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--limit", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	var issues []IssueListJSON
	if err := json.Unmarshal(out.Bytes(), &issues); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues (custom limit), got %d", len(issues))
	}
}
