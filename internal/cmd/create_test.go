package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads2/internal/storage/filesystem"
	"beads2/internal/storage"
)

func setupTestApp(t *testing.T) (*App, *filesystem.FilesystemStorage) {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	return &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}, store
}

func TestCreateBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Fix login bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := strings.TrimSpace(out.String())
	if !strings.HasPrefix(id, "bd-") {
		t.Errorf("expected id to start with bd-, got %q", id)
	}

	// Verify issue was created
	issue, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Title != "Fix login bug" {
		t.Errorf("expected title %q, got %q", "Fix login bug", issue.Title)
	}
	if issue.Type != storage.TypeTask {
		t.Errorf("expected type %q, got %q", storage.TypeTask, issue.Type)
	}
	if issue.Priority != storage.PriorityMedium {
		t.Errorf("expected priority %q, got %q", storage.PriorityMedium, issue.Priority)
	}
}

func TestCreateWithType(t *testing.T) {
	tests := []struct {
		typeFlag string
		expected storage.IssueType
	}{
		{"task", storage.TypeTask},
		{"bug", storage.TypeBug},
		{"feature", storage.TypeFeature},
		{"epic", storage.TypeEpic},
		{"chore", storage.TypeChore},
		{"FEATURE", storage.TypeFeature}, // test case insensitivity
	}

	for _, tt := range tests {
		t.Run(tt.typeFlag, func(t *testing.T) {
			app, store := setupTestApp(t)
			out := app.Out.(*bytes.Buffer)

			cmd := newCreateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{"Test issue", "--type", tt.typeFlag})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("create failed: %v", err)
			}

			id := strings.TrimSpace(out.String())
			issue, _ := store.Get(context.Background(), id)
			if issue.Type != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, issue.Type)
			}
		})
	}
}

func TestCreateWithPriority(t *testing.T) {
	tests := []struct {
		priority string
		expected storage.Priority
	}{
		{"critical", storage.PriorityCritical},
		{"high", storage.PriorityHigh},
		{"medium", storage.PriorityMedium},
		{"low", storage.PriorityLow},
		{"HIGH", storage.PriorityHigh}, // test case insensitivity
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			app, store := setupTestApp(t)
			out := app.Out.(*bytes.Buffer)

			cmd := newCreateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{"Test issue", "--priority", tt.priority})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("create failed: %v", err)
			}

			id := strings.TrimSpace(out.String())
			issue, _ := store.Get(context.Background(), id)
			if issue.Priority != tt.expected {
				t.Errorf("expected priority %q, got %q", tt.expected, issue.Priority)
			}
		})
	}
}

func TestCreateWithLabels(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "-l", "backend", "-l", "urgent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := strings.TrimSpace(out.String())
	issue, _ := store.Get(context.Background(), id)
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issue.Labels))
	}
	if issue.Labels[0] != "backend" || issue.Labels[1] != "urgent" {
		t.Errorf("expected labels [backend urgent], got %v", issue.Labels)
	}
}

func TestCreateWithAssignee(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--assignee", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := strings.TrimSpace(out.String())
	issue, _ := store.Get(context.Background(), id)
	if issue.Assignee != "alice" {
		t.Errorf("expected assignee %q, got %q", "alice", issue.Assignee)
	}
}

func TestCreateWithDescription(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--description", "This is a detailed description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := strings.TrimSpace(out.String())
	issue, _ := store.Get(context.Background(), id)
	if issue.Description != "This is a detailed description" {
		t.Errorf("expected description %q, got %q", "This is a detailed description", issue.Description)
	}
}

func TestCreateWithParent(t *testing.T) {
	app, store := setupTestApp(t)

	// Create parent issue first
	parentIssue := &storage.Issue{Title: "Parent epic"}
	parentID, err := store.Create(context.Background(), parentIssue)
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Child task", "--parent", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	childID := strings.TrimSpace(out.String())
	child, _ := store.Get(context.Background(), childID)
	if child.Parent != parentID {
		t.Errorf("expected parent %q, got %q", parentID, child.Parent)
	}

	// Verify parent has child in its children list
	parent, _ := store.Get(context.Background(), parentID)
	found := false
	for _, c := range parent.Children {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("parent should have child in children list")
	}
}

