package e2etests

import "strings"

// 02: Create with all supported flags.
func caseCreateWithFlags(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Create a dependency target first
	result, err := mustRun(r, sandbox, "create", "Dependency target", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create dependency target", n.NormalizeJSON([]byte(result.Stdout)))

	depID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create a parent
	result, err = mustRun(r, sandbox, "create", "Parent issue", "--type", "epic", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "create parent issue", n.NormalizeJSON([]byte(result.Stdout)))

	parentID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Create issue with all flags
	result, err = mustRun(r, sandbox, "create", "Full featured issue",
		"--type", "feature",
		"--priority", "1",
		"--description", "A detailed description",
		"--label", "urgent",
		"--label", "v2",
		"--assignee", "alice",
		"--depends-on", depID,
		"--parent", parentID,
		"--json",
	)
	if err != nil {
		return "", err
	}
	section(&out, "create with all flags", n.NormalizeJSON([]byte(result.Stdout)))

	fullID, err := mustExtractID(result)
	if err != nil {
		return "", err
	}

	// Show the fully created issue
	result, err = mustRun(r, sandbox, "show", fullID, "--json")
	if err != nil {
		return "", err
	}
	section(&out, "show the created issue", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
