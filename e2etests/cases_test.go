package e2etests

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
	{"01_create_basic", caseCreateBasic},
	{"02_create_with_flags", caseCreateWithFlags},
	{"03_show", caseShow},
	{"04_update", caseUpdate},
	{"05_list", caseList},
	{"06_close_reopen", caseCloseReopen},
	{"07_delete", caseDelete},
	{"08_dep_lifecycle", caseDepLifecycle},
	{"09_parent_children", caseParentChildren},
	{"10_comment", caseComment},
	{"11_ready_blocked", caseReadyBlocked},
	{"12_search", caseSearch},
	{"13_stats", caseStats},
	{"14_delete_cascade", caseDeleteCascade},
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

// 01: Create a basic task, show it, list all issues.
func caseCreateBasic(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create
	result, err := mustRun(r, sandbox, "create", "Fix login bug", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create basic task", n.NormalizeJSON([]byte(result.Stdout)))

	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Show
	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show the created issue", n.NormalizeJSON([]byte(result.Stdout)))

	// List
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list all issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}

// 02: Create with all supported flags.
func caseCreateWithFlags(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create a dependency target first
	result, err := mustRun(r, sandbox, "create", "Dependency target", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create dependency target", n.NormalizeJSON([]byte(result.Stdout)))

	depID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a parent
	result, err = mustRun(r, sandbox, "create", "Parent issue", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create parent issue", n.NormalizeJSON([]byte(result.Stdout)))

	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create issue with all flags
	result, err = mustRun(r, sandbox, "create", "Full featured issue",
		"--type", "feature",
		"--priority", "1",
		"--description", "A detailed description",
		"--label", "urgent",
		"--label", "v2",
		"--assignee", "alice",
		"--depends-on", depID,
		"--parent", parentID,
		"--json",
	)
	if err != nil {
		return "", err
	}
	section(&out, "create with all flags", n.NormalizeJSON([]byte(result.Stdout)))

	fullID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Show the fully created issue
	result, err = mustRun(r, sandbox, "show", fullID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show the created issue", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 03: Show a fully populated issue (with comments, deps, parent, children).
func caseShow(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create parent
	result, err := mustRun(r, sandbox, "create", "Parent task", "--json")
	if err != nil {
		return "", err
	}
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create main issue as child of parent
	result, err = mustRun(r, sandbox, "create", "Main task",
		"--type", "feature",
		"--priority", "1",
		"--description", "Main task description",
		"--label", "important",
		"--assignee", "bob",
		"--parent", parentID,
		"--json",
	)
	if err != nil {
		return "", err
	}
	mainID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a dependency
	result, err = mustRun(r, sandbox, "create", "Dependency task", "--json")
	if err != nil {
		return "", err
	}
	depID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a child of main
	result, err = mustRun(r, sandbox, "create", "Child task", "--parent", mainID, "--json")
	if err != nil {
		return "", err
	}

	// Add dependency
	_, err = mustRun(r, sandbox, "dep", "add", mainID, depID, "--json")
	if err != nil {
		return "", err
	}

	// Add comment
	_, err = mustRun(r, sandbox, "comment", "add", mainID, "This is a comment", "--json")
	if err != nil {
		return "", err
	}

	// Show the fully populated issue
	result, err = mustRun(r, sandbox, "show", mainID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show fully populated issue", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 04: Update all fields and verify via show.
func caseUpdate(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issue
	result, err := mustRun(r, sandbox, "create", "Original title", "--json")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Update title
	result, err = mustRun(r, sandbox, "update", id, "--title", "Updated title", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "update title", n.NormalizeJSON([]byte(result.Stdout)))

	// Update description
	result, err = mustRun(r, sandbox, "update", id, "--description", "New description", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "update description", n.NormalizeJSON([]byte(result.Stdout)))

	// Update priority
	result, err = mustRun(r, sandbox, "update", id, "--priority", "0", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "update priority", n.NormalizeJSON([]byte(result.Stdout)))

	// Update type
	result, err = mustRun(r, sandbox, "update", id, "--type", "bug", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "update type", n.NormalizeJSON([]byte(result.Stdout)))

	// Update status
	result, err = mustRun(r, sandbox, "update", id, "--status", "in-progress", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "update status", n.NormalizeJSON([]byte(result.Stdout)))

	// Add labels
	result, err = mustRun(r, sandbox, "update", id, "--add-label", "urgent", "--add-label", "v2", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add labels", n.NormalizeJSON([]byte(result.Stdout)))

	// Remove label
	result, err = mustRun(r, sandbox, "update", id, "--remove-label", "urgent", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "remove label", n.NormalizeJSON([]byte(result.Stdout)))

	// Update assignee
	result, err = mustRun(r, sandbox, "update", id, "--assignee", "charlie", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "update assignee", n.NormalizeJSON([]byte(result.Stdout)))

	// Show final state
	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show after all updates", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 05: List with various filters.
func caseList(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issues with various attributes
	result, err := mustRun(r, sandbox, "create", "Open task", "--type", "task", "--priority", "2", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "High bug", "--type", "bug", "--priority", "1", "--json")
	if err != nil {
		return "", err
	}
	bugID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Feature request", "--type", "feature", "--priority", "3", "--json")
	if err != nil {
		return "", err
	}

	// Close one issue
	_, err = mustRun(r, sandbox, "close", bugID, "--json")
	if err != nil {
		return "", err
	}

	// List all open (default)
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list open issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List all issues
	result, err = mustRun(r, sandbox, "list", "--all", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list all issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List closed
	result, err = mustRun(r, sandbox, "list", "--closed", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list closed issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List by type
	result, err = mustRun(r, sandbox, "list", "--type", "feature", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list by type feature", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List by priority
	result, err = mustRun(r, sandbox, "list", "--priority", "high", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list by priority high", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List format ids
	result, err = mustRun(r, sandbox, "list", "--format", "ids", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list format ids", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 06: Close and reopen lifecycle.
func caseCloseReopen(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issue
	result, err := mustRun(r, sandbox, "create", "Closeable task", "--json")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Close it
	result, err = mustRun(r, sandbox, "close", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "close issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Show closed state
	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show closed issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify in closed list
	result, err = mustRun(r, sandbox, "list", "--closed", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list closed", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Reopen
	result, err = mustRun(r, sandbox, "reopen", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "reopen issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Show reopened state
	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show reopened issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify in open list
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list open after reopen", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}

// 07: Delete with --force.
func caseDelete(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create two issues
	result, err := mustRun(r, sandbox, "create", "Keeper", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Deletable", "--json")
	if err != nil {
		return "", err
	}
	deleteID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Delete the second
	result, err = mustRun(r, sandbox, "delete", deleteID, "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "delete issue", n.NormalizeJSON([]byte(result.Stdout)))

	// List should only show first
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after delete", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Show deleted should fail
	showResult := r.Run(sandbox, "show", deleteID, "--json")
	sectionExitCode(&out, "show deleted issue", showResult.ExitCode)

	return out.String(), nil
}

// 08: Dependency lifecycle (add, list, remove, verify symmetry).
func caseDepLifecycle(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create two issues
	result, err := mustRun(r, sandbox, "create", "Issue A", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create issue A", n.NormalizeJSON([]byte(result.Stdout)))
	idA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue B", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create issue B", n.NormalizeJSON([]byte(result.Stdout)))
	idB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Add dep: A depends on B
	result, err = mustRun(r, sandbox, "dep", "add", idA, idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add dependency A depends on B", n.NormalizeJSON([]byte(result.Stdout)))

	// Show A has depends_on
	result, err = mustRun(r, sandbox, "show", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show A has depends_on", n.NormalizeJSON([]byte(result.Stdout)))

	// Show B has dependents
	result, err = mustRun(r, sandbox, "show", idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show B has dependents", n.NormalizeJSON([]byte(result.Stdout)))

	// Dep list A
	result, err = mustRun(r, sandbox, "dep", "list", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "dep list A", n.NormalizeJSON([]byte(result.Stdout)))

	// Remove dependency
	result, err = mustRun(r, sandbox, "dep", "remove", idA, idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "remove dependency", n.NormalizeJSON([]byte(result.Stdout)))

	// Show A after removal
	result, err = mustRun(r, sandbox, "show", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show A after dep removal", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 09: Parent/children lifecycle.
func caseParentChildren(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create parent
	result, err := mustRun(r, sandbox, "create", "Parent issue", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create parent", n.NormalizeJSON([]byte(result.Stdout)))
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create child
	result, err = mustRun(r, sandbox, "create", "Child issue", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create child", n.NormalizeJSON([]byte(result.Stdout)))
	childID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Set parent via update
	result, err = mustRun(r, sandbox, "update", childID, "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "set parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child has parent
	result, err = mustRun(r, sandbox, "show", childID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show child has parent", n.NormalizeJSON([]byte(result.Stdout)))

	// List children of parent
	result, err = mustRun(r, sandbox, "children", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list children of parent", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Remove parent via update
	result, err = mustRun(r, sandbox, "update", childID, "--parent", "", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "remove parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child after removal
	result, err = mustRun(r, sandbox, "show", childID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show child after parent removal", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 10: Comment add and list.
func caseComment(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issue
	result, err := mustRun(r, sandbox, "create", "Commentable task", "--json")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Add first comment
	result, err = mustRun(r, sandbox, "comment", "add", id, "First comment", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add first comment", n.NormalizeJSON([]byte(result.Stdout)))

	// Add second comment
	result, err = mustRun(r, sandbox, "comment", "add", id, "Second comment", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add second comment", n.NormalizeJSON([]byte(result.Stdout)))

	// List comments
	result, err = mustRun(r, sandbox, "comment", "list", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list comments", n.NormalizeJSON([]byte(result.Stdout)))

	// Show issue with comments
	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show issue with comments", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 11: Ready and blocked queries affected by dependencies.
func caseReadyBlocked(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issues
	result, err := mustRun(r, sandbox, "create", "Blocker task", "--json")
	if err != nil {
		return "", err
	}
	blockerID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Blocked task", "--json")
	if err != nil {
		return "", err
	}
	blockedID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Independent task", "--json")
	if err != nil {
		return "", err
	}

	// Add dep: blocked depends on blocker
	_, err = mustRun(r, sandbox, "dep", "add", blockedID, blockerID, "--json")
	if err != nil {
		return "", err
	}

	// Ready should show blocker and independent (not blocked)
	result, err = mustRun(r, sandbox, "ready", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "ready issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Blocked should show the blocked issue
	result, err = mustRun(r, sandbox, "blocked", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "blocked issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Close blocker
	_, err = mustRun(r, sandbox, "close", blockerID, "--json")
	if err != nil {
		return "", err
	}

	// Ready should now include previously blocked
	result, err = mustRun(r, sandbox, "ready", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "ready after closing blocker", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Blocked should be empty
	result = r.Run(sandbox, "blocked", "--json")
	section(&out, "blocked after closing blocker", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 12: Search by title and description.
func caseSearch(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issues
	_, err := mustRun(r, sandbox, "create", "Fix authentication bug", "--description", "Login fails for OAuth users", "--json")
	if err != nil {
		return "", err
	}

	_, err = mustRun(r, sandbox, "create", "Add search feature", "--description", "Full text search needed", "--json")
	if err != nil {
		return "", err
	}

	result, err := mustRun(r, sandbox, "create", "Update OAuth library", "--json")
	if err != nil {
		return "", err
	}
	oauthID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Close one to test --all
	_, err = mustRun(r, sandbox, "close", oauthID, "--json")
	if err != nil {
		return "", err
	}

	// Search open only (default)
	result, err = mustRun(r, sandbox, "search", "OAuth", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth open only", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search all
	result, err = mustRun(r, sandbox, "search", "OAuth", "--all", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth all", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search title only
	result, err = mustRun(r, sandbox, "search", "OAuth", "--title-only", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth title only", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search by description content
	result, err = mustRun(r, sandbox, "search", "Full text", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "search by description", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}

// 13: Stats with issues in various states.
func caseStats(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issues in various states
	result, err := mustRun(r, sandbox, "create", "Open task one", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Open task two", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "In progress task", "--json")
	if err != nil {
		return "", err
	}
	ipID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "update", ipID, "--status", "in-progress", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Closed task", "--json")
	if err != nil {
		return "", err
	}
	closedID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "close", closedID, "--json")
	if err != nil {
		return "", err
	}

	// Get stats
	result, err = mustRun(r, sandbox, "stats", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "stats", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 14: Delete cascade - tests removing an issue and cleaning up dependencies in all directions.
// Dependency graph:
//   A depends on B
//   B depends on C
//   D depends on C
//   E depends on A
//   F depends on E
//   F depends on G
//
// When we delete A with --cascade, we expect:
// - A is deleted
// - Dependencies referencing A are cleaned up (E's dependency on A removed)
// - A's dependencies are cleaned up (A's dependency on B removed from B's dependents)
func caseDeleteCascade(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create all issues: A, B, C, D, E, F, G
	result, err := mustRun(r, sandbox, "create", "Issue A", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create A", n.NormalizeJSON([]byte(result.Stdout)))
	idA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue B", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create B", n.NormalizeJSON([]byte(result.Stdout)))
	idB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue C", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create C", n.NormalizeJSON([]byte(result.Stdout)))
	idC, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue D", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create D", n.NormalizeJSON([]byte(result.Stdout)))
	idD, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue E", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create E", n.NormalizeJSON([]byte(result.Stdout)))
	idE, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue F", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create F", n.NormalizeJSON([]byte(result.Stdout)))
	idF, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue G", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create G", n.NormalizeJSON([]byte(result.Stdout)))
	idG, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Set up dependencies:
	// A depends on B
	_, err = mustRun(r, sandbox, "dep", "add", idA, idB, "--json")
	if err != nil {
		return "", err
	}

	// B depends on C
	_, err = mustRun(r, sandbox, "dep", "add", idB, idC, "--json")
	if err != nil {
		return "", err
	}

	// D depends on C
	_, err = mustRun(r, sandbox, "dep", "add", idD, idC, "--json")
	if err != nil {
		return "", err
	}

	// E depends on A
	_, err = mustRun(r, sandbox, "dep", "add", idE, idA, "--json")
	if err != nil {
		return "", err
	}

	// F depends on E
	_, err = mustRun(r, sandbox, "dep", "add", idF, idE, "--json")
	if err != nil {
		return "", err
	}

	// F depends on G
	_, err = mustRun(r, sandbox, "dep", "add", idF, idG, "--json")
	if err != nil {
		return "", err
	}

	// Show state before delete
	result, err = mustRun(r, sandbox, "show", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show A before delete", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "show", idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show B before delete (A's dependency)", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "show", idE, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show E before delete (depends on A)", n.NormalizeJSON([]byte(result.Stdout)))

	// Delete A with cascade
	result, err = mustRun(r, sandbox, "delete", idA, "--cascade", "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "delete A with cascade", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify A is deleted
	showResult := r.Run(sandbox, "show", idA, "--json")
	section(&out, "show A after delete", n.NormalizeJSON([]byte(showResult.Stdout)))

	// Show B after delete - should no longer have A as dependent
	result, err = mustRun(r, sandbox, "show", idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show B after delete (A removed from dependents)", n.NormalizeJSON([]byte(result.Stdout)))

	// Show E after delete - should no longer depend on A
	result, err = mustRun(r, sandbox, "show", idE, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show E after delete (no longer depends on A)", n.NormalizeJSON([]byte(result.Stdout)))

	// Show C - should still have B and D as dependents
	result, err = mustRun(r, sandbox, "show", idC, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show C (unaffected, still has B and D as dependents)", n.NormalizeJSON([]byte(result.Stdout)))

	// Show F - should still depend on E and G
	result, err = mustRun(r, sandbox, "show", idF, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show F (unaffected, still depends on E and G)", n.NormalizeJSON([]byte(result.Stdout)))

	// List all remaining issues
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list all after cascade delete", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}
