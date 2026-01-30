package meow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFormula writes a JSON formula file to dir.
func writeFormula(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".formula.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveFormulaSingle(t *testing.T) {
	dir := t.TempDir()
	writeFormula(t, dir, "simple", `{
		"formula": "simple",
		"description": "A simple formula",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "Do something", "needs": ["s0"]}
		]
	}`)

	f, err := ResolveFormula("simple", FormulaSearchPath{dir})
	if err != nil {
		t.Fatalf("ResolveFormula() error = %v", err)
	}

	if f.Formula != "simple" {
		t.Errorf("Formula = %q, want %q", f.Formula, "simple")
	}
	if len(f.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(f.Steps))
	}
	// needs should be merged into depends_on
	if len(f.Steps[0].DependsOn) != 1 || f.Steps[0].DependsOn[0] != "s0" {
		t.Errorf("Steps[0].DependsOn = %v, want [s0]", f.Steps[0].DependsOn)
	}
	if f.Steps[0].Needs != nil {
		t.Errorf("Steps[0].Needs = %v, want nil (should be cleared)", f.Steps[0].Needs)
	}
}

func TestResolveFormulaTwoLevelInheritance(t *testing.T) {
	dir := t.TempDir()

	writeFormula(t, dir, "grandparent", `{
		"formula": "grandparent",
		"description": "Grand",
		"version": 1,
		"type": "workflow",
		"vars": {
			"env": {"description": "Environment", "default": "staging"}
		},
		"steps": [
			{"id": "gp1", "title": "Grandparent step"}
		]
	}`)

	writeFormula(t, dir, "parent", `{
		"formula": "parent",
		"description": "Parent",
		"version": 1,
		"type": "workflow",
		"extends": ["grandparent"],
		"vars": {
			"env": {"description": "Environment override", "default": "prod"},
			"region": {"description": "Region", "default": "us-east-1"}
		},
		"steps": [
			{"id": "p1", "title": "Parent step"}
		]
	}`)

	writeFormula(t, dir, "child", `{
		"formula": "child",
		"description": "Child",
		"version": 1,
		"type": "workflow",
		"extends": ["parent"],
		"vars": {
			"region": {"description": "Child region", "default": "eu-west-1"}
		},
		"steps": [
			{"id": "c1", "title": "Child step"}
		]
	}`)

	f, err := ResolveFormula("child", FormulaSearchPath{dir})
	if err != nil {
		t.Fatalf("ResolveFormula() error = %v", err)
	}

	// Steps: grandparent, parent, child (appended in order).
	if len(f.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(f.Steps))
	}
	if f.Steps[0].ID != "gp1" {
		t.Errorf("Steps[0].ID = %q, want %q", f.Steps[0].ID, "gp1")
	}
	if f.Steps[1].ID != "p1" {
		t.Errorf("Steps[1].ID = %q, want %q", f.Steps[1].ID, "p1")
	}
	if f.Steps[2].ID != "c1" {
		t.Errorf("Steps[2].ID = %q, want %q", f.Steps[2].ID, "c1")
	}

	// Vars: child overrides parent which overrides grandparent.
	if f.Vars["env"].Default != "prod" {
		t.Errorf("Vars[env].Default = %q, want %q (parent overrides grandparent)", f.Vars["env"].Default, "prod")
	}
	if f.Vars["region"].Default != "eu-west-1" {
		t.Errorf("Vars[region].Default = %q, want %q (child overrides parent)", f.Vars["region"].Default, "eu-west-1")
	}

	// Extends should be cleared after resolution.
	if f.Extends != nil {
		t.Errorf("Extends = %v, want nil after resolution", f.Extends)
	}
}

func TestResolveFormulaCycleDetection(t *testing.T) {
	dir := t.TempDir()

	writeFormula(t, dir, "alpha", `{
		"formula": "alpha",
		"version": 1,
		"type": "workflow",
		"extends": ["beta"]
	}`)

	writeFormula(t, dir, "beta", `{
		"formula": "beta",
		"version": 1,
		"type": "workflow",
		"extends": ["alpha"]
	}`)

	_, err := ResolveFormula("alpha", FormulaSearchPath{dir})
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want it to mention cycle", err.Error())
	}
}

func TestValidateVarsMissingRequired(t *testing.T) {
	f := &Formula{
		Vars: map[string]*VarDef{
			"assignee": {Required: true},
			"repo_url": {Required: true},
			"version":  {Required: true, Default: "1.0.0"},
		},
	}

	err := ValidateVars(f, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing required vars, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "assignee") {
		t.Errorf("error should mention assignee: %q", msg)
	}
	if !strings.Contains(msg, "repo_url") {
		t.Errorf("error should mention repo_url: %q", msg)
	}
	// version has a default, so it should NOT be missing.
	if strings.Contains(msg, "version") {
		t.Errorf("error should not mention version (has default): %q", msg)
	}
}

