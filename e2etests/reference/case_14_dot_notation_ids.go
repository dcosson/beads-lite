package reference

import "strings"

// 14: Dot-notation hierarchical child ID lifecycle.
func caseDotNotationIDs(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create parent issue
	result, err := mustRun(r, sandbox, "create", "Parent task", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create parent", n.NormalizeJSON([]byte(result.Stdout)))
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create first child via --parent (should get parent.1)
	result, err = mustRun(r, sandbox, "create", "First child", "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create child 1 with --parent", n.NormalizeJSON([]byte(result.Stdout)))
	child1ID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create second child via --parent (should get parent.2)
	result, err = mustRun(r, sandbox, "create", "Second child", "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create child 2 with --parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Create grandchild via --parent on child1 (should get parent.1.1)
	result, err = mustRun(r, sandbox, "create", "Grandchild", "--parent", child1ID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create grandchild with --parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show parent — should list children
	result, err = mustRun(r, sandbox, "show", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show parent with children", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child1 — should show parent and grandchild
	result, err = mustRun(r, sandbox, "show", child1ID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show child1 with parent and grandchild", n.NormalizeJSON([]byte(result.Stdout)))

	// Delete child1 then create child3 — should get parent.3 (not reuse .1)
	result, err = mustRun(r, sandbox, "delete", child1ID, "--force")
	if err != nil {
		return "", err
	}
	section(&out, "delete child 1", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "create", "Third child", "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create child 3 after deletion (no reuse)", n.NormalizeJSON([]byte(result.Stdout)))

	// List children of parent
	result, err = mustRun(r, sandbox, "children", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list children after deletion", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}
