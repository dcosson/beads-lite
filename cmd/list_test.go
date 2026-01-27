package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

func setupTestApp(t *testing.T) (*App, func()) {
	t.Helper()
	dir := t.TempDir()
	fs := filesystem.New(dir)
	if err := fs.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	app := &App{
		Storage: fs,
		Out:     out,
		Err:     errOut,
	}

	return app, func() {}
}

func createTestIssue(t *testing.T, app *App, title string, opts ...func(*storage.Issue)) string {
	t.Helper()
	issue := &storage.Issue{
		Title:    title,
		Type:     storage.TypeTask,
		Priority: storage.PriorityMedium,
	}
	for _, opt := range opts {
		opt(issue)
	}
	id, err := app.Storage.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	return id
}

func TestListCmd_DefaultListsOpenIssues(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create some open issues
	id1 := createTestIssue(t, app, "Open issue 1")
	id2 := createTestIssue(t, app, "Open issue 2")

	// Create and close one issue
	id3 := createTestIssue(t, app, "Closed issue")
	if err := app.Storage.Close(context.Background(), id3); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain open issues
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain open issue %s, got: %s", id1, out)
	}
	if !strings.Contains(out, id2) {
		t.Errorf("output should contain open issue %s, got: %s", id2, out)
	}

	// Should not contain closed issue
	if strings.Contains(out, id3) {
		t.Errorf("output should not contain closed issue %s, got: %s", id3, out)
	}
}

func TestListCmd_AllIncludesClosedIssues(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create an open issue
	id1 := createTestIssue(t, app, "Open issue")

	// Create and close an issue
	id2 := createTestIssue(t, app, "Closed issue")
	if err := app.Storage.Close(context.Background(), id2); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain both issues
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain open issue %s, got: %s", id1, out)
	}
	if !strings.Contains(out, id2) {
		t.Errorf("output should contain closed issue %s, got: %s", id2, out)
	}
}

func TestListCmd_ClosedOnlyShowsClosedIssues(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create an open issue
	id1 := createTestIssue(t, app, "Open issue")

	// Create and close an issue
	id2 := createTestIssue(t, app, "Closed issue")
	if err := app.Storage.Close(context.Background(), id2); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--closed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should not contain open issue
	if strings.Contains(out, id1) {
		t.Errorf("output should not contain open issue %s, got: %s", id1, out)
	}

	// Should contain closed issue
	if !strings.Contains(out, id2) {
		t.Errorf("output should contain closed issue %s, got: %s", id2, out)
	}
}

func TestListCmd_FilterByType(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create issues of different types
	id1 := createTestIssue(t, app, "Bug issue", func(i *storage.Issue) {
		i.Type = storage.TypeBug
	})
	id2 := createTestIssue(t, app, "Feature issue", func(i *storage.Issue) {
		i.Type = storage.TypeFeature
	})

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--type", "bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain bug
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain bug %s, got: %s", id1, out)
	}

	// Should not contain feature
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain feature %s, got: %s", id2, out)
	}
}

func TestListCmd_FilterByPriority(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create issues of different priorities
	id1 := createTestIssue(t, app, "High priority issue", func(i *storage.Issue) {
		i.Priority = storage.PriorityHigh
	})
	id2 := createTestIssue(t, app, "Low priority issue", func(i *storage.Issue) {
		i.Priority = storage.PriorityLow
	})

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--priority", "high"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain high priority
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain high priority issue %s, got: %s", id1, out)
	}

	// Should not contain low priority
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain low priority issue %s, got: %s", id2, out)
	}
}

func TestListCmd_FilterByLabel(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create issues with different labels
	id1 := createTestIssue(t, app, "Backend issue", func(i *storage.Issue) {
		i.Labels = []string{"backend", "urgent"}
	})
	id2 := createTestIssue(t, app, "Frontend issue", func(i *storage.Issue) {
		i.Labels = []string{"frontend"}
	})

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--label", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain backend issue
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain backend issue %s, got: %s", id1, out)
	}

	// Should not contain frontend issue
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain frontend issue %s, got: %s", id2, out)
	}
}

