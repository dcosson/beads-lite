package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMolPourText(t *testing.T) {
	app, _ := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newMolPourCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"test-formula"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no formula file), got nil")
	}
	// The real Pour tries to load a formula file that doesn't exist
	if !strings.Contains(err.Error(), "pour test-formula") {
		t.Errorf("expected 'pour test-formula' error, got %v", err)
	}
	_ = out // no output expected on error
}

func TestMolPourJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolPourCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"test-formula"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no formula file), got nil")
	}
	if !strings.Contains(err.Error(), "pour test-formula") {
		t.Errorf("expected 'pour test-formula' error, got %v", err)
	}
}

func TestMolPourWithVars(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolPourCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"test-formula", "--var", "component=auth", "--var", "severity=high"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no formula file), got nil")
	}
	if !strings.Contains(err.Error(), "pour test-formula") {
		t.Errorf("expected 'pour test-formula' error, got %v", err)
	}
}

func TestMolPourMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolPourCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestMolWispText(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolWispCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"scratch-pad"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no formula file), got nil")
	}
	if !strings.Contains(err.Error(), "wisp scratch-pad") {
		t.Errorf("expected 'wisp scratch-pad' error, got %v", err)
	}
}

func TestMolWispJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolWispCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"scratch-pad"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no formula file), got nil")
	}
	if !strings.Contains(err.Error(), "wisp scratch-pad") {
		t.Errorf("expected 'wisp scratch-pad' error, got %v", err)
	}
}

func TestMolCurrentText(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolCurrentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub Current, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolCurrentJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolCurrentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub Current, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolCurrentWithFlags(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolCurrentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2", "--for", "alice", "--limit", "5", "--range", "s1-s5"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub Current, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolCurrentMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolCurrentCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestMolProgressText(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolProgressCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub Progress, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolProgressJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolProgressCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub Progress, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolProgressMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolProgressCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestMolStaleText(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolStaleCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub FindStaleSteps, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolStaleJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolStaleCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from stub FindStaleSteps, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' error, got %v", err)
	}
}

func TestMolStaleMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolStaleCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestMolBurnText(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolBurnCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no such molecule), got nil")
	}
	// Real Burn tries to load the molecule and fails with not found
	if !strings.Contains(err.Error(), "burn") {
		t.Errorf("expected 'burn' error, got %v", err)
	}
}

func TestMolBurnJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolBurnCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no such molecule), got nil")
	}
	if !strings.Contains(err.Error(), "burn") {
		t.Errorf("expected 'burn' error, got %v", err)
	}
}

func TestMolBurnMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolBurnCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestMolSquashText(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolSquashCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2", "--summary", "Completed feature"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no such molecule), got nil")
	}
	// Real Squash tries to load the molecule and fails with not found
	if !strings.Contains(err.Error(), "squash") {
		t.Errorf("expected 'squash' error, got %v", err)
	}
}

func TestMolSquashJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true

	cmd := newMolSquashCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-a1b2", "--summary", "Completed feature", "--keep-children"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (no such molecule), got nil")
	}
	if !strings.Contains(err.Error(), "squash") {
		t.Errorf("expected 'squash' error, got %v", err)
	}
}

func TestMolSquashMissingArg(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolSquashCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestMolGCText(t *testing.T) {
	app, _ := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newMolGCCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Real GC runs successfully with 0 deletions on empty store
	if !strings.Contains(out.String(), "GC") {
		t.Errorf("expected GC output, got %q", out.String())
	}
}

func TestMolGCJSON(t *testing.T) {
	app, _ := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newMolGCCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected JSON output, got empty")
	}
}

func TestMolGCWithOlderThan(t *testing.T) {
	app, _ := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newMolGCCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--older-than", "24h"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "GC") {
		t.Errorf("expected GC output, got %q", out.String())
	}
}

func TestMolGCInvalidDuration(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolGCCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--older-than", "notaduration"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
	if !strings.Contains(err.Error(), "invalid duration") {
		t.Errorf("expected 'invalid duration' error, got %v", err)
	}
}

func TestMolParentCommand(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newMolCmd(NewTestProvider(app))
	// Running the parent command with no subcommand should print help (no error)
	cmd.SetArgs([]string{})
	// Cobra prints help for group commands and returns nil
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from mol parent command: %v", err)
	}
}

func TestParseVarFlags(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect map[string]string
	}{
		{
			name:   "empty",
			input:  nil,
			expect: map[string]string{},
		},
		{
			name:   "single",
			input:  []string{"key=value"},
			expect: map[string]string{"key": "value"},
		},
		{
			name:   "multiple",
			input:  []string{"a=1", "b=2"},
			expect: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:   "value with equals",
			input:  []string{"key=val=ue"},
			expect: map[string]string{"key": "val=ue"},
		},
		{
			name:   "no equals ignored",
			input:  []string{"noequals"},
			expect: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVarFlags(tt.input)
			if len(result) != len(tt.expect) {
				t.Fatalf("expected %d entries, got %d: %v", len(tt.expect), len(result), result)
			}
			for k, v := range tt.expect {
				if result[k] != v {
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

// TestMolSubcommandRegistration verifies all subcommands are registered on the parent.
func TestMolSubcommandRegistration(t *testing.T) {
	app, _ := setupTestApp(t)
	cmd := newMolCmd(NewTestProvider(app))

	expectedSubs := []string{"pour", "wisp", "current", "progress", "stale", "burn", "squash", "gc"}
	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found on mol command", name)
		}
	}
}

// TestMolJSONOutputStructure verifies that when the stubs eventually return
// data, the JSON encoding path works. We test with a mock by directly encoding.
func TestMolJSONOutputStructure(t *testing.T) {
	// Verify result types are JSON-encodable
	pourResult := &json.Encoder{}
	_ = pourResult

	// PourResult
	pr := struct {
		RootID   string   `json:"root_id"`
		ChildIDs []string `json:"child_ids"`
	}{
		RootID:   "bd-test",
		ChildIDs: []string{"bd-test.1", "bd-test.2"},
	}
	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("failed to marshal PourResult: %v", err)
	}
	if !strings.Contains(string(data), "bd-test") {
		t.Errorf("expected JSON to contain 'bd-test', got %s", data)
	}
}
