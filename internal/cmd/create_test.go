package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func setupTestApp(t *testing.T) (*App, *filesystem.FilesystemStorage) {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	return &App{
		Storage: store,
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
	}, store
}

// extractCreatedID extracts the issue ID from create command output.
// The output format is:
//
//	✓ Created issue: bd-xxxx
//	  Title: ...
//	  Priority: ...
//	  Status: ...
func extractCreatedID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Created issue:") {
			parts := strings.Split(line, "Created issue:")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func TestCreateBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Fix login bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := extractCreatedID(out.String())
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
	if issue.Type != issuestorage.TypeTask {
		t.Errorf("expected type %q, got %q", issuestorage.TypeTask, issue.Type)
	}
	if issue.Priority != issuestorage.PriorityMedium {
		t.Errorf("expected priority %q, got %q", issuestorage.PriorityMedium, issue.Priority)
	}
}

func TestCreateWithType(t *testing.T) {
	tests := []struct {
		typeFlag string
		expected issuestorage.IssueType
	}{
		{"task", issuestorage.TypeTask},
		{"bug", issuestorage.TypeBug},
		{"feature", issuestorage.TypeFeature},
		{"epic", issuestorage.TypeEpic},
		{"chore", issuestorage.TypeChore},
		{"FEATURE", issuestorage.TypeFeature}, // test case insensitivity
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

			id := extractCreatedID(out.String())
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
		expected issuestorage.Priority
	}{
		{"0", issuestorage.PriorityCritical},
		{"p0", issuestorage.PriorityCritical},
		{"P0", issuestorage.PriorityCritical}, // test case insensitivity
		{"1", issuestorage.PriorityHigh},
		{"p1", issuestorage.PriorityHigh},
		{"2", issuestorage.PriorityMedium},
		{"p2", issuestorage.PriorityMedium},
		{"3", issuestorage.PriorityLow},
		{"p3", issuestorage.PriorityLow},
		{"4", issuestorage.PriorityBacklog},
		{"p4", issuestorage.PriorityBacklog},
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

			id := extractCreatedID(out.String())
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

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issue.Labels))
	}
	if issue.Labels[0] != "backend" || issue.Labels[1] != "urgent" {
		t.Errorf("expected labels [backend urgent], got %v", issue.Labels)
	}
}

func TestCreateWithLabelsFlag(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--labels", "urgent,backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issue.Labels))
	}
}

func TestCreateWithLabelAlias(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// --label (singular) is a hidden alias for --labels
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--label", "urgent,backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issue.Labels))
	}
}

func TestCreateWithLabelsAndLabelAliasCombined(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Both --labels and --label can be used together; values are merged
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--labels", "urgent,backend", "--label", "critical"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if len(issue.Labels) != 3 {
		t.Fatalf("expected 3 labels, got %d: %v", len(issue.Labels), issue.Labels)
	}
	// --labels values come first, then --label alias values are appended
	expected := []string{"urgent", "backend", "critical"}
	for i, want := range expected {
		if issue.Labels[i] != want {
			t.Errorf("label[%d]: expected %q, got %q", i, want, issue.Labels[i])
		}
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

	id := extractCreatedID(out.String())
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

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if issue.Description != "This is a detailed description" {
		t.Errorf("expected description %q, got %q", "This is a detailed description", issue.Description)
	}
}

func TestCreateWithParent(t *testing.T) {
	app, store := setupTestApp(t)

	// Create parent issue first
	parentIssue := &issuestorage.Issue{Title: "Parent epic"}
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

	childID := extractCreatedID(out.String())

	// Child should have dot-notation ID: parentID.1
	expectedChildID := parentID + ".1"
	if childID != expectedChildID {
		t.Errorf("expected child ID %q, got %q", expectedChildID, childID)
	}

	child, _ := store.Get(context.Background(), childID)
	if child.Parent != parentID {
		t.Errorf("expected parent %q, got %q", parentID, child.Parent)
	}

	// Verify parent has child in its children list
	parent, _ := store.Get(context.Background(), parentID)
	found := false
	for _, c := range parent.Children() {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("parent should have child in children list")
	}

	// Create a second child - should get .2
	out.Reset()
	cmd2 := newCreateCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"Second child", "--parent", parentID})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("create second child failed: %v", err)
	}

	secondChildID := extractCreatedID(out.String())
	expectedSecondID := parentID + ".2"
	if secondChildID != expectedSecondID {
		t.Errorf("expected second child ID %q, got %q", expectedSecondID, secondChildID)
	}
}

