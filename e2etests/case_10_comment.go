package e2etests

import "strings"

// 10: Comment add and list.
func caseComment(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create issue
	result, err := mustRun(r, sandbox, "create", "Commentable task", "--json")
	if err != nil {
		return "", err
	}
	id, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Add first comment
	result, err = mustRun(r, sandbox, "comment", "add", id, "First comment", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add first comment", n.NormalizeJSON([]byte(result.Stdout)))

	// Add second comment
	result, err = mustRun(r, sandbox, "comment", "add", id, "Second comment", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "add second comment", n.NormalizeJSON([]byte(result.Stdout)))

	// List comments
	result, err = mustRun(r, sandbox, "comment", "list", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list comments", n.NormalizeJSON([]byte(result.Stdout)))

	// Show issue with comments
	result, err = mustRun(r, sandbox, "show", id, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show issue with comments", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
