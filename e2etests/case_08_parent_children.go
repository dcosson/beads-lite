package e2etests

import "strings"

// 09: Parent/children lifecycle.
func caseParentChildren(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create parent
	result, err := mustRun(r, sandbox, "create", "Parent issue", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create parent", n.NormalizeJSON([]byte(result.Stdout)))
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create child
	result, err = mustRun(r, sandbox, "create", "Child issue", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create child", n.NormalizeJSON([]byte(result.Stdout)))
	childID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Set parent via update
	result, err = mustRun(r, sandbox, "update", childID, "--parent", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "set parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child has parent
	result, err = mustRun(r, sandbox, "show", childID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show child has parent", n.NormalizeJSON([]byte(result.Stdout)))

	// List all - verify parent-child dependency in list output
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list shows parent-child deps", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List children of parent
	result, err = mustRun(r, sandbox, "children", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list children of parent", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Remove parent via update
	result, err = mustRun(r, sandbox, "update", childID, "--parent", "", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "remove parent", n.NormalizeJSON([]byte(result.Stdout)))

	// Show child after removal
	result, err = mustRun(r, sandbox, "show", childID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show child after parent removal", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
