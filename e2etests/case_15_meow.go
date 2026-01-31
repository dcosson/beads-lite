package e2etests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 15: Full MEOW lifecycle — cook, pour, mol current, claim, close --continue, burn, wisp, squash.
func caseMeow(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// 1. Formula setup — write test formula into sandbox .beads/formulas/ directory.
	formulaDir := filepath.Join(sandbox, ".beads", "formulas")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		return "", fmt.Errorf("creating formula dir: %w", err)
	}
	formulaJSON := `{
  "formula": "test-workflow",
  "description": "Test workflow",
  "version": 1,
  "type": "workflow",
  "vars": {"name": {"default": "world"}},
  "steps": [
    {"id": "setup", "title": "Setup {{name}}", "type": "task"},
    {"id": "build", "title": "Build", "type": "task", "depends_on": ["setup"]},
    {"id": "test", "title": "Test", "type": "task", "depends_on": ["build"]}
  ]
}`
	if err := os.WriteFile(filepath.Join(formulaDir, "test-workflow.formula.json"), []byte(formulaJSON), 0o644); err != nil {
		return "", fmt.Errorf("writing formula: %w", err)
	}

	// Set BD_ACTOR=testuser for claim/close/current commands.
	// Runner.Run() passes os.Environ() to child processes, so this flows through.
	os.Setenv("BD_ACTOR", "testuser")
	defer os.Unsetenv("BD_ACTOR")

	// 2. Cook — dry-run preview.
	result, err := mustRun(r, sandbox, "cook", "test-workflow", "--var", "name=demo", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "cook preview", n.NormalizeJSON([]byte(result.Stdout)))

	// 3. Pour — create molecule.
	result, err = mustRun(r, sandbox, "mol", "pour", "test-workflow", "--var", "name=demo", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "pour molecule", n.NormalizeJSON([]byte(result.Stdout)))

	// Extract IDs from pour result for subsequent commands.
	rootID, childIDs, err := extractPourIDs(result.Stdout)
	if err != nil {
		return "", err
	}
	if len(childIDs) != 3 {
		return "", fmt.Errorf("expected 3 child IDs from pour, got %d", len(childIDs))
	}
	step1ID := childIDs[0] // setup
	step2ID := childIDs[1] // build
	step3ID := childIDs[2] // test

	// 4. Mol current — show all steps with status markers.
	result, err = mustRun(r, sandbox, "mol", "current", rootID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "mol current", n.NormalizeJSON([]byte(result.Stdout)))

	// 5. Claim + navigate workflow.

	// Claim root epic.
	result, err = mustRun(r, sandbox, "update", rootID, "--claim", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "claim root", n.NormalizeJSON([]byte(result.Stdout)))

	// Claim first ready step (setup).
	result, err = mustRun(r, sandbox, "update", step1ID, "--claim", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "claim step 1", n.NormalizeJSON([]byte(result.Stdout)))

	// Close step 1, verify step 2 auto-advances.
	result, err = mustRun(r, sandbox, "close", step1ID, "--continue", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "close step 1 continue", n.NormalizeJSON([]byte(result.Stdout)))

	// Close step 2, verify step 3 advances.
	result, err = mustRun(r, sandbox, "close", step2ID, "--continue", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "close step 2 continue", n.NormalizeJSON([]byte(result.Stdout)))

	// Close final step.
	result, err = mustRun(r, sandbox, "close", step3ID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "close step 3", n.NormalizeJSON([]byte(result.Stdout)))

	// Mol progress — verify 100% completion.
	result, err = mustRun(r, sandbox, "mol", "progress", rootID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "mol progress complete", n.NormalizeJSON([]byte(result.Stdout)))

	// 6. Burn — pour a second molecule then burn it.
	result, err = mustRun(r, sandbox, "mol", "pour", "test-workflow", "--var", "name=demo", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "pour for burn", n.NormalizeJSON([]byte(result.Stdout)))

	burnRootID, _, err := extractPourIDs(result.Stdout)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "mol", "burn", burnRootID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "burn molecule", n.NormalizeJSON([]byte(result.Stdout)))

	// 7. Wisp + squash.

	// Wisp — pour ephemeral molecule.
	result, err = mustRun(r, sandbox, "mol", "wisp", "test-workflow", "--var", "name=ephemeral", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "wisp molecule", n.NormalizeJSON([]byte(result.Stdout)))

	wispRootID, _, err := extractPourIDs(result.Stdout)
	if err != nil {
		return "", err
	}

	// Ready — verify wisp steps excluded from ready output.
	result, err = mustRun(r, sandbox, "ready", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "ready excludes wisp", n.NormalizeJSONSorted([]byte(result.Stdout)))

	// Squash wisp into digest.
	result, err = mustRun(r, sandbox, "mol", "squash", wispRootID, "--summary", "Test summary", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "squash wisp", n.NormalizeJSON([]byte(result.Stdout)))

	// Extract digest ID and show it.
	digestID, err := extractSquashDigestID(result.Stdout)
	if err != nil {
		return "", err
	}

	result, err = mustRun(r, sandbox, "show", digestID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show digest", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}

// extractPourIDs parses the JSON output of "mol pour" or "mol wisp" and
// returns the root ID and child IDs.
func extractPourIDs(jsonOutput string) (string, []string, error) {
	var result struct {
		RootID   string   `json:"RootID"`
		ChildIDs []string `json:"ChildIDs"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		return "", nil, fmt.Errorf("parsing pour/wisp result: %w\nraw: %s", err, jsonOutput)
	}
	if result.RootID == "" {
		return "", nil, fmt.Errorf("pour/wisp result has empty RootID, raw: %s", jsonOutput)
	}
	return result.RootID, result.ChildIDs, nil
}

// extractSquashDigestID parses the JSON output of "mol squash" and returns
// the digest issue ID.
func extractSquashDigestID(jsonOutput string) (string, error) {
	var result struct {
		DigestID string `json:"digest_id"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		return "", fmt.Errorf("parsing squash result: %w\nraw: %s", err, jsonOutput)
	}
	if result.DigestID == "" {
		return "", fmt.Errorf("squash result has empty digest_id, raw: %s", jsonOutput)
	}
	return result.DigestID, nil
}
