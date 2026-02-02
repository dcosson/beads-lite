package meow

import (
	"bytes"
	"context"
	"os"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

// newTestStore returns an initialised FilesystemStorage backed by a temp dir.
func newTestStore(t *testing.T) issuestorage.IssueStore {
	t.Helper()
	dir := t.TempDir()
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

// writeTestFormula writes a JSON formula file to dir and returns the search path.
func writeTestFormula(t *testing.T, dir, name, content string) FormulaSearchPath {
	t.Helper()
	writeFormula(t, dir, name, content)
	return FormulaSearchPath{dir}
}

func TestPourCreatesRootAndChildren(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "deploy", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build artifacts"},
			{"id": "test",  "title": "Run tests"},
			{"id": "ship",  "title": "Ship to prod"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "deploy",
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour() error = %v", err)
	}

	// Root should be an epic.
	root, err := store.Get(ctx, result.NewEpicID)
	if err != nil {
		t.Fatalf("Get root: %v", err)
	}
	if root.Type != issuestorage.TypeEpic {
		t.Errorf("root.Type = %q, want %q", root.Type, issuestorage.TypeEpic)
	}
	if root.Title != "deploy" {
		t.Errorf("root.Title = %q, want %q", root.Title, "deploy")
	}
	if root.Description != "Deploy pipeline" {
		t.Errorf("root.Description = %q, want %q", root.Description, "Deploy pipeline")
	}

	// Should have 3 children (plus 1 root = 4 entries in IDMapping).
	if result.Created != 4 {
		t.Fatalf("Created = %d, want 4", result.Created)
	}

	// Each child should have parent-child dep to root.
	for key, childID := range result.IDMapping {
		if childID == result.NewEpicID {
			continue // skip root entry
		}
		child, err := store.Get(ctx, childID)
		if err != nil {
			t.Fatalf("Get child %s: %v", key, err)
		}
		if child.Parent != result.NewEpicID {
			t.Errorf("child %s.Parent = %q, want %q", key, child.Parent, result.NewEpicID)
		}
	}
}

func TestPourChildrenAreParentChildDeps(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "simple", `{
		"formula": "simple",
		"description": "Simple formula",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "a", "title": "Step A"},
			{"id": "b", "title": "Step B"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "simple",
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour() error = %v", err)
	}

	// Verify root's dependents include parent-child for each child.
	root, err := store.Get(ctx, result.NewEpicID)
	if err != nil {
		t.Fatalf("Get root: %v", err)
	}
	children := root.Children()
	if len(children) != 2 {
		t.Fatalf("root.Children() = %v, want 2 entries", children)
	}
}

func TestPourDependsOnBecomesBlocksDep(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "pipeline", `{
		"formula": "pipeline",
		"description": "Pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build"},
			{"id": "test",  "title": "Test", "depends_on": ["build"]},
			{"id": "ship",  "title": "Ship", "depends_on": ["test"]}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "pipeline",
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour() error = %v", err)
	}

	buildID := result.IDMapping["pipeline.build"]
	testID := result.IDMapping["pipeline.test"]
	shipID := result.IDMapping["pipeline.ship"]

	// test should have a blocks dep on build.
	testIssue, err := store.Get(ctx, testID)
	if err != nil {
		t.Fatalf("Get test issue: %v", err)
	}
	if !testIssue.HasDependency(buildID) {
		t.Errorf("test issue should depend on build issue; deps = %v", testIssue.Dependencies)
	}

	// ship should have a blocks dep on test.
	shipIssue, err := store.Get(ctx, shipID)
	if err != nil {
		t.Fatalf("Get ship issue: %v", err)
	}
	if !shipIssue.HasDependency(testID) {
		t.Errorf("ship issue should depend on test issue; deps = %v", shipIssue.Dependencies)
	}
}

func TestPourWispSetsEphemeral(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "ephemeral", `{
		"formula": "ephemeral",
		"description": "Ephemeral formula",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "Step 1"},
			{"id": "s2", "title": "Step 2"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "ephemeral",
		Ephemeral:   true,
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour(Ephemeral=true) error = %v", err)
	}

	// Root should be ephemeral.
	root, err := store.Get(ctx, result.NewEpicID)
	if err != nil {
		t.Fatalf("Get root: %v", err)
	}
	if !root.Ephemeral {
		t.Error("root.Ephemeral = false, want true")
	}

	// Children should be ephemeral.
	for key, childID := range result.IDMapping {
		if childID == result.NewEpicID {
			continue
		}
		child, err := store.Get(ctx, childID)
		if err != nil {
			t.Fatalf("Get child %s: %v", key, err)
		}
		if !child.Ephemeral {
			t.Errorf("child %s.Ephemeral = false, want true", key)
		}
	}
}

func TestPourSubstitutesVariables(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "vars", `{
		"formula": "vars",
		"description": "Deploy {{service}}",
		"version": 1,
		"type": "workflow",
		"vars": {
			"service": {"required": true},
			"env":     {"default": "staging"}
		},
		"steps": [
			{"id": "s1", "title": "Build {{service}}", "description": "Build for {{env}}"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "vars",
		Vars:        map[string]string{"service": "api-gateway"},
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour() error = %v", err)
	}

	child, err := store.Get(ctx, result.IDMapping["vars.s1"])
	if err != nil {
		t.Fatalf("Get child: %v", err)
	}
	if child.Title != "Build api-gateway" {
		t.Errorf("child.Title = %q, want %q", child.Title, "Build api-gateway")
	}
	if child.Description != "Build for staging" {
		t.Errorf("child.Description = %q, want %q", child.Description, "Build for staging")
	}
}

func TestPourMissingRequiredVarFails(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "required", `{
		"formula": "required",
		"description": "Needs vars",
		"version": 1,
		"type": "workflow",
		"vars": {
			"name": {"required": true}
		},
		"steps": [
			{"id": "s1", "title": "Hello {{name}}"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	_, err := Pour(ctx, store, PourOptions{
		FormulaName: "required",
		SearchPath:  sp,
	})
	if err == nil {
		t.Fatal("Pour() should fail when required var is missing")
	}

	// No issues should have been created.
	issues, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues after validation failure, got %d", len(issues))
	}
}

func TestPourVaporFormulaWarnsOnStderr(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "vapor", `{
		"formula": "vapor",
		"description": "Vapor formula",
		"version": 1,
		"type": "workflow",
		"phase": "vapor",
		"steps": [
			{"id": "s1", "title": "Step 1"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	// Capture stderr.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	_, err := Pour(ctx, store, PourOptions{
		FormulaName: "vapor",
		SearchPath:  sp,
	})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("Pour() error = %v (should succeed with warning)", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()
	if stderr == "" {
		t.Error("expected warning on stderr for vapor formula + pour, got nothing")
	}
}

func TestPourVaporFormulaNoWarningForWisp(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "vapor-wisp", `{
		"formula": "vapor-wisp",
		"description": "Vapor formula",
		"version": 1,
		"type": "workflow",
		"phase": "vapor",
		"steps": [
			{"id": "s1", "title": "Step 1"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	// Capture stderr.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	_, err := Pour(ctx, store, PourOptions{
		FormulaName: "vapor-wisp",
		Ephemeral:   true,
		SearchPath:  sp,
	})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("Pour(Ephemeral=true) error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()
	if stderr != "" {
		t.Errorf("expected no warning for wisp on vapor formula, got %q", stderr)
	}
}

func TestPourUnknownDependsOnStepFails(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "bad-dep", `{
		"formula": "bad-dep",
		"description": "Bad dep",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "Step 1", "depends_on": ["nonexistent"]}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	_, err := Pour(ctx, store, PourOptions{
		FormulaName: "bad-dep",
		SearchPath:  sp,
	})
	if err == nil {
		t.Fatal("Pour() should fail when depends_on references unknown step")
	}
}

func TestPourRootTitleFallsBackToFormulaName(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "no-desc", `{
		"formula": "no-desc",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "Step 1"}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "no-desc",
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour() error = %v", err)
	}

	root, err := store.Get(ctx, result.NewEpicID)
	if err != nil {
		t.Fatalf("Get root: %v", err)
	}
	if root.Title != "no-desc" {
		t.Errorf("root.Title = %q, want %q (formula name fallback)", root.Title, "no-desc")
	}
}

func TestPourSetsStepFields(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "fields", `{
		"formula": "fields",
		"description": "Field test",
		"version": 1,
		"type": "workflow",
		"steps": [
			{
				"id": "s1",
				"title": "Detailed step",
				"description": "Do the thing",
				"type": "bug",
				"priority": "high",
				"labels": ["backend", "urgent"],
				"assignee": "alice"
			}
		]
	}`)

	store := newTestStore(t)
	ctx := context.Background()

	result, err := Pour(ctx, store, PourOptions{
		FormulaName: "fields",
		SearchPath:  sp,
	})
	if err != nil {
		t.Fatalf("Pour() error = %v", err)
	}

	child, err := store.Get(ctx, result.IDMapping["fields.s1"])
	if err != nil {
		t.Fatalf("Get child: %v", err)
	}
	if child.Type != issuestorage.TypeBug {
		t.Errorf("Type = %q, want %q", child.Type, issuestorage.TypeBug)
	}
	if child.Priority != issuestorage.PriorityHigh {
		t.Errorf("Priority = %q, want %q", child.Priority, issuestorage.PriorityHigh)
	}
	if len(child.Labels) != 2 || child.Labels[0] != "backend" || child.Labels[1] != "urgent" {
		t.Errorf("Labels = %v, want [backend urgent]", child.Labels)
	}
	if child.Assignee != "alice" {
		t.Errorf("Assignee = %q, want %q", child.Assignee, "alice")
	}
	if child.Description != "Do the thing" {
		t.Errorf("Description = %q, want %q", child.Description, "Do the thing")
	}
}
