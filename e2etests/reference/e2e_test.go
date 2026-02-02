package reference

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update expected output files")

// verifyReferenceBeads checks that BD_CMD points to the original beads binary,
// not beads-lite. Expected output files must be generated from the reference
// implementation. If the --help output contains "Beads Lite", this is the wrong
// binary and the update run is aborted.
func verifyReferenceBeads(t *testing.T, runner *Runner) {
	t.Helper()
	result := runner.Run("", "--help")
	if result.ExitCode != 0 {
		t.Fatalf("BD_CMD --help failed (exit %d): %s", result.ExitCode, result.Stderr)
	}
	if strings.Contains(result.Stdout, "Beads Lite") {
		t.Fatal("ABORTING: BD_CMD points to beads-lite, not the reference beads binary.\n" +
			"Expected output files must be generated from the original beads.\n" +
			"Set BD_CMD to the original beads binary (e.g. BD_CMD=$(which bd)).")
	}
}

func TestE2E(t *testing.T) {
	bdCmd := os.Getenv("BD_CMD")
	if bdCmd == "" {
		t.Skip("BD_CMD environment variable not set")
		return
	}

	runner := &Runner{BdCmd: bdCmd, KillDaemons: *update}

	if *update {
		verifyReferenceBeads(t, runner)
	}

	var prevSandbox string

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Tear down the previous sandbox (after daemon killall ran in SetupSandbox).
			if prevSandbox != "" {
				runner.TeardownSandbox(prevSandbox)
				prevSandbox = ""
			}

			sandbox, err := runner.SetupSandbox()
			if err != nil {
				t.Fatalf("failed to setup sandbox: %v", err)
			}
			prevSandbox = sandbox

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
				if tc.PostUpdate != nil {
					actual = tc.PostUpdate(actual)
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

			if diff := compareOutput(string(expected), actual); diff != "" {
				t.Errorf("output mismatch for %s:\n%s", tc.Name, diff)
			}
		})
	}

	// Kill any daemons left from the last test, then tear down the last sandbox.
	if prevSandbox != "" {
		if runner.KillDaemons {
			runner.KillAllDaemons(prevSandbox)
		}
		runner.TeardownSandbox(prevSandbox)
	}
}

// testSection represents a named section of test output (e.g., "=== create basic task ===\n{...}").
type testSection struct {
	Name    string
	Content string
}

// splitSections parses test output into named sections delimited by "=== name ===" headers.
func splitSections(s string) []testSection {
	var sections []testSection
	lines := strings.Split(s, "\n")
	var current *testSection
	var contentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
			if current != nil {
				current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
				sections = append(sections, *current)
			}
			name := strings.TrimPrefix(line, "=== ")
			name = strings.TrimSuffix(name, " ===")
			current = &testSection{Name: name}
			contentLines = nil
		} else if current != nil {
			contentLines = append(contentLines, line)
		}
	}
	if current != nil {
		current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
		sections = append(sections, *current)
	}

	return sections
}

// compareOutput compares expected and actual test output section by section.
// For JSON sections, it uses superset matching: the actual output may contain
// extra fields not in the expected output, but every field in the expected
// output must be present in the actual output with the same value.
// For non-JSON sections (e.g., EXIT_CODE), it uses exact string comparison.
// Returns an empty string if the outputs match, or a description of differences.
func compareOutput(expected, actual string) string {
	expSections := splitSections(expected)
	actSections := splitSections(actual)

	var b strings.Builder

	if len(expSections) != len(actSections) {
		b.WriteString(fmt.Sprintf("section count mismatch: expected %d, got %d\n", len(expSections), len(actSections)))
		b.WriteString(fmt.Sprintf("expected sections: %s\n", sectionNames(expSections)))
		b.WriteString(fmt.Sprintf("actual sections:   %s\n", sectionNames(actSections)))
	}

	maxSections := len(expSections)
	if len(actSections) < maxSections {
		maxSections = len(actSections)
	}

	for i := 0; i < maxSections; i++ {
		exp := expSections[i]
		act := actSections[i]

		if exp.Name != act.Name {
			b.WriteString(fmt.Sprintf("section %d: expected %q, got %q\n", i, exp.Name, act.Name))
			continue
		}

		// Try JSON superset comparison
		var expJSON, actJSON interface{}
		expErr := json.Unmarshal([]byte(exp.Content), &expJSON)
		actErr := json.Unmarshal([]byte(act.Content), &actJSON)

		if expErr == nil && actErr == nil {
			if err := jsonSupersetMatch(expJSON, actJSON, ""); err != nil {
				b.WriteString(fmt.Sprintf("section %q: %v\n", exp.Name, err))
			}
		} else {
			// Plain text comparison (e.g., EXIT_CODE sections)
			if exp.Content != act.Content {
				b.WriteString(fmt.Sprintf("section %q:\n  expected: %q\n  actual:   %q\n", exp.Name, exp.Content, act.Content))
			}
		}
	}

	// Report any extra expected sections
	for i := maxSections; i < len(expSections); i++ {
		b.WriteString(fmt.Sprintf("missing section: %q\n", expSections[i].Name))
	}
	// Report any extra actual sections
	for i := maxSections; i < len(actSections); i++ {
		b.WriteString(fmt.Sprintf("extra section: %q\n", actSections[i].Name))
	}

	return b.String()
}

func sectionNames(sections []testSection) string {
	names := make([]string, len(sections))
	for i, s := range sections {
		names[i] = s.Name
	}
	return "[" + strings.Join(names, ", ") + "]"
}

// jsonSupersetMatch checks that actual is a superset of expected.
// Every key/value in expected must exist in actual with the same value.
// Extra keys in actual are allowed. Arrays must have the same length
// and each element is compared with superset logic.
func jsonSupersetMatch(expected, actual interface{}, path string) error {
	switch exp := expected.(type) {
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: expected object, got %T", pathOrRoot(path), actual)
		}
		for key, expVal := range exp {
			childPath := path + "." + key
			actVal, exists := act[key]
			if !exists {
				return fmt.Errorf("%s: missing field %q", pathOrRoot(path), key)
			}
			if err := jsonSupersetMatch(expVal, actVal, childPath); err != nil {
				return err
			}
		}
		return nil

	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok {
			return fmt.Errorf("%s: expected array, got %T", pathOrRoot(path), actual)
		}
		if len(exp) != len(act) {
			return fmt.Errorf("%s: array length %d, expected %d", pathOrRoot(path), len(act), len(exp))
		}
		for i := range exp {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if err := jsonSupersetMatch(exp[i], act[i], childPath); err != nil {
				return err
			}
		}
		return nil

	default:
		// Primitives: exact match
		if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
			return fmt.Errorf("%s: expected %v, got %v", pathOrRoot(path), expected, actual)
		}
		return nil
	}
}

func pathOrRoot(path string) string {
	if path == "" {
		return "(root)"
	}
	return path
}
