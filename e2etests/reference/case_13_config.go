package reference

import "strings"

// 13: Config lifecycle — set, get, list, unset, validate.
func caseConfig(r *Runner, n *Normalizer, sandbox string) (string, error) {
	var out strings.Builder

	// List empty config
	result, err := mustRun(r, sandbox, "config", "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config list empty", n.NormalizeJSON([]byte(result.Stdout)))

	// Get a key that is not set
	result, err = mustRun(r, sandbox, "config", "get", "actor", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config get actor (not set)", n.NormalizeJSON([]byte(result.Stdout)))

	// Set core keys
	result, err = mustRun(r, sandbox, "config", "set", "actor", "testuser", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config set actor", n.NormalizeJSON([]byte(result.Stdout)))

	result, err = mustRun(r, sandbox, "config", "set", "defaults.priority", "high", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config set defaults.priority", n.NormalizeJSON([]byte(result.Stdout)))

	// Set a custom key
	result, err = mustRun(r, sandbox, "config", "set", "custom.key", "myvalue", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config set custom key", n.NormalizeJSON([]byte(result.Stdout)))

	// Get a set key
	result, err = mustRun(r, sandbox, "config", "get", "actor", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config get actor (set)", n.NormalizeJSON([]byte(result.Stdout)))

	// List all config
	result, err = mustRun(r, sandbox, "config", "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config list with entries", n.NormalizeJSON([]byte(result.Stdout)))

	// Validate valid config
	result, err = mustRun(r, sandbox, "config", "validate", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config validate clean", n.NormalizeJSON([]byte(result.Stdout)))

	// Unset a key
	result, err = mustRun(r, sandbox, "config", "unset", "custom.key", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config unset custom key", n.NormalizeJSON([]byte(result.Stdout)))

	// Verify unset
	result, err = mustRun(r, sandbox, "config", "get", "custom.key", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config get custom key (after unset)", n.NormalizeJSON([]byte(result.Stdout)))

	// List after unset
	result, err = mustRun(r, sandbox, "config", "list", "--json")
	if err != nil {
		return "", err
	}
	section(&out, "config list after unset", n.NormalizeJSON([]byte(result.Stdout)))

	return out.String(), nil
}
