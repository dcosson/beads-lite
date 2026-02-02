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
	expected := "bd version " + Version + "\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
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
}

func TestVersionCmd_Semver(t *testing.T) {
	// Gastown requires the version to be a valid semver >= 0.43.0.
	// Verify the default version has three numeric dot-separated parts.
	parts := strings.Split(Version, ".")
	if len(parts) != 3 {
		t.Errorf("Version %q is not a valid semver (expected 3 parts)", Version)
	}
}
