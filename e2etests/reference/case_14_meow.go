package reference

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 14: Full MEOW lifecycle — cook, pour, mol current, claim, close --continue, burn, wisp, squash.
func caseMeow(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Meow commands require --no-daemon with the reference binary.
	// beads-lite accepts it as a no-op, so this is safe for both.
	prevExtraArgs := r.ExtraArgs
	r.ExtraArgs = append(append([]string{}, r.ExtraArgs...), "--no-daemon")
	defer func() { r.ExtraArgs = prevExtraArgs }()

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

	// 3b. Mol show — inspect molecule after pour.
	result, err = mustRun(r, sandbox, "mol", "show", rootID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "mol show after pour", n.NormalizeJSON([]byte(result.Stdout)))

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

	// 5b. Mol show — inspect molecule mid-workflow (step 1 closed, step 2 claimed).
	result, err = mustRun(r, sandbox, "mol", "show", rootID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "mol show mid-workflow", n.NormalizeJSON([]byte(result.Stdout)))

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

	// Mol show — inspect molecule after all steps closed.
	result, err = mustRun(r, sandbox, "mol", "show", rootID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "mol show complete", n.NormalizeJSON([]byte(result.Stdout)))

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
// Both the reference binary and beads-lite use the same format:
// {new_epic_id, id_mapping, created, attached, phase}.
func extractPourResult(jsonOutput string) (pourResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &raw); err != nil {
		return pourResult{}, fmt.Errorf("parsing pour/wisp result: %w\nraw: %s", err, jsonOutput)
	}

	epicID, ok := raw["new_epic_id"].(string)
	if !ok || epicID == "" {
		return pourResult{}, fmt.Errorf("missing new_epic_id in pour/wisp output, raw: %s", jsonOutput)
	}

	idMapping, _ := raw["id_mapping"].(map[string]interface{})
	stepIDs := make(map[string]string)

	// Find the formula name (the key that maps to the root ID).
	var formulaName string
	for key, val := range idMapping {
		if id, ok := val.(string); ok && id == epicID {
			formulaName = key
			break
		}
	}
	// Extract child step IDs: keys are "formula.step_name".
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

// patchMeowExpected transforms the reference binary's expected output for
// sections where beads-lite intentionally differs. Applied after -update
// generates the expected file from the reference binary.
func patchMeowExpected(s string) string {
	sections := splitMeowSections(s)
	for i := range sections {
		if sections[i].Name != "close step 1 continue" && sections[i].Name != "close step 2 continue" {
			continue
		}
		updated, ok := patchContinueNextStepStatus(sections[i].Content)
		if ok {
			sections[i].Content = updated
		}
	}
	for i := range sections {
		if sections[i].Name != "burn molecule" {
			continue
		}
		updated, ok := patchBurnEventsRemoved(sections[i].Content)
		if ok {
			sections[i].Content = updated
		}
	}
	var out strings.Builder
	for _, sec := range sections {
		section(&out, sec.Name, sec.Content)
	}
	return out.String()
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

func patchContinueNextStepStatus(content string) (string, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return content, false
	}
	continueBlock, ok := raw["continue"].(map[string]interface{})
	if !ok {
		return content, false
	}
	nextStep, ok := continueBlock["next_step"].(map[string]interface{})
	if !ok {
		return content, false
	}
	if status, ok := nextStep["status"].(string); ok && status == "open" {
		nextStep["status"] = "in_progress"
		continueBlock["next_step"] = nextStep
		raw["continue"] = continueBlock
	}
	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return content, false
	}
	return string(updated), true
}

func patchBurnEventsRemoved(content string) (string, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return content, false
	}
	if _, ok := raw["events_removed"]; !ok {
		return content, false
	}
	raw["events_removed"] = 4
	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return content, false
	}
	return string(updated), true
}

type meowSection struct {
	Name    string
	Content string
}

func splitMeowSections(s string) []meowSection {
	lines := strings.Split(s, "\n")
	var sections []meowSection
	var current *meowSection
	var contentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
			if current != nil {
				current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
				sections = append(sections, *current)
			}
			name := strings.TrimSuffix(strings.TrimPrefix(line, "=== "), " ===")
			current = &meowSection{Name: name}
			contentLines = nil
			continue
		}
		if current != nil {
			contentLines = append(contentLines, line)
		}
	}
	if current != nil {
		current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
		sections = append(sections, *current)
	}
	return sections
}
