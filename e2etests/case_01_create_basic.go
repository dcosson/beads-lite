package e2etests

import "strings"

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
