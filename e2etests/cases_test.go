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

// mustRunJSON runs a JSON command and returns the result, failing the test case on error.
func mustRunJSON(r *Runner, sandbox string, args ...string) (RunResult, error) {
	result := r.RunJSON(sandbox, args...)
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
	result, err := mustRunJSON(r, sandbox, "create", "Fix login bug")
	if err != nil {
		return "", err
	}
	section(&out, "create basic task", n.NormalizeJSON([]byte(result.Stdout)))

	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Show
	result, err = mustRunJSON(r, sandbox, "show", id)
	if err != nil {
		return "", err
	}
	section(&out, "show the created issue", n.NormalizeJSON([]byte(result.Stdout)))

	// List
	result, err = mustRunJSON(r, sandbox, "list")
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
	result, err := mustRunJSON(r, sandbox, "create", "Dependency target")
	if err != nil {
		return "", err
	}
	section(&out, "create dependency target", n.NormalizeJSON([]byte(result.Stdout)))

	depID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a parent
	result, err = mustRunJSON(r, sandbox, "create", "Parent issue", "--type", "epic")
	if err != nil {
		return "", err
	}
	section(&out, "create parent issue", n.NormalizeJSON([]byte(result.Stdout)))

	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create issue with all flags
	result, err = mustRunJSON(r, sandbox, "create", "Full featured issue",
		"--type", "feature",
		"--priority", "1",
		"--description", "A detailed description",
		"--label", "urgent",
		"--label", "v2",
		"--assignee", "alice",
		"--depends-on", depID,
		"--parent", parentID,
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
	result, err = mustRunJSON(r, sandbox, "show", fullID)
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
	result, err := mustRunJSON(r, sandbox, "create", "Parent task")
	if err != nil {
		return "", err
	}
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create main issue as child of parent
	result, err = mustRunJSON(r, sandbox, "create", "Main task",
		"--type", "feature",
		"--priority", "1",
		"--description", "Main task description",
		"--label", "important",
		"--assignee", "bob",
		"--parent", parentID,
	)
	if err != nil {
		return "", err
	}
	mainID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a dependency
	result, err = mustRunJSON(r, sandbox, "create", "Dependency task")
	if err != nil {
		return "", err
	}
	depID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a child of main
	result, err = mustRunJSON(r, sandbox, "create", "Child task", "--parent", mainID)
	if err != nil {
		return "", err
	}

	// Add dependency
	_, err = mustRunJSON(r, sandbox, "dep", "add", mainID, depID)
	if err != nil {
		return "", err
	}

	// Add comment
	_, err = mustRunJSON(r, sandbox, "comment", "add", mainID, "This is a comment")
	if err != nil {
		return "", err
	}

	// Show the fully populated issue
	result, err = mustRunJSON(r, sandbox, "show", mainID)
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
	result, err := mustRunJSON(r, sandbox, "create", "Original title")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Update title
	result, err = mustRunJSON(r, sandbox, "update", id, "--title", "Updated title")
	if err != nil {
		return "", err
	}
	section(&out, "update title", n.NormalizeJSON([]byte(result.Stdout)))

	// Update description
	result, err = mustRunJSON(r, sandbox, "update", id, "--description", "New description")
	if err != nil {
		return "", err
	}
	section(&out, "update description", n.NormalizeJSON([]byte(result.Stdout)))

	// Update priority
	result, err = mustRunJSON(r, sandbox, "update", id, "--priority", "0")
	if err != nil {
		return "", err
	}
	section(&out, "update priority", n.NormalizeJSON([]byte(result.Stdout)))

	// Update type
	result, err = mustRunJSON(r, sandbox, "update", id, "--type", "bug")
	if err != nil {
		return "", err
	}
	section(&out, "update type", n.NormalizeJSON([]byte(result.Stdout)))

	// Update status
	result, err = mustRunJSON(r, sandbox, "update", id, "--status", "in-progress")
	if err != nil {
		return "", err
	}
	section(&out, "update status", n.NormalizeJSON([]byte(result.Stdout)))

	// Add labels
	result, err = mustRunJSON(r, sandbox, "update", id, "--add-label", "urgent", "--add-label", "v2")
	if err != nil {
		return "", err
	}
	section(&out, "add labels", n.NormalizeJSON([]byte(result.Stdout)))

	// Remove label
	result, err = mustRunJSON(r, sandbox, "update", id, "--remove-label", "urgent")
	if err != nil {
		return "", err
	}
	section(&out, "remove label", n.NormalizeJSON([]byte(result.Stdout)))

	// Update assignee
	result, err = mustRunJSON(r, sandbox, "update", id, "--assignee", "charlie")
	if err != nil {
		return "", err
	}
	section(&out, "update assignee", n.NormalizeJSON([]byte(result.Stdout)))

	// Show final state
	result, err = mustRunJSON(r, sandbox, "show", id)
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
	result, err := mustRunJSON(r, sandbox, "create", "Open task", "--type", "task", "--priority", "2")
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "High bug", "--type", "bug", "--priority", "1")
	if err != nil {
		return "", err
	}
	bugID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Feature request", "--type", "feature", "--priority", "3")
	if err != nil {
		return "", err
	}

	// Close one issue
	_, err = mustRunJSON(r, sandbox, "close", bugID)
	if err != nil {
		return "", err
	}

	// List all open (default)
	result, err = mustRunJSON(r, sandbox, "list")
	if err != nil {
		return "", err
	}
	section(&out, "list open issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List all issues
	result, err = mustRunJSON(r, sandbox, "list", "--all")
	if err != nil {
		return "", err
	}
	section(&out, "list all issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List closed
	result, err = mustRunJSON(r, sandbox, "list", "--closed")
	if err != nil {
		return "", err
	}
	section(&out, "list closed issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List by type
	result, err = mustRunJSON(r, sandbox, "list", "--type", "feature")
	if err != nil {
		return "", err
	}
	section(&out, "list by type feature", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List by priority
	result, err = mustRunJSON(r, sandbox, "list", "--priority", "high")
	if err != nil {
		return "", err
	}
	section(&out, "list by priority high", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List format ids
	result, err = mustRunJSON(r, sandbox, "list", "--format", "ids")
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
	result, err := mustRunJSON(r, sandbox, "create", "Closeable task")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Close it
	result, err = mustRunJSON(r, sandbox, "close", id)
	if err != nil {
		return "", err
	}
	section(&out, "close issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Show closed state
	result, err = mustRunJSON(r, sandbox, "show", id)
	if err != nil {
		return "", err
	}
	section(&out, "show closed issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify in closed list
	result, err = mustRunJSON(r, sandbox, "list", "--closed")
	if err != nil {
		return "", err
	}
	section(&out, "list closed", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Reopen
	result, err = mustRunJSON(r, sandbox, "reopen", id)
	if err != nil {
		return "", err
	}
	section(&out, "reopen issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Show reopened state
	result, err = mustRunJSON(r, sandbox, "show", id)
	if err != nil {
		return "", err
	}
	section(&out, "show reopened issue", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify in open list
	result, err = mustRunJSON(r, sandbox, "list")
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
	result, err := mustRunJSON(r, sandbox, "create", "Keeper")
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Deletable")
	if err != nil {
		return "", err
	}
	deleteID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Delete the second
	result, err = mustRunJSON(r, sandbox, "delete", deleteID, "--force")
	if err != nil {
		return "", err
	}
	section(&out, "delete issue", n.NormalizeJSON([]byte(result.Stdout)))

	// List should only show first
	result, err = mustRunJSON(r, sandbox, "list")
	if err != nil {
		return "", err
	}
	section(&out, "list after delete", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Show deleted should fail
	showResult := r.RunJSON(sandbox, "show", deleteID)
	sectionExitCode(&out, "show deleted issue", showResult.ExitCode)

	return out.String(), nil
}

// 08: Dependency lifecycle (add, list, remove, verify symmetry).
func caseDepLifecycle(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create two issues
	result, err := mustRunJSON(r, sandbox, "create", "Issue A")
	if err != nil {
		return "", err
	}
	section(&out, "create issue A", n.NormalizeJSON([]byte(result.Stdout)))
	idA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Issue B")
	if err != nil {
		return "", err
	}
	section(&out, "create issue B", n.NormalizeJSON([]byte(result.Stdout)))
	idB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Add dep: A depends on B
	result, err = mustRunJSON(r, sandbox, "dep", "add", idA, idB)
	if err != nil {
		return "", err
	}
	section(&out, "add dependency A depends on B", n.NormalizeJSON([]byte(result.Stdout)))

	// Show A has depends_on
	result, err = mustRunJSON(r, sandbox, "show", idA)
	if err != nil {
		return "", err
	}
	section(&out, "show A has depends_on", n.NormalizeJSON([]byte(result.Stdout)))

	// Show B has dependents
	result, err = mustRunJSON(r, sandbox, "show", idB)
	if err != nil {
		return "", err
	}
	section(&out, "show B has dependents", n.NormalizeJSON([]byte(result.Stdout)))

	// Dep list A
	result, err = mustRunJSON(r, sandbox, "dep", "list", idA)
	if err != nil {
		return "", err
	}
	section(&out, "dep list A", n.NormalizeJSON([]byte(result.Stdout)))

	// Remove dependency
	result, err = mustRunJSON(r, sandbox, "dep", "remove", idA, idB)
	if err != nil {
		return "", err
	}
	section(&out, "remove dependency", n.NormalizeJSON([]byte(result.Stdout)))

	// Show A after removal
	result, err = mustRunJSON(r, sandbox, "show", idA)
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
	result, err := mustRunJSON(r, sandbox, "create", "Parent issue", "--type", "epic")
	if err != nil {
		return "", err
	}
	section(&out, "create parent", n.NormalizeJSON([]byte(result.Stdout)))
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create child
	result, err = mustRunJSON(r, sandbox, "create", "Child issue")
	if err != nil {
		return "", err
	}
	section(&out, "create child", n.NormalizeJSON([]byte(result.Stdout)))
	childID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Set parent
	result, err = mustRunJSON(r, sandbox, "parent", "set", childID, parentID)
	if err != nil {
		return "", err
	}
	section(&out, "set parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child has parent
	result, err = mustRunJSON(r, sandbox, "show", childID)
	if err != nil {
		return "", err
	}
	section(&out, "show child has parent", n.NormalizeJSON([]byte(result.Stdout)))

	// List children of parent
	result, err = mustRunJSON(r, sandbox, "children", parentID)
	if err != nil {
		return "", err
	}
	section(&out, "list children of parent", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Remove parent
	result, err = mustRunJSON(r, sandbox, "parent", "remove", childID)
	if err != nil {
		return "", err
	}
	section(&out, "remove parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child after removal
	result, err = mustRunJSON(r, sandbox, "show", childID)
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
	result, err := mustRunJSON(r, sandbox, "create", "Commentable task")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Add first comment
	result, err = mustRunJSON(r, sandbox, "comment", "add", id, "First comment")
	if err != nil {
		return "", err
	}
	section(&out, "add first comment", n.NormalizeJSON([]byte(result.Stdout)))

	// Add second comment
	result, err = mustRunJSON(r, sandbox, "comment", "add", id, "Second comment")
	if err != nil {
		return "", err
	}
	section(&out, "add second comment", n.NormalizeJSON([]byte(result.Stdout)))

	// List comments
	result, err = mustRunJSON(r, sandbox, "comment", "list", id)
	if err != nil {
		return "", err
	}
	section(&out, "list comments", n.NormalizeJSON([]byte(result.Stdout)))

	// Show issue with comments
	result, err = mustRunJSON(r, sandbox, "show", id)
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
	result, err := mustRunJSON(r, sandbox, "create", "Blocker task")
	if err != nil {
		return "", err
	}
	blockerID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Blocked task")
	if err != nil {
		return "", err
	}
	blockedID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Independent task")
	if err != nil {
		return "", err
	}

	// Add dep: blocked depends on blocker
	_, err = mustRunJSON(r, sandbox, "dep", "add", blockedID, blockerID)
	if err != nil {
		return "", err
	}

	// Ready should show blocker and independent (not blocked)
	result, err = mustRunJSON(r, sandbox, "ready")
	if err != nil {
		return "", err
	}
	section(&out, "ready issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Blocked should show the blocked issue
	result, err = mustRunJSON(r, sandbox, "blocked")
	if err != nil {
		return "", err
	}
	section(&out, "blocked issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Close blocker
	_, err = mustRunJSON(r, sandbox, "close", blockerID)
	if err != nil {
		return "", err
	}

	// Ready should now include previously blocked
	result, err = mustRunJSON(r, sandbox, "ready")
	if err != nil {
		return "", err
	}
	section(&out, "ready after closing blocker", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Blocked should be empty
	result = r.RunJSON(sandbox, "blocked")
	section(&out, "blocked after closing blocker", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// 12: Search by title and description.
func caseSearch(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issues
	_, err := mustRunJSON(r, sandbox, "create", "Fix authentication bug", "--description", "Login fails for OAuth users")
	if err != nil {
		return "", err
	}

	_, err = mustRunJSON(r, sandbox, "create", "Add search feature", "--description", "Full text search needed")
	if err != nil {
		return "", err
	}

	result, err := mustRunJSON(r, sandbox, "create", "Update OAuth library")
	if err != nil {
		return "", err
	}
	oauthID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Close one to test --all
	_, err = mustRunJSON(r, sandbox, "close", oauthID)
	if err != nil {
		return "", err
	}

	// Search open only (default)
	result, err = mustRunJSON(r, sandbox, "search", "OAuth")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth open only", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search all
	result, err = mustRunJSON(r, sandbox, "search", "OAuth", "--all")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth all", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search title only
	result, err = mustRunJSON(r, sandbox, "search", "OAuth", "--title-only")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth title only", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search by description content
	result, err = mustRunJSON(r, sandbox, "search", "Full text")
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
	result, err := mustRunJSON(r, sandbox, "create", "Open task one")
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Open task two")
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "In progress task")
	if err != nil {
		return "", err
	}
	ipID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}
	_, err = mustRunJSON(r, sandbox, "update", ipID, "--status", "in-progress")
	if err != nil {
		return "", err
	}

	result, err = mustRunJSON(r, sandbox, "create", "Closed task")
	if err != nil {
		return "", err
	}
	closedID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}
	_, err = mustRunJSON(r, sandbox, "close", closedID)
	if err != nil {
		return "", err
	}

	// Get stats
	result, err = mustRunJSON(r, sandbox, "stats")
	if err != nil {
		return "", err
	}
	section(&out, "stats", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
