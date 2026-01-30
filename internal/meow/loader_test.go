package meow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFormulaJSON(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"formula": "test-workflow",
		"description": "A test workflow",
		"version": 1,
		"type": "workflow",
		"vars": {
			"version": {
				"description": "The semantic version",
				"required": true
			}
		},
		"steps": [
			{"id": "s1", "title": "First step"}
		]
	}`
	if err := os.WriteFile(filepath.Join(dir, "test-workflow.formula.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := LoadFormula("test-workflow", FormulaSearchPath{dir})
	if err != nil {
		t.Fatalf("LoadFormula() error = %v", err)
	}

	if f.Formula != "test-workflow" {
		t.Errorf("Formula = %q, want %q", f.Formula, "test-workflow")
	}
	if f.Description != "A test workflow" {
		t.Errorf("Description = %q, want %q", f.Description, "A test workflow")
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want 1", f.Version)
	}
	if f.Type != FormulaTypeWorkflow {
		t.Errorf("Type = %q, want %q", f.Type, FormulaTypeWorkflow)
	}
	if len(f.Vars) != 1 {
		t.Fatalf("len(Vars) = %d, want 1", len(f.Vars))
	}
	v := f.Vars["version"]
	if v == nil {
		t.Fatal("Vars[\"version\"] is nil")
	}
	if v.Description != "The semantic version" {
		t.Errorf("Vars[\"version\"].Description = %q, want %q", v.Description, "The semantic version")
	}
	if !v.Required {
		t.Error("Vars[\"version\"].Required = false, want true")
	}
	if len(f.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(f.Steps))
	}
	if f.Steps[0].ID != "s1" {
		t.Errorf("Steps[0].ID = %q, want %q", f.Steps[0].ID, "s1")
	}
}

func TestLoadFormulaTOML(t *testing.T) {
	dir := t.TempDir()
	content := `formula = "test-toml"
description = "A TOML formula"
version = 1
type = "expansion"

[vars.version]
description = "The semantic version"
required = true

[[steps]]
id = "s1"
title = "First step"
`
	if err := os.WriteFile(filepath.Join(dir, "test-toml.formula.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := LoadFormula("test-toml", FormulaSearchPath{dir})
	if err != nil {
		t.Fatalf("LoadFormula() error = %v", err)
	}

	if f.Formula != "test-toml" {
		t.Errorf("Formula = %q, want %q", f.Formula, "test-toml")
	}
	if f.Type != FormulaTypeExpansion {
		t.Errorf("Type = %q, want %q", f.Type, FormulaTypeExpansion)
	}
	if len(f.Vars) != 1 {
		t.Fatalf("len(Vars) = %d, want 1", len(f.Vars))
	}
	v := f.Vars["version"]
	if v == nil {
		t.Fatal("Vars[\"version\"] is nil")
	}
	if v.Description != "The semantic version" {
		t.Errorf("Vars[\"version\"].Description = %q, want %q", v.Description, "The semantic version")
	}
	if !v.Required {
		t.Error("Vars[\"version\"].Required = false, want true")
	}
	if len(f.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(f.Steps))
	}
}

func TestSearchPathPriority(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()

	// Project-level formula (higher priority)
	projectContent := `{"formula": "shared", "description": "project-level", "version": 1, "type": "workflow"}`
	if err := os.WriteFile(filepath.Join(projectDir, "shared.formula.json"), []byte(projectContent), 0644); err != nil {
		t.Fatal(err)
	}

	// User-level formula (lower priority)
	userContent := `{"formula": "shared", "description": "user-level", "version": 2, "type": "workflow"}`
	if err := os.WriteFile(filepath.Join(userDir, "shared.formula.json"), []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := LoadFormula("shared", FormulaSearchPath{projectDir, userDir})
	if err != nil {
		t.Fatalf("LoadFormula() error = %v", err)
	}

	if f.Description != "project-level" {
		t.Errorf("Description = %q, want %q (project-level should shadow user-level)", f.Description, "project-level")
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want 1 (project-level should shadow user-level)", f.Version)
	}
}

func TestMissingFormulaError(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadFormula("nonexistent", FormulaSearchPath{dir})
	if err == nil {
		t.Fatal("expected error for missing formula, got nil")
	}

	want := `formula "nonexistent" not found in search path`
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error = %q, want prefix %q", got, want)
	}
}

func TestMalformedFormulaError(t *testing.T) {
	t.Run("malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bad.formula.json"), []byte(`{not json`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadFormula("bad", FormulaSearchPath{dir})
		if err == nil {
			t.Fatal("expected error for malformed JSON, got nil")
		}

		// Error should mention the file path
		filePath := filepath.Join(dir, "bad.formula.json")
		if got := err.Error(); !contains(got, filePath) {
			t.Errorf("error = %q, want it to contain file path %q", got, filePath)
		}
	})

	t.Run("malformed TOML", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bad.formula.toml"), []byte(`[[[invalid`), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadFormula("bad", FormulaSearchPath{dir})
		if err == nil {
			t.Fatal("expected error for malformed TOML, got nil")
		}

		filePath := filepath.Join(dir, "bad.formula.toml")
		if got := err.Error(); !contains(got, filePath) {
			t.Errorf("error = %q, want it to contain file path %q", got, filePath)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