func TestCreateWithDependsOn(t *testing.T) {
	app, store := setupTestApp(t)

	// Create dependency issue first
	depIssue := &storage.Issue{Title: "Dependency"}
	depID, err := store.Create(context.Background(), depIssue)
	if err != nil {
		t.Fatalf("failed to create dependency: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "--depends-on", depID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	dependentID := strings.TrimSpace(out.String())
	dependent, _ := store.Get(context.Background(), dependentID)

	found := false
	for _, d := range dependent.DependsOn {
		if d == depID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("issue should have dependency in depends_on list")
	}

	// Verify dependency has dependent in its dependents list
	dep, _ := store.Get(context.Background(), depID)
	found = false
	for _, d := range dep.Dependents {
		if d == dependentID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("dependency should have dependent in dependents list")
	}
}

func TestCreateWithMultipleDependencies(t *testing.T) {
	app, store := setupTestApp(t)

	// Create two dependency issues
	dep1, _ := store.Create(context.Background(), &storage.Issue{Title: "Dep 1"})
	dep2, _ := store.Create(context.Background(), &storage.Issue{Title: "Dep 2"})

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "-d", dep1, "-d", dep2})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	dependentID := strings.TrimSpace(out.String())
	dependent, _ := store.Get(context.Background(), dependentID)

	if len(dependent.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(dependent.DependsOn))
	}
}

func TestCreateWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	id := result["id"]
	if !strings.HasPrefix(id, "bd-") {
		t.Errorf("expected id to start with bd-, got %q", id)
	}

	// Verify issue exists
	_, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
}

func TestCreateInvalidType(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--type", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("expected error about invalid type, got: %v", err)
	}
}

func TestCreateInvalidPriority(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--priority", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid priority")
	}
	if !strings.Contains(err.Error(), "invalid priority") {
		t.Errorf("expected error about invalid priority, got: %v", err)
	}
}

func TestCreateNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no title provided")
	}
}

func TestCreateParentNotFound(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--parent", "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent parent")
	}
}

func TestCreateDependencyNotFound(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--depends-on", "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent dependency")
	}
}

func TestCreateAllFlags(t *testing.T) {
	app, store := setupTestApp(t)

	// Create parent and dependency first
	parentID, _ := store.Create(context.Background(), &storage.Issue{Title: "Parent"})
	depID, _ := store.Create(context.Background(), &storage.Issue{Title: "Dependency"})

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{
		"Full featured issue",
		"--type", "feature",
		"--priority", "high",
		"--parent", parentID,
		"--depends-on", depID,
		"--label", "backend",
		"--label", "api",
		"--assignee", "bob",
		"--description", "A comprehensive description",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := strings.TrimSpace(out.String())
	issue, _ := store.Get(context.Background(), id)

	if issue.Title != "Full featured issue" {
		t.Errorf("title mismatch")
	}
	if issue.Type != storage.TypeFeature {
		t.Errorf("type mismatch: got %q", issue.Type)
	}
	if issue.Priority != storage.PriorityHigh {
		t.Errorf("priority mismatch: got %q", issue.Priority)
	}
	if issue.Parent != parentID {
		t.Errorf("parent mismatch: got %q", issue.Parent)
	}
	if len(issue.DependsOn) != 1 || issue.DependsOn[0] != depID {
		t.Errorf("depends_on mismatch: got %v", issue.DependsOn)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("labels mismatch: got %v", issue.Labels)
	}
	if issue.Assignee != "bob" {
		t.Errorf("assignee mismatch: got %q", issue.Assignee)
	}
	if issue.Description != "A comprehensive description" {
		t.Errorf("description mismatch: got %q", issue.Description)
	}
}
