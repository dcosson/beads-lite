package e2etests

import "strings"

// 17: Delete tombstone workflow — soft-delete, hard-delete, cascade, dry-run, reason.
func caseDeleteTombstone(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// --- Part a: Soft delete (default) ---
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

	// Show should still work and show tombstone status
	result, err = mustRun(r, sandbox, "show", tombstoneID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show tombstoned issue", n.NormalizeJSON([]byte(result.Stdout)))

	// List should NOT include tombstoned issue
	result, err = mustRun(r, sandbox, "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list after soft delete", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// List with --status tombstone SHOULD include it
	result, err = mustRun(r, sandbox, "list", "--status", "tombstone", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "list tombstones", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// --- Part b: Hard delete ---
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

	// Show should fail for hard-deleted issue
	showResult := r.Run(sandbox, "show", hardDeleteID, "--json")
	sectionExitCode(&out, "show hard-deleted issue", showResult.ExitCode)

	// --- Part c: Soft delete with --reason ---
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

	// Show should display the reason
	result, err = mustRun(r, sandbox, "show", reasonID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show issue with reason", n.NormalizeJSON([]byte(result.Stdout)))

	// --- Part d: Cascade soft delete ---
	result, err = mustRun(r, sandbox, "create", "Parent issue", "--json")
	if err != nil {
		return "", err
	}
	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "create", "Child issue", "--json")
	if err != nil {
		return "", err
	}
	childID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// child depends on parent
	_, err = mustRun(r, sandbox, "dep", "add", childID, parentID, "--json")
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", parentID, "--cascade", "--force", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "cascade soft delete", n.NormalizeJSON([]byte(result.Stdout)))

	// Both should be tombstoned
	result, err = mustRun(r, sandbox, "show", parentID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show parent after cascade", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "show", childID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show child after cascade", n.NormalizeJSON([]byte(result.Stdout)))

	// --- Part e: Dry run ---
	result, err = mustRun(r, sandbox, "create", "Dry run target", "--json")
	if err != nil {
		return "", err
	}
	dryRunID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "delete", dryRunID, "--dry-run", "--force")
	if err != nil {
		return "", err
	}
	section(&out, "dry run", n.normalizeText(result.Stdout))

	// Issue should still exist (not tombstoned)
	result, err = mustRun(r, sandbox, "show", dryRunID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show after dry run", n.NormalizeJSON([]byte(result.Stdout)))

	// --- Part f: Update rejects tombstone status ---
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