func TestCreateWithParentSubChildren(t *testing.T) {
	app, store := setupTestApp(t)

	// Create parent
	parentIssue := &issuestorage.Issue{Title: "Parent epic"}
	parentID, err := store.Create(context.Background(), parentIssue)
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create first child via --parent
	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Child task", "--parent", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create child failed: %v", err)
	}

	childID := extractCreatedID(out.String())
	expectedChildID := parentID + ".1"
	if childID != expectedChildID {
		t.Errorf("expected child ID %q, got %q", expectedChildID, childID)
	}

	// Create grandchild via --parent (child as parent)
	out.Reset()
	cmd2 := newCreateCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"Grandchild task", "--parent", childID})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("create grandchild failed: %v", err)
	}

	grandchildID := extractCreatedID(out.String())
	expectedGrandchildID := childID + ".1"
	if grandchildID != expectedGrandchildID {
		t.Errorf("expected grandchild ID %q, got %q", expectedGrandchildID, grandchildID)
	}

	// Verify the grandchild's parent is the child
	grandchild, _ := store.Get(context.Background(), grandchildID)
	if grandchild.Parent != childID {
		t.Errorf("grandchild parent: expected %q, got %q", childID, grandchild.Parent)
	}

	// Create second grandchild — should get .2
	out.Reset()
	cmd3 := newCreateCmd(NewTestProvider(app))
	cmd3.SetArgs([]string{"Second grandchild", "--parent", childID})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("create second grandchild failed: %v", err)
	}

	grandchild2ID := extractCreatedID(out.String())
	expectedGrandchild2ID := childID + ".2"
	if grandchild2ID != expectedGrandchild2ID {
		t.Errorf("expected second grandchild ID %q, got %q", expectedGrandchild2ID, grandchild2ID)
	}

	// Verify parent's children list has child
	parent, _ := store.Get(context.Background(), parentID)
	children := parent.Children()
	found := false
	for _, c := range children {
		if c == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("parent should have child %s in children list, got %v", childID, children)
	}

	// Verify child's children list has both grandchildren
	child, _ := store.Get(context.Background(), childID)
	grandchildren := child.Children()
	if len(grandchildren) != 2 {
		t.Errorf("expected child to have 2 grandchildren, got %d: %v", len(grandchildren), grandchildren)
	}
}

func TestCreateWithParentDepthLimit(t *testing.T) {
	app, store := setupTestApp(t)

	// Create root
	rootIssue := &issuestorage.Issue{Title: "Root"}
	rootID, err := store.Create(context.Background(), rootIssue)
	if err != nil {
		t.Fatalf("failed to create root: %v", err)
	}

	// Build chain to depth 3: root -> .1 -> .1.1 -> .1.1.1
	currentParent := rootID
	for depth := 1; depth <= 3; depth++ {
		out := app.Out.(*bytes.Buffer)
		out.Reset()
		cmd := newCreateCmd(NewTestProvider(app))
		cmd.SetArgs([]string{fmt.Sprintf("Depth %d", depth), "--parent", currentParent})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("create at depth %d failed: %v", depth, err)
		}
		currentParent = extractCreatedID(out.String())
	}

	// Depth 4 should fail
	out := app.Out.(*bytes.Buffer)
	out.Reset()
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Too deep", "--parent", currentParent})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error creating child at depth 4, got nil")
	}
}

