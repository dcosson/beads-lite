package reference

import "strings"

// 18: Auto-close parent — closing last child auto-closes parent, multi-level.
func caseAutoClose(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Enable auto-close (sandbox disables it by default for reference compat).
	_, err := mustRun(r, sandbox, "config", "set", "graph.auto_close_parent", "true", "--json")
	if err != nil {
		return "", err
	}

	// Create a 2-level hierarchy: grandparent > parent > tasks.
	result, err := mustRun(r, sandbox, "create", "Project Alpha", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	grandparent, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Phase 1", "--type", "epic", "--parent", grandparent, "--json")
	if err != nil {
		return "", err
	}
	parent, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Task A", "--parent", parent, "--json")
	if err != nil {
		return "", err
	}
	taskA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Task B", "--parent", parent, "--json")
	if err != nil {
		return "", err
	}
	taskB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Close Task A — parent should still be open (Task B remains).
	result, err = mustRun(r, sandbox, "close", taskA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "close task A", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify parent is still open.
	result, err = mustRun(r, sandbox, "show", parent, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "parent still open", n.NormalizeJSON([]byte(result.Stdout)))

	// Close Task B — parent should auto-close, grandparent should also auto-close
	// (multi-level recursion since parent was grandparent's only child).
	result, err = mustRun(r, sandbox, "close", taskB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "close task B triggers auto-close", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify parent is now closed.
	result, err = mustRun(r, sandbox, "show", parent, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "parent auto-closed", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify grandparent is also closed (multi-level).
	result, err = mustRun(r, sandbox, "show", grandparent, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "grandparent auto-closed", n.NormalizeJSON([]byte(result.Stdout)))

	// Test: auto-close respects config flag. Create new hierarchy with auto-close off.
	_, err = mustRun(r, sandbox, "config", "set", "graph.auto_close_parent", "false", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Project Beta", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	parent2, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Only child", "--parent", parent2, "--json")
	if err != nil {
		return "", err
	}
	onlyChild, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Close the only child — parent should NOT auto-close since flag is off.
	_, err = mustRun(r, sandbox, "close", onlyChild, "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "show", parent2, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "parent stays open when auto-close off", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
