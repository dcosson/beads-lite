package e2etests

import "strings"

// 05: List with various filters.
func caseList(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create a parent epic first
	result, err := mustRun(r, sandbox, "create", "Parent epic", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create issues with various attributes
	result, err = mustRun(r, sandbox, "create", "Open task", "--type", "task", "--priority", "2", "--json")
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

	// Create a fully-populated issue with assignee, labels, description, parent
	result, err = mustRun(r, sandbox, "create", "Feature request",
		"--type", "feature",
		"--priority", "3",
		"--assignee", "alice",
		"--label", "urgent",
		"--label", "v2",
		"--description", "This is a detailed description",
		"--parent", parentID,
		"--json")
	if err != nil {
		return "", err
	}

	// Close one issue
	_, err = mustRun(r, sandbox, "close", bugID, "--json")
	if err != nil {
		return "", err
	}

	// List all open (default) - shows what fields appear in list output
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

	// List closed (use --status closed for original beads compatibility)
	result, err = mustRun(r, sandbox, "list", "--status", "closed", "--json")
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

	// List by priority (use P1 format for original beads compatibility)
	result, err = mustRun(r, sandbox, "list", "--priority", "P1", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list by priority P1", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}
