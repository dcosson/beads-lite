package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/meow"
)

// writeCookFormula writes a formula JSON file into the formulas dir
// under a temp directory, matching DefaultSearchPath layout.
// Returns the dir which acts as configDir (the .beads directory).
func writeCookFormula(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	formulaDir := filepath.Join(dir, "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		t.Fatalf("mkdir formulas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(formulaDir, name+".formula.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write formula: %v", err)
	}
	return dir
}

func TestCookTextOutput(t *testing.T) {
	configDir := writeCookFormula(t, "deploy", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build artifacts", "type": "task"},
			{"id": "test",  "title": "Run tests", "depends_on": ["build"]},
			{"id": "ship",  "title": "Ship to prod", "depends_on": ["test"]}
		]
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	out := app.Out.(*bytes.Buffer)

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()

	// Root line
	if !strings.Contains(output, "Root: Deploy pipeline (epic)") {
		t.Errorf("expected root line, got:\n%s", output)
	}

	// Steps header
	if !strings.Contains(output, "Steps (3):") {
		t.Errorf("expected steps count, got:\n%s", output)
	}

	// Step entries with type and title
	if !strings.Contains(output, "[task] build: Build artifacts") {
		t.Errorf("expected build step, got:\n%s", output)
	}
	if !strings.Contains(output, "[task] test: Run tests") {
		t.Errorf("expected test step, got:\n%s", output)
	}
	if !strings.Contains(output, "[task] ship: Ship to prod") {
		t.Errorf("expected ship step, got:\n%s", output)
	}

	// Dependency display
	if !strings.Contains(output, "depends on: build") {
		t.Errorf("expected dependency on build, got:\n%s", output)
	}
	if !strings.Contains(output, "depends on: test") {
		t.Errorf("expected dependency on test, got:\n%s", output)
	}
}

func TestCookJSONOutput(t *testing.T) {
	configDir := writeCookFormula(t, "deploy", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build artifacts", "type": "task"},
			{"id": "test",  "title": "Run tests", "depends_on": ["build"]}
		]
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meow.CookResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}

	if result.Root.Title != "Deploy pipeline" {
		t.Errorf("Root.Title = %q, want %q", result.Root.Title, "Deploy pipeline")
	}
	if result.Root.Type != "epic" {
		t.Errorf("Root.Type = %q, want %q", result.Root.Type, "epic")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(result.Steps))
	}
	if result.Steps[0].StepID != "build" {
		t.Errorf("Steps[0].StepID = %q, want %q", result.Steps[0].StepID, "build")
	}
}

func TestCookWithVars(t *testing.T) {
	configDir := writeCookFormula(t, "svc", `{
		"formula": "svc",
		"description": "Deploy {{service}}",
		"version": 1,
		"type": "workflow",
		"vars": {
			"service": {"required": true}
		},
		"steps": [
			{"id": "s1", "title": "Build {{service}}"}
		]
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	out := app.Out.(*bytes.Buffer)

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"svc", "--var", "service=api-gateway"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Deploy api-gateway") {
		t.Errorf("expected substituted root title, got:\n%s", output)
	}
	if !strings.Contains(output, "Build api-gateway") {
		t.Errorf("expected substituted step title, got:\n%s", output)
	}
}

func TestCookMissingFormula(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigDir = t.TempDir() // no formulas

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing formula, got nil")
	}
	if !strings.Contains(err.Error(), "cook nonexistent") {
		t.Errorf("expected 'cook nonexistent' error, got %v", err)
	}
}

func TestCookMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestCookMissingRequiredVar(t *testing.T) {
	configDir := writeCookFormula(t, "needs-var", `{
		"formula": "needs-var",
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

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"needs-var"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing required var, got nil")
	}
	if !strings.Contains(err.Error(), "cook needs-var") {
		t.Errorf("expected 'cook needs-var' error, got %v", err)
	}
}

func TestCookNoSteps(t *testing.T) {
	configDir := writeCookFormula(t, "empty", `{
		"formula": "empty",
		"description": "Empty formula",
		"version": 1,
		"type": "workflow",
		"steps": []
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	out := app.Out.(*bytes.Buffer)

	cmd := newCookCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"empty"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Root: Empty formula (epic)") {
		t.Errorf("expected root line, got:\n%s", output)
	}
	if !strings.Contains(output, "(no steps)") {
		t.Errorf("expected '(no steps)' message, got:\n%s", output)
	}
}