func TestCreateWithDeps(t *testing.T) {
	app, store := setupTestApp(t)

	// Create dependency issue first
	depIssue := &issuestorage.Issue{Title: "Dependency"}
	depID, err := store.Create(context.Background(), depIssue)
	if err != nil {
		t.Fatalf("failed to create dependency: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "--deps", depID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	dependentID := extractCreatedID(out.String())
	dependent, _ := store.Get(context.Background(), dependentID)

	found := false
	for _, dep := range dependent.Dependencies {
		if dep.ID == depID {
			found = true
			if dep.Type != issuestorage.DepTypeBlocks {
				t.Errorf("expected dependency type %q, got %q", issuestorage.DepTypeBlocks, dep.Type)
			}
		}
	}
	if !found {
		t.Errorf("issue should have dependency in dependencies list")
	}

	// Verify dependency has dependent in its dependents list
	dep, _ := store.Get(context.Background(), depID)
	if !dep.HasDependent(dependentID) {
		t.Errorf("dependency should have dependent in dependents list")
	}
}

func TestCreateWithTypedDep(t *testing.T) {
	app, store := setupTestApp(t)

	depID, err := store.Create(context.Background(), &issuestorage.Issue{Title: "Dependency"})
	if err != nil {
		t.Fatalf("failed to create dependency: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "--deps", "tracks:" + depID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	dependentID := extractCreatedID(out.String())
	dependent, _ := store.Get(context.Background(), dependentID)

	found := false
	for _, dep := range dependent.Dependencies {
		if dep.ID == depID {
			found = true
			if dep.Type != issuestorage.DepTypeTracks {
				t.Errorf("expected dependency type %q, got %q", issuestorage.DepTypeTracks, dep.Type)
			}
		}
	}
	if !found {
		t.Errorf("expected dependency %s to be present", depID)
	}
}

func TestCreateWithDepsInvalidType(t *testing.T) {
	app, store := setupTestApp(t)

	depID, err := store.Create(context.Background(), &issuestorage.Issue{Title: "Dependency"})
	if err != nil {
		t.Fatalf("failed to create dependency: %v", err)
	}

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "--deps", "invalid:" + depID})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid dependency type")
	}
	if !strings.Contains(err.Error(), "invalid dependency type") {
		t.Fatalf("expected error about invalid dependency type, got: %v", err)
	}
}

func TestCreateWithDepsTooManyColons(t *testing.T) {
	app, store := setupTestApp(t)

	depID, err := store.Create(context.Background(), &issuestorage.Issue{Title: "Dependency"})
	if err != nil {
		t.Fatalf("failed to create dependency: %v", err)
	}

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "--deps", "tracks:" + depID + ":extra"})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid dependency format")
	}
	if !strings.Contains(err.Error(), "invalid dependency") {
		t.Fatalf("expected error about invalid dependency format, got: %v", err)
	}
}

func TestCreateWithMultipleDependencies(t *testing.T) {
	app, store := setupTestApp(t)

	// Create two dependency issues
	dep1, _ := store.Create(context.Background(), &issuestorage.Issue{Title: "Dep 1"})
	dep2, _ := store.Create(context.Background(), &issuestorage.Issue{Title: "Dep 2"})

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Dependent task", "-d", dep1, "-d", "tracks:" + dep2})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	dependentID := extractCreatedID(out.String())
	dependent, _ := store.Get(context.Background(), dependentID)

	if len(dependent.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(dependent.Dependencies))
	}
	for _, dep := range dependent.Dependencies {
		switch dep.ID {
		case dep1:
			if dep.Type != issuestorage.DepTypeBlocks {
				t.Errorf("expected dependency %s type %q, got %q", dep1, issuestorage.DepTypeBlocks, dep.Type)
			}
		case dep2:
			if dep.Type != issuestorage.DepTypeTracks {
				t.Errorf("expected dependency %s type %q, got %q", dep2, issuestorage.DepTypeTracks, dep.Type)
			}
		}
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

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	id := result["id"].(string)
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
	// Test that word priorities are rejected (must use 0-4 or P0-P4)
	invalidPriorities := []string{"invalid", "medium", "high", "low", "critical"}

	for _, priority := range invalidPriorities {
		t.Run(priority, func(t *testing.T) {
			app, _ := setupTestApp(t)

			cmd := newCreateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{"Test issue", "--priority", priority})
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for priority %q", priority)
			}
			if !strings.Contains(err.Error(), "invalid priority") {
				t.Errorf("expected error about invalid priority, got: %v", err)
			}
			if !strings.Contains(err.Error(), "not words like high/medium/low") {
				t.Errorf("expected error message to mention word restriction, got: %v", err)
			}
		})
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
	cmd.SetArgs([]string{"Test issue", "--deps", "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent dependency")
	}
}

