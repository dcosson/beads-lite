package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSeedFormula writes a formula JSON file into the formulas dir
// under a temp directory. Returns the dir which acts as configDir.
func writeSeedFormula(t *testing.T, name, content string) string {
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

// writeSeedFormulas writes multiple formula JSON files into a single formulas dir.
func writeSeedFormulas(t *testing.T, formulas map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	formulaDir := filepath.Join(dir, "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		t.Fatalf("mkdir formulas: %v", err)
	}
	for name, content := range formulas {
		if err := os.WriteFile(filepath.Join(formulaDir, name+".formula.json"), []byte(content), 0o644); err != nil {
			t.Fatalf("write formula %s: %v", name, err)
		}
	}
	return dir
}

func TestMolSeedSingleFormulaText(t *testing.T) {
	configDir := writeSeedFormula(t, "deploy", `{
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
	out := app.Out.(*bytes.Buffer)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "✓ deploy") {
		t.Errorf("expected success checkmark for deploy, got:\n%s", output)
	}
}

func TestMolSeedSingleFormulaJSON(t *testing.T) {
	configDir := writeSeedFormula(t, "deploy", `{
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
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []SeedResult
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "deploy" {
		t.Errorf("Name = %q, want %q", results[0].Name, "deploy")
	}
	if !results[0].OK {
		t.Errorf("expected OK=true, got false: %s", results[0].Error)
	}
	if results[0].Source == "" {
		t.Error("Source should not be empty")
	}
}

func TestMolSeedMissingFormula(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigDir = t.TempDir() // no formulas

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing formula, got nil")
	}
	if !strings.Contains(err.Error(), "1 of 1 formulas failed") {
		t.Errorf("expected preflight failure error, got %v", err)
	}
}

func TestMolSeedMissingFormulaJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.ConfigDir = t.TempDir()
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"nonexistent"})
	// JSON mode still returns error for failed checks
	err := cmd.Execute()
	// In JSON mode the results are written before the error is returned
	if err != nil {
		// Error is expected, but JSON should still have been written
		_ = err
	}

	var results []SeedResult
	if jsonErr := json.Unmarshal(out.Bytes(), &results); jsonErr != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", jsonErr, out.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OK {
		t.Error("expected OK=false for missing formula")
	}
	if results[0].Error == "" {
		t.Error("expected non-empty Error for missing formula")
	}
}

func TestMolSeedNoArgsNoPatrol(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no args, got nil")
	}
	if !strings.Contains(err.Error(), "requires a formula name") {
		t.Errorf("expected 'requires a formula name' error, got %v", err)
	}
}

func TestMolSeedPatrolAllPresent(t *testing.T) {
	minimalFormula := `{
		"formula": "placeholder",
		"description": "test",
		"version": 1,
		"type": "workflow",
		"steps": [{"id": "s1", "title": "Step 1"}]
	}`

	formulas := map[string]string{
		"mol-deacon-patrol":   minimalFormula,
		"mol-witness-patrol":  minimalFormula,
		"mol-refinery-patrol": minimalFormula,
	}
	configDir := writeSeedFormulas(t, formulas)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	out := app.Out.(*bytes.Buffer)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--patrol"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "✓ All patrol formulas accessible") {
		t.Errorf("expected success message, got:\n%s", output)
	}
	if !strings.Contains(output, "✓ mol-deacon-patrol") {
		t.Errorf("expected deacon checkmark, got:\n%s", output)
	}
	if !strings.Contains(output, "✓ mol-witness-patrol") {
		t.Errorf("expected witness checkmark, got:\n%s", output)
	}
	if !strings.Contains(output, "✓ mol-refinery-patrol") {
		t.Errorf("expected refinery checkmark, got:\n%s", output)
	}
}

