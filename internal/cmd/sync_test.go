package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyncCmd(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out: &out,
	}

	cmd := newSyncCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	if !strings.Contains(out.String(), "no-op") {
		t.Errorf("expected no-op message, got: %s", out.String())
	}
}

func TestSyncCmd_ImportOnly(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out: &out,
	}

	cmd := newSyncCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--import-only"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync --import-only failed: %v", err)
	}

	if !strings.Contains(out.String(), "no-op") {
		t.Errorf("expected no-op message, got: %s", out.String())
	}
}

func TestSyncCmd_JSON(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out:  &out,
		JSON: true,
	}

	cmd := newSyncCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status":"noop"`) {
		t.Errorf("expected JSON noop status, got: %s", got)
	}
}
