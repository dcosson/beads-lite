package reference

import "strings"

// 15: bd graph — basic tree rendering with back-references and annotations.
func caseGraphBasic(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create a parent epic with child tasks.
	result, err := mustRun(r, sandbox, "create", "Setup Infrastructure", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Provision servers", "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	t1ID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Configure networking", "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	t2ID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Integration tests", "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	t3ID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// t2 depends on t1, t3 depends on both t1 and t2 (creates a DAG with back-reference).
	_, err = mustRun(r, sandbox, "dep", "add", t2ID, t1ID, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", t3ID, t1ID, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", t3ID, t2ID, "--json")
	if err != nil {
		return "", err
	}

	// Graph with parent ID — text output.
	result, err = mustRun(r, sandbox, "graph", parentID)
	if err != nil {
		return "", err
	}
	section(&out, "graph parent text", n.normalizeText(result.Stdout))

	// Graph with parent ID — JSON output.
	result, err = mustRun(r, sandbox, "graph", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "graph parent json", n.NormalizeJSON([]byte(result.Stdout)))

	// Graph global mode (no args) — text output.
	result, err = mustRun(r, sandbox, "graph")
	if err != nil {
		return "", err
	}
	section(&out, "graph global text", n.normalizeText(result.Stdout))

	return out.String(), nil
}
