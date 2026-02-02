package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestImportCmd(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out: &out,
	}

	cmd := newImportCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import command failed: %v", err)
	}

	if !strings.Contains(out.String(), "no-op") {
		t.Errorf("expected no-op message, got: %s", out.String())
	}
}

func TestImportCmd_JSON(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out:  &out,
		JSON: true,
	}

	cmd := newImportCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status":"noop"`) {
		t.Errorf("expected JSON noop status, got: %s", got)
	}
}

func TestImportCmd_WithInputFlag(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out: &out,
	}

	cmd := newImportCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"-i", ".beads/issues.jsonl"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import command with -i flag failed: %v", err)
	}

	if !strings.Contains(out.String(), "no-op") {
		t.Errorf("expected no-op message, got: %s", out.String())
	}
}

func TestImportCmd_WithAllFlags(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out:  &out,
		JSON: true,
	}

	cmd := newImportCmd(NewTestProvider(app))
	cmd.SetArgs([]string{
		"-i", ".beads/issues.jsonl",
		"--rename-on-import",
		"--no-git-history",
		"--protect-left-snapshot",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("import command with all flags failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status":"noop"`) {
		t.Errorf("expected JSON noop status, got: %s", got)
	}
}
