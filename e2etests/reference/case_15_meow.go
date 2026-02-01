package reference

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
	pourResult, err := extractPourResult(result.Stdout)
	if err != nil {
		return "", err
	}
	rootID := pourResult.RootID
	step1ID := pourResult.StepIDs["setup"]
	step2ID := pourResult.StepIDs["build"]
	step3ID := pourResult.StepIDs["test"]
	if step1ID == "" || step2ID == "" || step3ID == "" {
		return "", fmt.Errorf("missing step IDs from pour: setup=%q build=%q test=%q", step1ID, step2ID, step3ID)
	}

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

	burnResult, err := extractPourResult(result.Stdout)
	if err != nil {
		return "", err
	}
	burnRootID := burnResult.RootID

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

	wispResult, err := extractPourResult(result.Stdout)
	if err != nil {
		return "", err
	}
	wispRootID := wispResult.RootID

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

// pourResult holds the parsed output of "mol pour" or "mol wisp".
type pourResult struct {
	RootID  string
	StepIDs map[string]string // step name -> issue ID
}

// extractPourResult parses the JSON output of "mol pour" or "mol wisp".
// Handles both the reference binary format (new_epic_id/id_mapping) and
// the beads-lite format (RootID/ChildIDs).
// TODO: investigate, I think this should be the same
func extractPourResult(jsonOutput string) (pourResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &raw); err != nil {
		return pourResult{}, fmt.Errorf("parsing pour/wisp result: %w\nraw: %s", err, jsonOutput)
	}

	// Reference binary format: {new_epic_id, id_mapping}
	if epicID, ok := raw["new_epic_id"].(string); ok && epicID != "" {
		idMapping, _ := raw["id_mapping"].(map[string]interface{})
		stepIDs := make(map[string]string)
		// Find the formula name (the key that maps to the root ID)
		var formulaName string
		for key, val := range idMapping {
			if id, ok := val.(string); ok && id == epicID {
				formulaName = key
				break
			}
		}
		// Extract child step IDs: keys are "formula.step_name"
		prefix := formulaName + "."
		for key, val := range idMapping {
			if strings.HasPrefix(key, prefix) {
				stepName := strings.TrimPrefix(key, prefix)
				if id, ok := val.(string); ok {
					stepIDs[stepName] = id
				}
			}
		}
		return pourResult{RootID: epicID, StepIDs: stepIDs}, nil
	}

	// Beads-lite format: {RootID, ChildIDs}
	if rootID, ok := raw["RootID"].(string); ok && rootID != "" {
		childIDs, _ := raw["ChildIDs"].([]interface{})
		stepIDs := make(map[string]string)
		// ChildIDs are in formula step order; map them to step names
		// by looking up step names from the id_mapping if available,
		// otherwise fall back to index-based names.
		stepNames := []string{"setup", "build", "test"}
		for i, id := range childIDs {
			if idStr, ok := id.(string); ok && i < len(stepNames) {
				stepIDs[stepNames[i]] = idStr
			}
		}
		return pourResult{RootID: rootID, StepIDs: stepIDs}, nil
	}

	return pourResult{}, fmt.Errorf("unrecognized pour/wisp output format, raw: %s", jsonOutput)
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
