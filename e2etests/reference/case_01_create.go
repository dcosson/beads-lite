package reference

import "strings"

// 01: Create issues — basic and with all supported flags.
func caseCreate(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// --- Basic create, show, list ---

	result, err := mustRun(r, sandbox, "create", "Fix login bug", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create basic task", n.NormalizeJSON([]byte(result.Stdout)))

	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show the created issue", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list all issues", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// --- Create with all flags ---

	result, err = mustRun(r, sandbox, "create", "Dependency target", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create dependency target", n.NormalizeJSON([]byte(result.Stdout)))

	depID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Parent issue", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create parent issue", n.NormalizeJSON([]byte(result.Stdout)))

	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Full featured issue",
		"--type", "feature",
		"--priority", "1",
		"--description", "A detailed description",
		"--label", "urgent",
		"--label", "v2",
		"--assignee", "alice",
		"--deps", depID,
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

	result, err = mustRun(r, sandbox, "show", fullID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show issue created with flags", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