func TestMolSeedPatrolSomeMissing(t *testing.T) {
	// Isolate from real formula paths on the system.
	t.Setenv("GT_ROOT", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	minimalFormula := `{
		"formula": "placeholder",
		"description": "test",
		"version": 1,
		"type": "workflow",
		"steps": [{"id": "s1", "title": "Step 1"}]
	}`

	// Only provide one of the three patrol formulas.
	formulas := map[string]string{
		"mol-deacon-patrol": minimalFormula,
	}
	configDir := writeSeedFormulas(t, formulas)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	out := app.Out.(*bytes.Buffer)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--patrol"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing patrol formulas, got nil")
	}
	if !strings.Contains(err.Error(), "2 of 3 formulas failed") {
		t.Errorf("expected '2 of 3 formulas failed' error, got %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "✓ mol-deacon-patrol") {
		t.Errorf("expected deacon checkmark, got:\n%s", output)
	}
	if !strings.Contains(output, "✗ mol-witness-patrol") {
		t.Errorf("expected witness failure mark, got:\n%s", output)
	}
	if !strings.Contains(output, "✗ mol-refinery-patrol") {
		t.Errorf("expected refinery failure mark, got:\n%s", output)
	}
}

func TestMolSeedPatrolAllMissing(t *testing.T) {
	// Isolate from real formula paths on the system.
	t.Setenv("GT_ROOT", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	app, _ := setupTestApp(t)
	app.ConfigDir = t.TempDir() // no formulas

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--patrol"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for all missing patrol formulas, got nil")
	}
	if !strings.Contains(err.Error(), "3 of 3 formulas failed") {
		t.Errorf("expected '3 of 3 formulas failed' error, got %v", err)
	}
}

func TestMolSeedPatrolJSON(t *testing.T) {
	minimalFormula := `{
		"formula": "placeholder",
		"description": "test",
		"version": 1,
		"type": "workflow",
		"steps": [{"id": "s1", "title": "Step 1"}]
	}`

	formulas := map[string]string{
		"mol-deacon-patrol":   minimalFormula,
		"mol-witness-patrol":  minimalFormula,
		"mol-refinery-patrol": minimalFormula,
	}
	configDir := writeSeedFormulas(t, formulas)

	app, _ := setupTestApp(t)
	app.ConfigDir = configDir
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--patrol"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var results []SeedResult
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, out.String())
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("expected OK=true for %s, got false: %s", r.Name, r.Error)
		}
		if r.Source == "" {
			t.Errorf("expected non-empty Source for %s", r.Name)
		}
	}
}

func TestMolSeedWithVars(t *testing.T) {
	configDir := writeSeedFormula(t, "svc", `{
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

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"svc", "--var", "service=api-gateway"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "✓ svc") {
		t.Errorf("expected success checkmark for svc, got:\n%s", output)
	}
}

func TestMolSeedMissingRequiredVar(t *testing.T) {
	configDir := writeSeedFormula(t, "needs-var", `{
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

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"needs-var"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing required var, got nil")
	}
	if !strings.Contains(err.Error(), "1 of 1 formulas failed") {
		t.Errorf("expected preflight failure error, got %v", err)
	}
}

func TestMolSeedInvalidFormulaJSON(t *testing.T) {
	dir := t.TempDir()
	formulaDir := filepath.Join(dir, "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		t.Fatalf("mkdir formulas: %v", err)
	}
	// Write invalid JSON
	if err := os.WriteFile(filepath.Join(formulaDir, "broken.formula.json"), []byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("write formula: %v", err)
	}

	app, _ := setupTestApp(t)
	app.ConfigDir = dir

	cmd := newMolSeedCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"broken"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid formula, got nil")
	}
	if !strings.Contains(err.Error(), "1 of 1 formulas failed") {
		t.Errorf("expected preflight failure error, got %v", err)
	}
}

func TestMolSeedSubcommandRegistration(t *testing.T) {
	app, _ := setupTestApp(t)
	cmd := newMolCmd(NewTestProvider(app))

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "seed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'seed' subcommand to be registered on mol command")
	}
}
