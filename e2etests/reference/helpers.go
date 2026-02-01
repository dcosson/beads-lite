package reference

import (
	"fmt"
	"strings"
)

// TestCase defines a named e2e test scenario.
type TestCase struct {
	Name string
	Fn   func(r *Runner, n *Normalizer, sandbox string) (string, error)
}

// testCases is the ordered registry of all e2e test cases.
var testCases = []TestCase{
	{"01_create", caseCreate},
	{"02_show", caseShow},
	{"03_update", caseUpdate},
	{"04_list", caseList},
	{"05_close_reopen", caseCloseReopen},
	{"06_delete", caseDelete},
	{"07_deps", caseDeps},
	{"08_parent_children", caseParentChildren},
	{"09_comment", caseComment},
	{"10_ready_blocked", caseReadyBlocked},
	{"11_search", caseSearch},
	{"12_stats", caseStats},
	{"13_config", caseConfig},
	{"14_dot_notation_ids", caseDotNotationIDs},
	{"15_meow", caseMeow},
}

// section writes a section header and normalized JSON content to the builder.
func section(out *strings.Builder, label string, content string) {
	out.WriteString("=== ")
	out.WriteString(label)
	out.WriteString(" ===\n")
	out.WriteString(content)
	out.WriteString("\n\n")
}

// sectionExitCode writes a section with just an exit code.
func sectionExitCode(out *strings.Builder, label string, exitCode int) {
	out.WriteString("=== ")
	out.WriteString(label)
	out.WriteString(" ===\n")
	out.WriteString(fmt.Sprintf("EXIT_CODE: %d", exitCode))
	out.WriteString("\n\n")
}

// mustRun runs a command and returns the result, failing the test case on error.
func mustRun(r *Runner, sandbox string, args ...string) (RunResult, error) {
	result := r.Run(sandbox, args...)
	if result.ExitCode != 0 {
		return result, fmt.Errorf("command %v failed (exit %d): %s", args, result.ExitCode, result.Stderr)
	}
	return result, nil
}

// mustExtractID extracts the issue ID from a create response.
func mustExtractID(result RunResult) (string, error) {
	id := ExtractID([]byte(result.Stdout))
	if id == "" {
		return "", fmt.Errorf("failed to extract ID from: %s", result.Stdout)
	}
	return id, nil
}
