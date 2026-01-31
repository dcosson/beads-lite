package e2etests

import (
	"flag"
	"os"
	"testing"
)

var genDelete = flag.Bool("gen-delete", false, "generate expected output for delete test")

// TestGenDeleteExpected generates the expected output file for the delete e2e
// test case. This is separate from the standard -update flow because the
// delete test includes tombstone behavior that is beads-lite-specific.
//
// Run with:
//
//	BD_CMD=./bd go test ./e2etests/ -run TestGenDeleteExpected -gen-delete
func TestGenDeleteExpected(t *testing.T) {
	if !*genDelete {
		t.Skip("run with -gen-delete to generate expected output")
	}

	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
	}

	runner := &Runner{BdCmd: bdCmd}
	sandbox, err := runner.SetupSandbox()
	if err != nil {
		t.Fatalf("failed to setup sandbox: %v", err)
	}
	defer runner.TeardownSandbox(sandbox)

	norm := NewNormalizer()
	actual, err := caseDelete(runner, norm, sandbox)
	if err != nil {
		t.Fatalf("test case failed: %v", err)
	}

	if err := os.MkdirAll("expected", 0755); err != nil {
		t.Fatalf("failed to create expected dir: %v", err)
	}
	if err := os.WriteFile("expected/06_delete.txt", []byte(actual), 0644); err != nil {
		t.Fatalf("failed to write expected file: %v", err)
	}
	t.Logf("updated expected/06_delete.txt")
}