func TestListCmd_FilterByMultipleLabels(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create issues with different label combinations
	id1 := createTestIssue(t, app, "Has both labels", func(i *storage.Issue) {
		i.Labels = []string{"backend", "urgent"}
	})
	id2 := createTestIssue(t, app, "Has only backend", func(i *storage.Issue) {
		i.Labels = []string{"backend"}
	})

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--label", "backend", "--label", "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain issue with both labels
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain issue with both labels %s, got: %s", id1, out)
	}

	// Should not contain issue with only one label
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain issue with only backend label %s, got: %s", id2, out)
	}
}

func TestListCmd_FilterByAssignee(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create issues with different assignees
	id1 := createTestIssue(t, app, "Alice's issue", func(i *storage.Issue) {
		i.Assignee = "alice"
	})
	id2 := createTestIssue(t, app, "Bob's issue", func(i *storage.Issue) {
		i.Assignee = "bob"
	})

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--assignee", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain alice's issue
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain alice's issue %s, got: %s", id1, out)
	}

	// Should not contain bob's issue
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain bob's issue %s, got: %s", id2, out)
	}
}

func TestListCmd_FilterByParent(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create parent issue
	parentID := createTestIssue(t, app, "Parent issue")

	// Create child of parent
	id1 := createTestIssue(t, app, "Child issue")
	if err := app.Storage.SetParent(context.Background(), id1, parentID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create unrelated issue
	id2 := createTestIssue(t, app, "Unrelated issue")

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--parent", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain child issue
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain child issue %s, got: %s", id1, out)
	}

	// Should not contain unrelated issue
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain unrelated issue %s, got: %s", id2, out)
	}

	// Should not contain parent itself
	if strings.Contains(out, parentID) {
		t.Errorf("output should not contain parent %s, got: %s", parentID, out)
	}
}

func TestListCmd_RootsOnlyShowsRootIssues(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create parent (root) issue
	id1 := createTestIssue(t, app, "Root issue")

	// Create child issue
	id2 := createTestIssue(t, app, "Child issue")
	if err := app.Storage.SetParent(context.Background(), id2, id1); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--roots"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain root issue
	if !strings.Contains(out, id1) {
		t.Errorf("output should contain root issue %s, got: %s", id1, out)
	}

	// Should not contain child issue
	if strings.Contains(out, id2) {
		t.Errorf("output should not contain child issue %s, got: %s", id2, out)
	}
}

func TestListCmd_FormatIds(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create some issues
	id1 := createTestIssue(t, app, "Issue 1")
	id2 := createTestIssue(t, app, "Issue 2")

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--format", "ids"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// Should have exactly 2 lines (one per issue)
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %s", len(lines), out)
	}

	// Each line should be just an ID
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "bd-") {
			t.Errorf("expected line to be an ID, got: %s", line)
		}
	}

	// Both IDs should be present
	if !strings.Contains(out, id1) || !strings.Contains(out, id2) {
		t.Errorf("expected both IDs %s and %s, got: %s", id1, id2, out)
	}
}

func TestListCmd_FormatLong(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	// Create an issue with all fields
	id := createTestIssue(t, app, "Test issue", func(i *storage.Issue) {
		i.Type = storage.TypeBug
		i.Priority = storage.PriorityHigh
		i.Labels = []string{"backend", "urgent"}
		i.Assignee = "alice"
		i.Description = "This is a test description"
	})

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{"--format", "long"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should contain various details
	if !strings.Contains(out, id) {
		t.Errorf("output should contain ID %s, got: %s", id, out)
	}
	if !strings.Contains(out, "Test issue") {
		t.Errorf("output should contain title, got: %s", out)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output should contain type, got: %s", out)
	}
	if !strings.Contains(out, "high") {
		t.Errorf("output should contain priority, got: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("output should contain assignee, got: %s", out)
	}
	if !strings.Contains(out, "backend") {
		t.Errorf("output should contain labels, got: %s", out)
	}
	if !strings.Contains(out, "This is a test description") {
		t.Errorf("output should contain description, got: %s", out)
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	app.JSON = true

	// Create an issue
	id := createTestIssue(t, app, "JSON test issue")

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).Bytes()

	// Should be valid JSON
	var issues []*storage.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Should contain our issue
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ID != id {
		t.Errorf("expected issue ID %s, got %s", id, issues[0].ID)
	}
}

func TestListCmd_EmptyList(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	cmd := NewListCmd(app)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := app.Out.(*bytes.Buffer).String()

	// Should be empty (no issues)
	if out != "" {
		t.Errorf("expected empty output, got: %s", out)
	}
}
