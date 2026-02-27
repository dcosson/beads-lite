package reference

import "strings"

// 16: bd graph — cross-parent blocking visualization with waves.
func caseGraphCrossParent(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create two parent epics: Parent B blocks Parent A.
	result, err := mustRun(r, sandbox, "create", "Setup Infrastructure", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	parentB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Build Application", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	parentA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Parent A is blocked by Parent B.
	_, err = mustRun(r, sandbox, "dep", "add", parentA, parentB, "--json")
	if err != nil {
		return "", err
	}

	// Tasks under Parent B.
	result, err = mustRun(r, sandbox, "create", "Provision servers", "--parent", parentB, "--json")
	if err != nil {
		return "", err
	}
	b1, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	_, err = mustRun(r, sandbox, "create", "Configure monitoring", "--parent", parentB, "--json")
	if err != nil {
		return "", err
	}

	// b2 depends on b1.
	result, err = mustRun(r, sandbox, "create", "Deploy infrastructure", "--parent", parentB, "--json")
	if err != nil {
		return "", err
	}
	b3, err := mustExtractID(result)
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", b3, b1, "--json")
	if err != nil {
		return "", err
	}

	// Tasks under Parent A.
	_, err = mustRun(r, sandbox, "create", "Implement auth", "--parent", parentA, "--json")
	if err != nil {
		return "", err
	}

	_, err = mustRun(r, sandbox, "create", "Build API", "--parent", parentA, "--json")
	if err != nil {
		return "", err
	}

	// Also add a standalone task (no parent).
	_, err = mustRun(r, sandbox, "create", "Update docs", "--json")
	if err != nil {
		return "", err
	}

	// Graph global mode — text output shows parent ordering and parent-blocked annotations.
	result, err = mustRun(r, sandbox, "graph")
	if err != nil {
		return "", err
	}
	section(&out, "graph cross-parent text", n.normalizeText(result.Stdout))

	// Graph JSON with waves — full structured output including wave grouping.
	// Waves are tested via JSON (not text) to avoid non-deterministic ID ordering
	// within wave lines after normalization.
	result, err = mustRun(r, sandbox, "graph", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "graph cross-parent json", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
