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

// writeFormulaFile writes a formula file into the formulas dir under a temp
// directory matching DefaultSearchPath layout. Returns the dir (configDir).
func writeFormulaFile(t *testing.T, name, ext, content string) string {
	t.Helper()
	dir := t.TempDir()
	formulaDir := filepath.Join(dir, "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		t.Fatalf("mkdir formulas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(formulaDir, name+".formula."+ext), []byte(content), 0o644); err != nil {
		t.Fatalf("write formula: %v", err)
	}
	return dir
}

// writeMultipleFormulas writes multiple formula files into the same configDir.
func writeMultipleFormulas(t *testing.T, formulas map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	formulaDir := filepath.Join(dir, "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		t.Fatalf("mkdir formulas: %v", err)
	}
	for filename, content := range formulas {
		if err := os.WriteFile(filepath.Join(formulaDir, filename), []byte(content), 0o644); err != nil {
			t.Fatalf("write formula %s: %v", filename, err)
		}
	}
	return dir
}

func TestFormulaListText(t *testing.T) {
	configDir := writeMultipleFormulas(t, map[string]string{
		"deploy.formula.json": `{
			"formula": "deploy",
			"description": "Deploy pipeline",
			"version": 1,
			"type": "workflow",
			"phase": "liquid",
			"steps": [{"id": "s1", "title": "Build"}]
		}`,
		"triage.formula.toml": `formula = "triage"
description = "Bug triage"
version = 1
type = "expansion"
`,
	})

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.FormulaSearchPath{filepath.Join(configDir, "formulas")}
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Formulas (2):") {
		t.Errorf("expected 'Formulas (2):', got:\n%s", output)
	}
	if !strings.Contains(output, "deploy") {
		t.Errorf("expected 'deploy' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "triage") {
		t.Errorf("expected 'triage' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Deploy pipeline") {
		t.Errorf("expected description in output, got:\n%s", output)
	}
	if !strings.Contains(output, "workflow") {
		t.Errorf("expected type in output, got:\n%s", output)
	}
	if !strings.Contains(output, "[liquid]") {
		t.Errorf("expected phase in output, got:\n%s", output)
	}
}

func TestFormulaListJSON(t *testing.T) {
	configDir := writeFormulaFile(t, "deploy", "json", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow"
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.FormulaSearchPath{filepath.Join(configDir, "formulas")}
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entries []meow.FormulaEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "deploy" {
		t.Errorf("Name = %q, want %q", entries[0].Name, "deploy")
	}
	if entries[0].Type != meow.FormulaTypeWorkflow {
		t.Errorf("Type = %q, want %q", entries[0].Type, meow.FormulaTypeWorkflow)
	}
}

func TestFormulaListEmpty(t *testing.T) {
	app, _ := setupTestApp(t)
	emptyDir := t.TempDir()
	app.ConfigDir = emptyDir
	app.FormulaPath = meow.FormulaSearchPath{filepath.Join(emptyDir, "formulas")}
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "No formulas found.") {
		t.Errorf("expected 'No formulas found.', got:\n%s", out.String())
	}
}

func TestFormulaListPriority(t *testing.T) {
	// Create two search path dirs: project-level shadows user-level.
	projectDir := t.TempDir()
	userDir := t.TempDir()
	projectFormulas := filepath.Join(projectDir, "formulas")
	userFormulas := filepath.Join(userDir, "formulas")
	if err := os.MkdirAll(projectFormulas, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userFormulas, 0o755); err != nil {
		t.Fatal(err)
	}

	// Same name in both dirs with different descriptions.
	projectContent := `{"formula": "shared", "description": "project-level", "version": 1, "type": "workflow"}`
	if err := os.WriteFile(filepath.Join(projectFormulas, "shared.formula.json"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}
	userContent := `{"formula": "shared", "description": "user-level", "version": 2, "type": "workflow"}`
	if err := os.WriteFile(filepath.Join(userFormulas, "shared.formula.json"), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _ := setupTestApp(t)
	app.FormulaPath = meow.FormulaSearchPath{projectFormulas, userFormulas}
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaListCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entries []meow.FormulaEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (project shadows user), got %d", len(entries))
	}
	if entries[0].Description != "project-level" {
		t.Errorf("Description = %q, want %q (project should shadow user)", entries[0].Description, "project-level")
	}
}

func TestFormulaShowText(t *testing.T) {
	configDir := writeFormulaFile(t, "deploy", "json", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"phase": "liquid",
		"vars": {
			"env": {"description": "Target environment", "required": true, "enum": ["staging", "prod"]},
			"version": {"description": "Release version", "default": "latest"}
		},
		"steps": [
			{"id": "build", "title": "Build artifacts", "type": "task"},
			{"id": "test", "title": "Run tests", "depends_on": ["build"]},
			{"id": "ship", "title": "Ship to prod", "depends_on": ["test"]}
		]
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.DefaultSearchPath(configDir)
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()

	// Header
	if !strings.Contains(output, "Formula: deploy") {
		t.Errorf("expected formula name, got:\n%s", output)
	}
	if !strings.Contains(output, "Description: Deploy pipeline") {
		t.Errorf("expected description, got:\n%s", output)
	}
	if !strings.Contains(output, "Type:        workflow") {
		t.Errorf("expected type, got:\n%s", output)
	}
	if !strings.Contains(output, "Phase:       liquid") {
		t.Errorf("expected phase, got:\n%s", output)
	}
	if !strings.Contains(output, "Version:     1") {
		t.Errorf("expected version, got:\n%s", output)
	}

	// Variables
	if !strings.Contains(output, "Variables (2):") {
		t.Errorf("expected 'Variables (2):', got:\n%s", output)
	}
	if !strings.Contains(output, "env") {
		t.Errorf("expected 'env' variable, got:\n%s", output)
	}
	if !strings.Contains(output, "(required)") {
		t.Errorf("expected '(required)' marker, got:\n%s", output)
	}
	if !strings.Contains(output, "enum: staging, prod") {
		t.Errorf("expected enum values, got:\n%s", output)
	}
	if !strings.Contains(output, "default: latest") {
		t.Errorf("expected default value, got:\n%s", output)
	}

	// Steps
	if !strings.Contains(output, "Steps (3):") {
		t.Errorf("expected 'Steps (3):', got:\n%s", output)
	}
	if !strings.Contains(output, "[task] build: Build artifacts") {
		t.Errorf("expected build step, got:\n%s", output)
	}
	if !strings.Contains(output, "depends on: build") {
		t.Errorf("expected dependency, got:\n%s", output)
	}
}

func TestFormulaShowJSON(t *testing.T) {
	configDir := writeFormulaFile(t, "deploy", "json", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build artifacts", "type": "task"}
		]
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.DefaultSearchPath(configDir)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var formula meow.Formula
	if err := json.Unmarshal(out.Bytes(), &formula); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if formula.Formula != "deploy" {
		t.Errorf("Formula = %q, want %q", formula.Formula, "deploy")
	}
	if len(formula.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(formula.Steps))
	}
}

func TestFormulaShowResolvedInheritance(t *testing.T) {
	dir := t.TempDir()
	formulaDir := filepath.Join(dir, "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Parent formula
	parent := `{
		"formula": "base",
		"description": "Base workflow",
		"version": 1,
		"type": "workflow",
		"vars": {"team": {"description": "Team name"}},
		"steps": [{"id": "setup", "title": "Setup environment"}]
	}`
	if err := os.WriteFile(filepath.Join(formulaDir, "base.formula.json"), []byte(parent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Child formula that extends parent
	child := `{
		"formula": "deploy",
		"description": "Deploy workflow",
		"version": 1,
		"type": "workflow",
		"extends": ["base"],
		"steps": [{"id": "ship", "title": "Ship it"}]
	}`
	if err := os.WriteFile(filepath.Join(formulaDir, "deploy.formula.json"), []byte(child), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _ := setupTestApp(t)
	app.ConfigDir = dir
	app.FormulaPath = meow.DefaultSearchPath(dir)
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	// Should show the extends chain
	if !strings.Contains(output, "Extends:     base") {
		t.Errorf("expected 'Extends: base', got:\n%s", output)
	}
	// Resolved formula should include parent's step
	if !strings.Contains(output, "setup") {
		t.Errorf("expected inherited 'setup' step, got:\n%s", output)
	}
	// And the child's step
	if !strings.Contains(output, "ship") {
		t.Errorf("expected 'ship' step, got:\n%s", output)
	}
	// Parent's var should be inherited
	if !strings.Contains(output, "team") {
		t.Errorf("expected inherited 'team' variable, got:\n%s", output)
	}
}

func TestFormulaShowMissing(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigDir = t.TempDir()
	app.FormulaPath = meow.DefaultSearchPath(app.ConfigDir)

	cmd := newFormulaShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing formula, got nil")
	}
	if !strings.Contains(err.Error(), "formula show") {
		t.Errorf("expected 'formula show' error, got %v", err)
	}
}

func TestFormulaShowMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newFormulaShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestFormulaConvertJSONtoTOML(t *testing.T) {
	configDir := writeFormulaFile(t, "deploy", "json", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "build", "title": "Build artifacts"}
		]
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.DefaultSearchPath(configDir)
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaConvertCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Converted:") {
		t.Errorf("expected 'Converted:' output, got:\n%s", output)
	}
	if !strings.Contains(output, ".formula.toml") {
		t.Errorf("expected TOML destination in output, got:\n%s", output)
	}

	// Verify the TOML file was created and is valid.
	tomlPath := filepath.Join(configDir, "formulas", "deploy.formula.toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("failed to read converted TOML file: %v", err)
	}
	if !strings.Contains(string(data), `formula = "deploy"`) {
		t.Errorf("TOML output should contain formula name, got:\n%s", data)
	}

	// Round-trip: load the converted TOML and verify fields.
	f, err := meow.LoadFormula("deploy", meow.FormulaSearchPath{filepath.Join(configDir, "formulas")})
	if err != nil {
		t.Fatalf("failed to load converted formula: %v", err)
	}
	if f.Formula != "deploy" {
		t.Errorf("round-trip Formula = %q, want %q", f.Formula, "deploy")
	}
	if f.Description != "Deploy pipeline" {
		t.Errorf("round-trip Description = %q, want %q", f.Description, "Deploy pipeline")
	}
}

func TestFormulaConvertTOMLtoJSON(t *testing.T) {
	configDir := writeFormulaFile(t, "triage", "toml", `formula = "triage"
description = "Bug triage"
version = 1
type = "expansion"

[[steps]]
id = "s1"
title = "Classify bug"
`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.DefaultSearchPath(configDir)
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaConvertCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"triage"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, ".formula.json") {
		t.Errorf("expected JSON destination in output, got:\n%s", output)
	}

	// Verify the JSON file was created and is valid.
	jsonPath := filepath.Join(configDir, "formulas", "triage.formula.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read converted JSON file: %v", err)
	}

	var f meow.Formula
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("converted JSON is invalid: %v\nraw: %s", err, data)
	}
	if f.Formula != "triage" {
		t.Errorf("round-trip Formula = %q, want %q", f.Formula, "triage")
	}
	if f.Type != meow.FormulaTypeExpansion {
		t.Errorf("round-trip Type = %q, want %q", f.Type, meow.FormulaTypeExpansion)
	}
}

func TestFormulaConvertJSON(t *testing.T) {
	configDir := writeFormulaFile(t, "deploy", "json", `{
		"formula": "deploy",
		"description": "Deploy pipeline",
		"version": 1,
		"type": "workflow"
	}`)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.FormulaPath = meow.DefaultSearchPath(configDir)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newFormulaConvertCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if result["from_format"] != "json" {
		t.Errorf("from_format = %q, want %q", result["from_format"], "json")
	}
	if !strings.HasSuffix(result["destination"], ".formula.toml") {
		t.Errorf("destination should end with .formula.toml, got %q", result["destination"])
	}
}

func TestFormulaConvertMissing(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigDir = t.TempDir()
	app.FormulaPath = meow.DefaultSearchPath(app.ConfigDir)

	cmd := newFormulaConvertCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing formula, got nil")
	}
	if !strings.Contains(err.Error(), "formula convert") {
		t.Errorf("expected 'formula convert' error, got %v", err)
	}
}

func TestFormulaConvertMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newFormulaConvertCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestFormulaParentCommand(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newFormulaCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from formula parent command: %v", err)
	}
}

func TestFormulaSubcommandRegistration(t *testing.T) {
	app, _ := setupTestApp(t)
	cmd := newFormulaCmd(NewTestProvider(app))

	expectedSubs := []string{"list", "show", "convert"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found on formula command", name)
		}
	}
}
