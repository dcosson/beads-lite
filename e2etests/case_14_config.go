package e2etests

import "strings"

// 14: Config get/set/list/unset/validate lifecycle.
func caseConfig(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// Set a core config value
	result, err := mustRun(r, sandbox, "config", "set", "defaults.priority", "high", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config set", n.NormalizeJSON([]byte(result.Stdout)))

	// Get the value back
	result, err = mustRun(r, sandbox, "config", "get", "defaults.priority", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config get", n.NormalizeJSON([]byte(result.Stdout)))

	// List all config values
	result, err = mustRun(r, sandbox, "config", "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config list", n.NormalizeJSON([]byte(result.Stdout)))

	// Set a custom key
	result, err = mustRun(r, sandbox, "config", "set", "custom.key", "myval", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config set custom", n.NormalizeJSON([]byte(result.Stdout)))

	// Unset the custom key
	result, err = mustRun(r, sandbox, "config", "unset", "custom.key", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config unset", n.NormalizeJSON([]byte(result.Stdout)))

	// Validate config (should pass)
	result, err = mustRun(r, sandbox, "config", "validate", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config validate", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