// mapConfigStore is a minimal config.Store backed by a map, for tests.
type mapConfigStore struct {
	data map[string]string
}

func (m *mapConfigStore) Get(key string) (string, bool) {
	v, ok := m.data[key]
	return v, ok
}

func (m *mapConfigStore) Set(key, value string) error {
	m.data[key] = value
	return nil
}

func (m *mapConfigStore) SetInMemory(key, value string) {
	m.data[key] = value
}

func (m *mapConfigStore) Unset(key string) error {
	delete(m.data, key)
	return nil
}

func (m *mapConfigStore) All() map[string]string {
	cp := make(map[string]string, len(m.data))
	for k, v := range m.data {
		cp[k] = v
	}
	return cp
}

func TestCreateWithParent_ConfigMaxDepth(t *testing.T) {
	dir := t.TempDir()
	// Set max depth to 1 via the option (simulating config propagation)
	store := filesystem.New(dir, "bd-", filesystem.WithMaxHierarchyDepth(1))
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	cfg := &mapConfigStore{data: map[string]string{"hierarchy.max_depth": "1"}}
	app := &App{
		Storage:     store,
		ConfigStore: cfg,
		Out:         &bytes.Buffer{},
		Err:         &bytes.Buffer{},
	}
	out := app.Out.(*bytes.Buffer)

	// Create a root issue
	parentIssue := &issuestorage.Issue{Title: "Parent"}
	parentID, err := store.Create(context.Background(), parentIssue)
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child at depth 1 — should succeed
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Child", "--parent", parentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create child at depth 1 failed: %v", err)
	}
	childID := extractCreatedID(out.String())
	if childID != parentID+".1" {
		t.Errorf("expected child ID %q, got %q", parentID+".1", childID)
	}

	// Create grandchild at depth 2 — should fail (max=1)
	out.Reset()
	cmd2 := newCreateCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"Grandchild", "--parent", childID})
	err = cmd2.Execute()
	if err == nil {
		t.Fatal("expected error creating grandchild with max_depth=1")
	}
	if !errors.Is(err, issuestorage.ErrMaxDepthExceeded) {
		t.Errorf("expected ErrMaxDepthExceeded, got: %v", err)
	}
}

func TestCreateAllFlags(t *testing.T) {
	app, store := setupTestApp(t)

	// Create parent and dependency first
	parentID, _ := store.Create(context.Background(), &issuestorage.Issue{Title: "Parent"})
	depID, _ := store.Create(context.Background(), &issuestorage.Issue{Title: "Dependency"})

	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{
		"Full featured issue",
		"--type", "feature",
		"--priority", "1",
		"--parent", parentID,
		"--deps", depID,
		"--labels", "backend,api",
		"--assignee", "bob",
		"--description", "A comprehensive description",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := extractCreatedID(out.String())

	// With --parent, child ID should be dot-notation
	expectedID := parentID + ".1"
	if id != expectedID {
		t.Errorf("expected dot-notation ID %q, got %q", expectedID, id)
	}

	issue, _ := store.Get(context.Background(), id)

	if issue.Title != "Full featured issue" {
		t.Errorf("title mismatch")
	}
	if issue.Type != issuestorage.TypeFeature {
		t.Errorf("type mismatch: got %q", issue.Type)
	}
	if issue.Priority != issuestorage.PriorityHigh {
		t.Errorf("priority mismatch: got %q", issue.Priority)
	}
	if issue.Parent != parentID {
		t.Errorf("parent mismatch: got %q", issue.Parent)
	}
	// Should have 2 dependencies: parent-child (from --parent) and blocks (from --deps)
	if len(issue.Dependencies) != 2 || !issue.HasDependency(depID) || !issue.HasDependency(parentID) {
		t.Errorf("dependencies mismatch: got %v", issue.Dependencies)
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

func TestCreateRequireDescription_Enabled_NoDesc(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{
		"create.require-description": "true",
	}}

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when description is required but not provided")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Errorf("expected error about required description, got: %v", err)
	}
}

func TestCreateRequireDescription_Enabled_WithDesc(t *testing.T) {
	app, store := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{
		"create.require-description": "true",
	}}
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--description", "A valid description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create should succeed with description: %v", err)
	}

	id := extractCreatedID(out.String())
	issue, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Description != "A valid description" {
		t.Errorf("expected description %q, got %q", "A valid description", issue.Description)
	}
}

