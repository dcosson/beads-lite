package e2etests

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateMeowExpected generates the expected output for case_16_meow.
// This is separate from the standard -update flow because meow commands are
// beads-lite-specific and don't exist in the reference beads binary.
//
// Usage: BD_CMD=./bd go test ./e2etests -run TestGenerateMeowExpected -v -count=1
func TestGenerateMeowExpected(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	runner := &Runner{BdCmd: bdCmd}
	sandbox, err := runner.SetupSandbox()
	if err != nil {
		t.Fatalf("setup sandbox: %v", err)
	}
	defer runner.TeardownSandbox(sandbox)

	norm := NewNormalizer()
	actual, err := caseMeow(runner, norm, sandbox)
	if err != nil {
		t.Fatalf("caseMeow failed: %v", err)
	}

	expectedFile := filepath.Join("expected", "15_meow.txt")
	if err := os.MkdirAll("expected", 0755); err != nil {
		t.Fatalf("mkdir expected: %v", err)
	}
	if err := os.WriteFile(expectedFile, []byte(actual), 0644); err != nil {
		t.Fatalf("write expected: %v", err)
	}
	t.Logf("Generated %s (%d bytes)", expectedFile, len(actual))
}
