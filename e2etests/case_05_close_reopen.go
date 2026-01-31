package e2etests

import "strings"

// 05: Close and reopen lifecycle.
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
	result, err = mustRun(r, sandbox, "list", "--status", "closed", "--json")
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
