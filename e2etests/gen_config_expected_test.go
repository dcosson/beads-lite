package e2etests

import (
	"flag"
	"os"
	"testing"
)

var genConfig = flag.Bool("gen-config", false, "generate expected output for config test")

// TestGenConfigExpected generates the expected output file for the config e2e
// test case. This is separate from the standard -update flow because the
// config commands behave differently in beads-lite vs the reference binary.
//
// Run with:
//
//	BD_CMD=./bd go test ./e2etests/ -run TestGenConfigExpected -gen-config
func TestGenConfigExpected(t *testing.T) {
	if !*genConfig {
		t.Skip("run with -gen-config to generate expected output")
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
	actual, err := caseConfig(runner, norm, sandbox)
	if err != nil {
		t.Fatalf("test case failed: %v", err)
	}

	if err := os.MkdirAll("expected", 0755); err != nil {
		t.Fatalf("failed to create expected dir: %v", err)
	}
	if err := os.WriteFile("expected/13_config.txt", []byte(actual), 0644); err != nil {
		t.Fatalf("failed to write expected file: %v", err)
	}
	t.Logf("updated expected/13_config.txt")
}
