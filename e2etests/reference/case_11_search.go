package reference

import "strings"

// 11: Search by title and description.
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

	// Close one to test status filtering
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

	// Search closed
	result, err = mustRun(r, sandbox, "search", "OAuth", "--status", "closed", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "search OAuth closed", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Search by description content
	result, err = mustRun(r, sandbox, "search", "Full text", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "search by description", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}
