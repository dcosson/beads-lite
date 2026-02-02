package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrimeCmd(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out: &out,
	}

	cmd := newPrimeCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prime command failed: %v", err)
	}

	if !strings.Contains(out.String(), "no-op") {
		t.Errorf("expected no-op message, got: %s", out.String())
	}
}

func TestPrimeCmd_JSON(t *testing.T) {
	var out bytes.Buffer
	app := &App{
		Out:  &out,
		JSON: true,
	}

	cmd := newPrimeCmd(NewTestProvider(app))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prime command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status":"noop"`) {
		t.Errorf("expected JSON noop status, got: %s", got)
	}
}
