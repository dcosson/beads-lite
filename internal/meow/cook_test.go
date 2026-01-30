package meow

import "testing"

func TestCookReturnsCorrectStructure(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "deploy", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build artifacts", "type": "task", "priority": "high", "labels": ["ci"]},
			{"id": "test",  "title": "Run tests", "depends_on": ["build"]},
			{"id": "ship",  "title": "Ship to prod", "depends_on": ["test"], "assignee": "alice"}
		]
	}`)

	result, err := Cook("deploy", nil, sp)
	if err != nil {
		t.Fatalf("Cook() error = %v", err)
	}

	// Root should mirror formula description.
	if result.Root.Title != "Deploy pipeline" {
		t.Errorf("Root.Title = %q, want %q", result.Root.Title, "Deploy pipeline")
	}
	if result.Root.Type != "epic" {
		t.Errorf("Root.Type = %q, want %q", result.Root.Type, "epic")
	}

	// Should have 3 steps.
	if len(result.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(result.Steps))
	}

	// Verify step fields.
	build := result.Steps[0]
	if build.StepID != "build" {
		t.Errorf("Steps[0].StepID = %q, want %q", build.StepID, "build")
	}
	if build.Title != "Build artifacts" {
		t.Errorf("Steps[0].Title = %q, want %q", build.Title, "Build artifacts")
	}
	if build.Type != "task" {
		t.Errorf("Steps[0].Type = %q, want %q", build.Type, "task")
	}
	if build.Priority != "high" {
		t.Errorf("Steps[0].Priority = %q, want %q", build.Priority, "high")
	}
	if len(build.Labels) != 1 || build.Labels[0] != "ci" {
		t.Errorf("Steps[0].Labels = %v, want [ci]", build.Labels)
	}

	// Verify depends_on uses step IDs.
	test := result.Steps[1]
	if len(test.DependsOn) != 1 || test.DependsOn[0] != "build" {
		t.Errorf("Steps[1].DependsOn = %v, want [build]", test.DependsOn)
	}

	ship := result.Steps[2]
	if ship.Assignee != "alice" {
		t.Errorf("Steps[2].Assignee = %q, want %q", ship.Assignee, "alice")
	}
	if len(ship.DependsOn) != 1 || ship.DependsOn[0] != "test" {
		t.Errorf("Steps[2].DependsOn = %v, want [test]", ship.DependsOn)
	}
}

func TestCookSubstitutesVariables(t *testing.T) {
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

	result, err := Cook("vars", map[string]string{"service": "api-gateway"}, sp)
	if err != nil {
		t.Fatalf("Cook() error = %v", err)
	}

	if result.Root.Title != "Deploy api-gateway" {
		t.Errorf("Root.Title = %q, want %q", result.Root.Title, "Deploy api-gateway")
	}
	if result.Steps[0].Title != "Build api-gateway" {
		t.Errorf("Steps[0].Title = %q, want %q", result.Steps[0].Title, "Build api-gateway")
	}
	if result.Steps[0].Description != "Build for staging" {
		t.Errorf("Steps[0].Description = %q, want %q", result.Steps[0].Description, "Build for staging")
	}
}

func TestCookNoSideEffects(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "noop", `{
		"formula": "noop",
		"description": "No side effects",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "Step 1"},
			{"id": "s2", "title": "Step 2"}
		]
	}`)

	// Cook should succeed without any storage backend.
	result, err := Cook("noop", nil, sp)
	if err != nil {
		t.Fatalf("Cook() error = %v", err)
	}

	// Basic sanity — no storage was needed.
	if len(result.Steps) != 2 {
		t.Errorf("len(Steps) = %d, want 2", len(result.Steps))
	}
}

func TestCookMissingRequiredVarFails(t *testing.T) {
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

	_, err := Cook("required", nil, sp)
	if err == nil {
		t.Fatal("Cook() should fail when required var is missing")
	}
}

func TestCookRootTitleFallsBackToFormulaName(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "no-desc", `{
		"formula": "no-desc",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "Step 1"}
		]
	}`)

	result, err := Cook("no-desc", nil, sp)
	if err != nil {
		t.Fatalf("Cook() error = %v", err)
	}

	if result.Root.Title != "no-desc" {
		t.Errorf("Root.Title = %q, want %q (formula name fallback)", result.Root.Title, "no-desc")
	}
}

func TestCookDefaultStepTypeIsTask(t *testing.T) {
	dir := t.TempDir()
	sp := writeTestFormula(t, dir, "default-type", `{
		"formula": "default-type",
		"description": "Default type",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "No explicit type"}
		]
	}`)

	result, err := Cook("default-type", nil, sp)
	if err != nil {
		t.Fatalf("Cook() error = %v", err)
	}

	if result.Steps[0].Type != "task" {
		t.Errorf("Steps[0].Type = %q, want %q", result.Steps[0].Type, "task")
	}
}