func TestCreateRequireDescription_Disabled(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{
		"create.require-description": "false",
	}}

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create should succeed when require-description is false: %v", err)
	}
}

func TestCreateRequireDescription_NotSet(t *testing.T) {
	app, _ := setupTestApp(t)
	// No ConfigStore set — default behavior, description not required

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create should succeed when config not set: %v", err)
	}
}

func TestCreateWithCustomType(t *testing.T) {
	app, store := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{
		"types.custom": "widget,gadget",
	}}
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test custom type", "--type", "widget"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create with custom type failed: %v", err)
	}

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if issue.Type != issuestorage.IssueType("widget") {
		t.Errorf("expected type %q, got %q", "widget", issue.Type)
	}
}

func TestCreateWithCustomTypeInvalid(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{
		"types.custom": "widget,gadget",
	}}

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--type", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	msg := err.Error()
	if !strings.Contains(msg, "widget") || !strings.Contains(msg, "gadget") {
		t.Errorf("error should list custom types: %s", msg)
	}
	if !strings.Contains(msg, "task") || !strings.Contains(msg, "bug") {
		t.Errorf("error should list built-in types: %s", msg)
	}
}

func TestCreateWithMolType(t *testing.T) {
	tests := []struct {
		molType  string
		expected issuestorage.MolType
	}{
		{"swarm", issuestorage.MolTypeSwarm},
		{"patrol", issuestorage.MolTypePatrol},
		{"work", issuestorage.MolTypeWork},
	}

	for _, tt := range tests {
		t.Run(tt.molType, func(t *testing.T) {
			app, store := setupTestApp(t)
			out := app.Out.(*bytes.Buffer)

			cmd := newCreateCmd(NewTestProvider(app))
			cmd.SetArgs([]string{"Test issue", "--mol-type", tt.molType})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("create failed: %v", err)
			}

			id := extractCreatedID(out.String())
			issue, _ := store.Get(context.Background(), id)
			if issue.MolType != tt.expected {
				t.Errorf("expected mol_type %q, got %q", tt.expected, issue.MolType)
			}
		})
	}
}

func TestCreateWithMolTypeDefault(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	id := extractCreatedID(out.String())
	issue, _ := store.Get(context.Background(), id)
	if issue.MolType != "" {
		t.Errorf("expected empty mol_type by default, got %q", issue.MolType)
	}
}

func TestCreateWithMolTypeInvalid(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--mol-type", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid mol-type")
	}
	if !strings.Contains(err.Error(), "invalid mol-type") {
		t.Errorf("expected error about invalid mol-type, got: %v", err)
	}
}

func TestCreateWithCustomID(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Custom ID issue", "--id", "bd-custom123"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create with --id failed: %v", err)
	}

	id := extractCreatedID(out.String())
	if id != "bd-custom123" {
		t.Errorf("expected id %q, got %q", "bd-custom123", id)
	}

	issue, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Title != "Custom ID issue" {
		t.Errorf("expected title %q, got %q", "Custom ID issue", issue.Title)
	}
}

func TestCreateWithCustomID_NoHyphen(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--id", "nohyphen"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for ID without hyphen")
	}
	if !strings.Contains(err.Error(), "must contain a hyphen") {
		t.Errorf("expected error about hyphen, got: %v", err)
	}
}

func TestCreateWithCustomID_WrongPrefix(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--id", "xx-abc"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for wrong prefix")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("expected error about prefix, got: %v", err)
	}
}

