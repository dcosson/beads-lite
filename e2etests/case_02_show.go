package e2etests

import "strings"

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