func TestValidateVarsEnumConstraint(t *testing.T) {
	f := &Formula{
		Vars: map[string]*VarDef{
			"priority": {Enum: []string{"P1", "P2", "P3"}},
		},
	}

	// Valid value.
	if err := ValidateVars(f, map[string]string{"priority": "P2"}); err != nil {
		t.Errorf("valid enum value should not error: %v", err)
	}

	// Invalid value.
	err := ValidateVars(f, map[string]string{"priority": "P9"})
	if err == nil {
		t.Fatal("expected error for invalid enum value, got nil")
	}
	if !strings.Contains(err.Error(), "P9") {
		t.Errorf("error should mention invalid value: %q", err.Error())
	}
}

func TestValidateVarsPatternConstraint(t *testing.T) {
	f := &Formula{
		Vars: map[string]*VarDef{
			"version": {Pattern: `^\d+\.\d+\.\d+$`},
		},
	}

	// Valid value.
	if err := ValidateVars(f, map[string]string{"version": "1.2.3"}); err != nil {
		t.Errorf("valid pattern value should not error: %v", err)
	}

	// Invalid value.
	err := ValidateVars(f, map[string]string{"version": "abc"})
	if err == nil {
		t.Fatal("expected error for invalid pattern value, got nil")
	}
	if !strings.Contains(err.Error(), "abc") {
		t.Errorf("error should mention invalid value: %q", err.Error())
	}
}

func TestSubstituteVarsReplacement(t *testing.T) {
	f := &Formula{
		Formula: "test",
		Version: 1,
		Type:    FormulaTypeWorkflow,
		Vars: map[string]*VarDef{
			"project": {Default: "myproject"},
			"env":     {},
		},
		Steps: []*Step{
			{
				ID:          "s1",
				Title:       "Deploy {{project}} to {{env}}",
				Description: "Deploying {{project}} in {{env}} environment",
				Assignee:    "{{assignee}}",
			},
		},
	}

	result := SubstituteVars(f, map[string]string{
		"env":      "production",
		"assignee": "alice",
	})

	// Original should be unmodified.
	if f.Steps[0].Title != "Deploy {{project}} to {{env}}" {
		t.Error("SubstituteVars mutated the original formula")
	}

	// Check substitutions.
	if result.Steps[0].Title != "Deploy myproject to production" {
		t.Errorf("Title = %q, want %q", result.Steps[0].Title, "Deploy myproject to production")
	}
	if result.Steps[0].Description != "Deploying myproject in production environment" {
		t.Errorf("Description = %q, want %q", result.Steps[0].Description, "Deploying myproject in production environment")
	}
	if result.Steps[0].Assignee != "alice" {
		t.Errorf("Assignee = %q, want %q", result.Steps[0].Assignee, "alice")
	}
}

func TestSubstituteVarsUnknownLeftAsIs(t *testing.T) {
	f := &Formula{
		Formula: "test",
		Version: 1,
		Type:    FormulaTypeWorkflow,
		Steps: []*Step{
			{
				ID:    "s1",
				Title: "Deploy to {{env}} with {{unknown_var}}",
			},
		},
	}

	result := SubstituteVars(f, map[string]string{"env": "staging"})

	want := "Deploy to staging with {{unknown_var}}"
	if result.Steps[0].Title != want {
		t.Errorf("Title = %q, want %q", result.Steps[0].Title, want)
	}
}

func TestSubstituteVarsChildren(t *testing.T) {
	f := &Formula{
		Formula: "test",
		Version: 1,
		Type:    FormulaTypeWorkflow,
		Steps: []*Step{
			{
				ID:    "parent",
				Title: "Parent {{name}}",
				Children: []*Step{
					{ID: "child", Title: "Child {{name}}"},
				},
			},
		},
	}

	result := SubstituteVars(f, map[string]string{"name": "Alice"})

	if result.Steps[0].Title != "Parent Alice" {
		t.Errorf("parent Title = %q, want %q", result.Steps[0].Title, "Parent Alice")
	}
	if result.Steps[0].Children[0].Title != "Child Alice" {
		t.Errorf("child Title = %q, want %q", result.Steps[0].Children[0].Title, "Child Alice")
	}
}

func TestMergeNeedsToDependsOn(t *testing.T) {
	dir := t.TempDir()
	writeFormula(t, dir, "with-needs", `{
		"formula": "with-needs",
		"version": 1,
		"type": "workflow",
		"steps": [
			{"id": "s1", "title": "First"},
			{"id": "s2", "title": "Second", "needs": ["s1"], "depends_on": ["s0"]}
		]
	}`)

	f, err := ResolveFormula("with-needs", FormulaSearchPath{dir})
	if err != nil {
		t.Fatalf("ResolveFormula() error = %v", err)
	}

	s2 := f.Steps[1]
	// depends_on should contain both original and merged needs.
	if len(s2.DependsOn) != 2 {
		t.Fatalf("len(DependsOn) = %d, want 2", len(s2.DependsOn))
	}
	if s2.DependsOn[0] != "s0" || s2.DependsOn[1] != "s1" {
		t.Errorf("DependsOn = %v, want [s0, s1]", s2.DependsOn)
	}
	if s2.Needs != nil {
		t.Errorf("Needs = %v, want nil", s2.Needs)
	}
}