func TestCreateWithCustomID_AllowedPrefix(t *testing.T) {
	app, store := setupTestApp(t)
	app.ConfigStore = &mapConfigStore{data: map[string]string{
		"allowed_prefixes": "other,proj",
	}}
	out := app.Out.(*bytes.Buffer)

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Allowed prefix issue", "--id", "other-abc"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create with allowed prefix failed: %v", err)
	}

	id := extractCreatedID(out.String())
	if id != "other-abc" {
		t.Errorf("expected id %q, got %q", "other-abc", id)
	}

	issue, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Title != "Allowed prefix issue" {
		t.Errorf("expected title %q, got %q", "Allowed prefix issue", issue.Title)
	}
}

func TestCreateWithCustomID_AlreadyExists(t *testing.T) {
	app, store := setupTestApp(t)

	// Create an issue with explicit ID first
	existing := &issuestorage.Issue{ID: "bd-exist1", Title: "Existing"}
	if _, err := store.Create(context.Background(), existing); err != nil {
		t.Fatalf("failed to create existing issue: %v", err)
	}

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Duplicate", "--id", "bd-exist1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when creating with duplicate ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected error about already exists, got: %v", err)
	}
}

func TestCreateWithCustomID_AndParent(t *testing.T) {
	app, store := setupTestApp(t)

	parentID, _ := store.Create(context.Background(), &issuestorage.Issue{Title: "Parent"})

	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--id", "bd-custom", "--parent", parentID})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --id and --parent are set")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("expected error about --id and --parent conflict, got: %v", err)
	}
}

func TestCreateWithCustomID_Force(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Without --force, wrong prefix should fail
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--id", "xx-abc"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error without --force")
	}

	// With --force, wrong prefix should succeed
	out.Reset()
	cmd2 := newCreateCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"Forced issue", "--id", "xx-abc", "--force"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("create with --force failed: %v", err)
	}

	id := extractCreatedID(out.String())
	if id != "xx-abc" {
		t.Errorf("expected id %q, got %q", "xx-abc", id)
	}

	issue, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Title != "Forced issue" {
		t.Errorf("expected title %q, got %q", "Forced issue", issue.Title)
	}
}

func TestCreateWithCustomID_ForceNoHyphen(t *testing.T) {
	app, _ := setupTestApp(t)

	// --force should NOT bypass the hyphen requirement (it's a format check, not a prefix check)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Test issue", "--id", "nohyphen", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error even with --force when ID has no hyphen")
	}
	if !strings.Contains(err.Error(), "must contain a hyphen") {
		t.Errorf("expected error about hyphen, got: %v", err)
	}
}

func TestCreateWithCustomID_TombstoneResurrection(t *testing.T) {
	app, store := setupTestApp(t)

	// Create and tombstone an issue
	original := &issuestorage.Issue{ID: "bd-tomb1", Title: "Original"}
	if _, err := store.Create(context.Background(), original); err != nil {
		t.Fatalf("failed to create original issue: %v", err)
	}
	if err := store.CreateTombstone(context.Background(), "bd-tomb1", "test", "testing"); err != nil {
		t.Fatalf("failed to tombstone issue: %v", err)
	}

	// Verify it's tombstoned
	tombstoned, err := store.Get(context.Background(), "bd-tomb1")
	if err != nil {
		t.Fatalf("failed to get tombstoned issue: %v", err)
	}
	if tombstoned.Status != issuestorage.StatusTombstone {
		t.Fatalf("expected tombstone status, got %q", tombstoned.Status)
	}

	// Create with same ID should succeed (resurrect)
	out := app.Out.(*bytes.Buffer)
	cmd := newCreateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"Resurrected issue", "--id", "bd-tomb1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create with tombstoned ID failed: %v", err)
	}

	id := extractCreatedID(out.String())
	if id != "bd-tomb1" {
		t.Errorf("expected id %q, got %q", "bd-tomb1", id)
	}

	// The new issue should be open with the new title
	resurrected, err := store.Get(context.Background(), "bd-tomb1")
	if err != nil {
		t.Fatalf("failed to get resurrected issue: %v", err)
	}
	if resurrected.Title != "Resurrected issue" {
		t.Errorf("expected title %q, got %q", "Resurrected issue", resurrected.Title)
	}
	if resurrected.Status != issuestorage.StatusOpen {
		t.Errorf("expected status %q, got %q", issuestorage.StatusOpen, resurrected.Status)
	}
}
