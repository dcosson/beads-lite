package reference

import (
	"fmt"
	"strings"
)

// TestCase defines a named e2e test scenario.
type TestCase struct {
	Name string
	Fn   func(r *Runner, n *Normalizer, sandbox string) (string, error)
	// PostUpdate optionally transforms the expected output after generating
	// it from the reference binary. Used when we can't or don't want our
	// implementation to perfectly match the reference implementation due to a
	// bug or confusing behavior.
	PostUpdate func(string) string
}

// testCases is the ordered registry of all e2e test cases.
var testCases = []TestCase{
	{Name: "01_create", Fn: caseCreate},
	{Name: "02_show", Fn: caseShow},
	{Name: "03_update", Fn: caseUpdate},
	{Name: "04_list", Fn: caseList},
	{Name: "05_close_reopen", Fn: caseCloseReopen},
	{Name: "06_delete", Fn: caseDelete},
	{Name: "07_deps", Fn: caseDeps},
	{Name: "08_parent_children", Fn: caseParentChildren},
	{Name: "09_comment", Fn: caseComment},
	{Name: "10_ready_blocked", Fn: caseReadyBlocked},
	{Name: "11_search", Fn: caseSearch},
	{Name: "12_stats", Fn: caseStats},
	{Name: "13_config", Fn: caseConfig},
	{Name: "14_meow", Fn: caseMeow, PostUpdate: patchMeowExpected},
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
