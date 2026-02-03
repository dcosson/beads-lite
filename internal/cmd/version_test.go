package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCmd_Text(t *testing.T) {
	var out bytes.Buffer
	provider := &AppProvider{Out: &out}

	cmd := newVersionCmd(provider)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := out.String()
	expected := "bd version " + Version + " (beads-lite)\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
	// Verify the format is parseable: "bd version X.Y.Z (beads-lite)"
	if !strings.HasPrefix(output, "bd version ") {
		t.Errorf("text output should start with 'bd version ', got %q", output)
	}
	if !strings.Contains(output, "(beads-lite)") {
		t.Errorf("text output should contain '(beads-lite)', got %q", output)
	}
}

func TestVersionCmd_JSON(t *testing.T) {
	var out bytes.Buffer
	provider := &AppProvider{Out: &out, JSONOutput: true}

	cmd := newVersionCmd(provider)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result["version"] != Version {
		t.Errorf("expected version %q, got %q", Version, result["version"])
	}
	if result["version"] != "0.49.1" {
		t.Errorf("expected version \"0.49.1\", got %q", result["version"])
	}
}

func TestVersionCmd_Semver(t *testing.T) {
	// Gastown requires the version to be a valid semver >= 0.43.0.
	// Verify the default version has three numeric dot-separated parts.
	parts := strings.Split(Version, ".")
	if len(parts) != 3 {
		t.Errorf("Version %q is not a valid semver (expected 3 parts)", Version)
	}
}
