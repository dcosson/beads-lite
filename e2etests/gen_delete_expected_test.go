package e2etests

import (
	"flag"
	"os"
	"testing"
)

var genTombstone = flag.Bool("gen-tombstone", false, "generate expected output for tombstone test")

// TestGenTombstoneExpected generates the expected output file for the
// tombstone e2e test case. Run with:
//
//	BD_CMD=/path/to/bd go test ./e2etests/ -run TestGenTombstoneExpected -gen-tombstone
func TestGenTombstoneExpected(t *testing.T) {
	if !*genTombstone {
		t.Skip("run with -gen-tombstone to generate expected output")
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
	actual, err := caseDeleteTombstone(runner, norm, sandbox)
	if err != nil {
		t.Fatalf("test case failed: %v", err)
	}

	if err := os.MkdirAll("expected", 0755); err != nil {
		t.Fatalf("failed to create expected dir: %v", err)
	}
	if err := os.WriteFile("expected/17_delete_tombstone.txt", []byte(actual), 0644); err != nil {
		t.Fatalf("failed to write expected file: %v", err)
	}
	t.Logf("updated expected/17_delete_tombstone.txt")
}
