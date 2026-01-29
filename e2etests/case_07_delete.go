package e2etests

import "strings"

// 07: Delete with --force and --cascade.
func caseDelete(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// --- Part 1: Simple delete ---
	// Create two issues
	result, err := mustRun(r, sandbox, "create", "Keeper", "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Deletable", "--json")
	if err != nil {
		return "", err
	}
	deleteID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Delete the second
	result, err = mustRun(r, sandbox, "delete", deleteID, "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "delete issue", n.NormalizeJSON([]byte(result.Stdout)))

	// List should only show first
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after delete", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Show deleted should fail
	showResult := r.Run(sandbox, "show", deleteID, "--json")
	sectionExitCode(&out, "show deleted issue", showResult.ExitCode)

	// --- Part 2: Cascade delete ---
	// Create issues: A, B, C, D, E, F, G with dependency graph:
	//   A depends on B, B depends on C, D depends on C
	//   E depends on A, F depends on E, F depends on G
	// Deleting A with cascade should delete A, E, F (the dependent chain)

	result, err = mustRun(r, sandbox, "create", "Issue A", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create A", n.NormalizeJSON([]byte(result.Stdout)))
	idA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue B", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create B", n.NormalizeJSON([]byte(result.Stdout)))
	idB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue C", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create C", n.NormalizeJSON([]byte(result.Stdout)))
	idC, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue D", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create D", n.NormalizeJSON([]byte(result.Stdout)))
	idD, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue E", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create E", n.NormalizeJSON([]byte(result.Stdout)))
	idE, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue F", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create F", n.NormalizeJSON([]byte(result.Stdout)))
	idF, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue G", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create G", n.NormalizeJSON([]byte(result.Stdout)))
	idG, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Set up dependencies
	_, err = mustRun(r, sandbox, "dep", "add", idA, idB, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", idB, idC, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", idD, idC, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", idE, idA, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", idF, idE, "--json")
	if err != nil {
		return "", err
	}
	_, err = mustRun(r, sandbox, "dep", "add", idF, idG, "--json")
	if err != nil {
		return "", err
	}

	// Show state before cascade delete
	result, err = mustRun(r, sandbox, "show", idA, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show A before cascade", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "show", idE, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show E before cascade (depends on A)", n.NormalizeJSON([]byte(result.Stdout)))

	// Cascade delete A
	result, err = mustRun(r, sandbox, "delete", idA, "--cascade", "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "cascade delete A", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify A, E, F are deleted (show returns empty)
	showResult = r.Run(sandbox, "show", idA, "--json")
	section(&out, "show A after cascade", n.NormalizeJSON([]byte(showResult.Stdout)))

	showResult = r.Run(sandbox, "show", idE, "--json")
	section(&out, "show E after cascade", n.NormalizeJSON([]byte(showResult.Stdout)))

	showResult = r.Run(sandbox, "show", idF, "--json")
	section(&out, "show F after cascade", n.NormalizeJSON([]byte(showResult.Stdout)))

	// B should still exist with A removed from dependents
	result, err = mustRun(r, sandbox, "show", idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show B after cascade (A removed)", n.NormalizeJSON([]byte(result.Stdout)))

	// G should still exist with F removed from dependents
	result, err = mustRun(r, sandbox, "show", idG, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show G after cascade (F removed)", n.NormalizeJSON([]byte(result.Stdout)))

	// List all remaining issues after cascade
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after cascade", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Suppress unused variable warnings
	_ = idC
	_ = idD

	return out.String(), nil
}
