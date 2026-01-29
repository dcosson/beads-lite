package e2etests

import "strings"

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
