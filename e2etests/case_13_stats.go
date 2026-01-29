package e2etests

import "strings"

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
	_, err = mustRun(r, sandbox, "update", ipID, "--status", "in_progress", "--json")
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
