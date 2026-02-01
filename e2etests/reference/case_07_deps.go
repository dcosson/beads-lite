package reference

import "strings"

// 07: Dependency lifecycle (add, list, remove, verify symmetry).
func caseDeps(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create two issues
	result, err := mustRun(r, sandbox, "create", "Issue A", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create issue A", n.NormalizeJSON([]byte(result.Stdout)))
	idA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue B", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create issue B", n.NormalizeJSON([]byte(result.Stdout)))
	idB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Add dep: A depends on B (A is blocked by B)
	result, err = mustRun(r, sandbox, "dep", "add", idA, idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add dependency A depends on B", n.NormalizeJSON([]byte(result.Stdout)))

	// Show A has depends_on
	result, err = mustRun(r, sandbox, "show", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show A has depends_on", n.NormalizeJSON([]byte(result.Stdout)))

	// Show B has dependents
	result, err = mustRun(r, sandbox, "show", idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show B has dependents", n.NormalizeJSON([]byte(result.Stdout)))

	// List all - verify dependencies array is populated
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list shows dependencies", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Dep list A
	result, err = mustRun(r, sandbox, "dep", "list", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "dep list A", n.NormalizeJSON([]byte(result.Stdout)))

	// Remove dependency
	result, err = mustRun(r, sandbox, "dep", "remove", idA, idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "remove dependency", n.NormalizeJSON([]byte(result.Stdout)))

	// Show A after removal
	result, err = mustRun(r, sandbox, "show", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show A after dep removal", n.NormalizeJSON([]byte(result.Stdout)))

	// List after removal - verify dependencies array is empty
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after dep removal", n.NormalizeJSONSorted([]byte(result.Stdout)))

	return out.String(), nil
}
