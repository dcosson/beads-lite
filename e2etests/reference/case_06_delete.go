package reference

import "strings"

// 06: Delete — soft (tombstone), hard, cascade, dry-run.
func caseDelete(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// --- Part 1: Soft delete (tombstone) ---

	result, err := mustRun(r, sandbox, "create", "Issue to tombstone", "--json")
	if err != nil {
		return "", err
	}
	tombstoneID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", tombstoneID, "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "soft delete", n.NormalizeJSON([]byte(result.Stdout)))

	showResult := r.Run(sandbox, "show", tombstoneID, "--json")
	sectionExitCode(&out, "show tombstoned issue", showResult.ExitCode)

	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after soft delete", n.NormalizeJSONSorted([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "list", "--status", "tombstone", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list tombstones", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// --- Part 2: Hard delete ---

	result, err = mustRun(r, sandbox, "create", "Issue to hard delete", "--json")
	if err != nil {
		return "", err
	}
	hardDeleteID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", hardDeleteID, "--force", "--hard", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "hard delete", n.NormalizeJSON([]byte(result.Stdout)))

	showResult = r.Run(sandbox, "show", hardDeleteID, "--json")
	sectionExitCode(&out, "show hard-deleted issue", showResult.ExitCode)

	// --- Part 3: Delete with --reason ---

	result, err = mustRun(r, sandbox, "create", "Issue with reason", "--json")
	if err != nil {
		return "", err
	}
	reasonID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", reasonID, "--force", "--reason", "duplicate of other-id", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "delete with reason", n.NormalizeJSON([]byte(result.Stdout)))

	showResult = r.Run(sandbox, "show", reasonID, "--json")
	sectionExitCode(&out, "show issue with reason", showResult.ExitCode)

	// --- Part 4: Cascade hard delete ---
	// Graph: A->B->C, D->C, E->A, F->E, F->G (-> = "depends on")
	// Cascade-deleting A should remove A, E, F but not B, C, D, G.

	result, err = mustRun(r, sandbox, "create", "Issue A", "--json")
	if err != nil {
		return "", err
	}
	idA, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue B", "--json")
	if err != nil {
		return "", err
	}
	idB, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue C", "--json")
	if err != nil {
		return "", err
	}
	idC, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue D", "--json")
	if err != nil {
		return "", err
	}
	idD, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue E", "--json")
	if err != nil {
		return "", err
	}
	idE, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue F", "--json")
	if err != nil {
		return "", err
	}
	idF, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Issue G", "--json")
	if err != nil {
		return "", err
	}
	idG, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

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

	result, err = mustRun(r, sandbox, "delete", idA, "--cascade", "--force", "--hard", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "cascade hard delete A", n.NormalizeJSON([]byte(result.Stdout)))

	// A, E, F should be gone
	showResult = r.Run(sandbox, "show", idA, "--json")
	section(&out, "show A after cascade", n.NormalizeJSON([]byte(showResult.Stdout)))

	showResult = r.Run(sandbox, "show", idE, "--json")
	section(&out, "show E after cascade", n.NormalizeJSON([]byte(showResult.Stdout)))

	showResult = r.Run(sandbox, "show", idF, "--json")
	section(&out, "show F after cascade", n.NormalizeJSON([]byte(showResult.Stdout)))

	// B, G should still exist
	result, err = mustRun(r, sandbox, "show", idB, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show B after cascade (still exists)", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "show", idG, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show G after cascade (still exists)", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after cascade", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// --- Part 5: Cascade soft delete ---

	result, err = mustRun(r, sandbox, "create", "Cascade parent", "--json")
	if err != nil {
		return "", err
	}
	cascadeParentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Cascade child", "--json")
	if err != nil {
		return "", err
	}
	cascadeChildID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	_, err = mustRun(r, sandbox, "dep", "add", cascadeChildID, cascadeParentID, "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", cascadeParentID, "--cascade", "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "cascade soft delete", n.NormalizeJSON([]byte(result.Stdout)))

	showResult = r.Run(sandbox, "show", cascadeParentID, "--json")
	sectionExitCode(&out, "show parent after cascade soft delete", showResult.ExitCode)

	showResult = r.Run(sandbox, "show", cascadeChildID, "--json")
	sectionExitCode(&out, "show child after cascade soft delete", showResult.ExitCode)

	// --- Part 6: Dry run ---

	result, err = mustRun(r, sandbox, "create", "Dry run target", "--json")
	if err != nil {
		return "", err
	}
	dryRunID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", dryRunID, "--dry-run", "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "dry run", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "show", dryRunID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show after dry run", n.NormalizeJSON([]byte(result.Stdout)))

	// --- Part 7: Update rejects tombstone status ---

	result, err = mustRun(r, sandbox, "create", "Status test", "--json")
	if err != nil {
		return "", err
	}
	statusTestID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	updateResult := r.Run(sandbox, "update", statusTestID, "--status", "tombstone")
	sectionExitCode(&out, "update --status tombstone", updateResult.ExitCode)

	return out.String(), nil
}
