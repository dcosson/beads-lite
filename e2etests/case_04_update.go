package e2etests

import "strings"

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
