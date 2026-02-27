package reference

import "strings"

// 17: Cascade parent blocking — ready and blocked respect inherited blocks.
func caseCascadeReady(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Ensure cascade is enabled (it's the default, but be explicit).
	_, err := mustRun(r, sandbox, "config", "set", "graph.cascade_parent_blocking", "true", "--json")
	if err != nil {
		return "", err
	}

	// Create two epics: Epic B blocks Epic A.
	result, err := mustRun(r, sandbox, "create", "Foundation", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	epicB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Application", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	epicA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	_, err = mustRun(r, sandbox, "dep", "add", epicA, epicB, "--json")
	if err != nil {
		return "", err
	}

	// Tasks under Epic B.
	result, err = mustRun(r, sandbox, "create", "Setup database", "--parent", epicB, "--json")
	if err != nil {
		return "", err
	}
	taskB1, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Tasks under Epic A (these should be parent-blocked because Epic A depends on Epic B).
	_, err = mustRun(r, sandbox, "create", "Build auth", "--parent", epicA, "--json")
	if err != nil {
		return "", err
	}

	_, err = mustRun(r, sandbox, "create", "Build API", "--parent", epicA, "--json")
	if err != nil {
		return "", err
	}

	// A standalone task (unblocked).
	_, err = mustRun(r, sandbox, "create", "Fix typo", "--json")
	if err != nil {
		return "", err
	}

	// Ready with cascade=true — children of Epic A should NOT appear.
	result, err = mustRun(r, sandbox, "ready", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "ready cascade on", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Blocked with cascade=true — children of Epic A should show inherited blockers.
	result, err = mustRun(r, sandbox, "blocked", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "blocked cascade on", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Show a parent-blocked task — should have inherited blocks section.
	// Use the show of epicA to check inherited blocks display.
	result, err = mustRun(r, sandbox, "show", epicA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show blocked epic", n.NormalizeJSON([]byte(result.Stdout)))

	// Now disable cascade and verify children of Epic A become "ready".
	_, err = mustRun(r, sandbox, "config", "set", "graph.cascade_parent_blocking", "false", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "ready", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "ready cascade off", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Re-enable cascade.
	_, err = mustRun(r, sandbox, "config", "set", "graph.cascade_parent_blocking", "true", "--json")
	if err != nil {
		return "", err
	}

	// Close the blocker task in Epic B — Epic A's children should now be ready.
	_, err = mustRun(r, sandbox, "close", taskB1, "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "ready", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "ready after closing blocker", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}
