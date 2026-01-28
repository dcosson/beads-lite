package e2etests

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update expected output files")

func TestE2E(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set; skipping e2e tests")
	}

	runner := &Runner{BdCmd: bdCmd}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			sandbox, err := runner.SetupSandbox()
			if err != nil {
				t.Fatalf("failed to setup sandbox: %v", err)
			}
			defer runner.TeardownSandbox(sandbox)

			norm := NewNormalizer()
			actual, err := tc.Fn(runner, norm, sandbox)
			if err != nil {
				t.Fatalf("test case failed: %v", err)
			}

			expectedFile := filepath.Join("expected", tc.Name+".txt")

			if *update {
				if err := os.MkdirAll("expected", 0755); err != nil {
					t.Fatalf("failed to create expected dir: %v", err)
				}
				if err := os.WriteFile(expectedFile, []byte(actual), 0644); err != nil {
					t.Fatalf("failed to write expected file: %v", err)
				}
				t.Logf("updated %s", expectedFile)
				return
			}

			expected, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Fatalf("no expected file %q (run with -update to generate): %v", expectedFile, err)
			}

			if actual != string(expected) {
				t.Errorf("output mismatch for %s:\n%s", tc.Name, lineDiff(string(expected), actual))
			}
		})
	}
}

func TestCommandDiscovery(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set; skipping e2e tests")
	}

	runner := &Runner{BdCmd: bdCmd}

	unknown, err := DiscoverCommands(runner)
	if err != nil {
		t.Fatalf("command discovery failed: %v", err)
	}

	if len(unknown) > 0 {
		t.Errorf("discovered commands not in knownCommands registry:\n  %s\n\nAdd these to knownCommands in commands.go and create test cases for them.",
			strings.Join(unknown, "\n  "))
	}
}

// lineDiff produces a simple line-by-line diff between two strings.
func lineDiff(expected, actual string) string {
	expLines := strings.Split(expected, "\n")
	actLines := strings.Split(actual, "\n")

	var b strings.Builder
	maxLines := len(expLines)
	if len(actLines) > maxLines {
		maxLines = len(actLines)
	}

	for i := 0; i < maxLines; i++ {
		expLine := ""
		actLine := ""
		if i < len(expLines) {
			expLine = expLines[i]
		}
		if i < len(actLines) {
			actLine = actLines[i]
		}

		if expLine != actLine {
			b.WriteString(fmt.Sprintf("line %d:\n  expected: %q\n  actual:   %q\n", i+1, expLine, actLine))
		}
	}

	if len(expLines) != len(actLines) {
		b.WriteString(fmt.Sprintf("\nexpected %d lines, got %d lines\n", len(expLines), len(actLines)))
	}

	return b.String()
}
