package meow

import (
	"fmt"
	"path/filepath"
)

// CookOutput is the resolved formula with source path, returned by Cook.
type CookOutput struct {
	*Formula
	Source string `json:"source"`
}

// Cook resolves a formula and returns the resolved formula with source path.
// This is a dry-run preview — no issues are created.
func Cook(name string, vars map[string]string, path FormulaSearchPath) (*CookOutput, error) {
	// 1. Find formula file path (resolve symlinks for consistent output).
	sourcePath, _, err := FindFormulaFile(name, path)
	if err != nil {
		return nil, fmt.Errorf("finding formula: %w", err)
	}
	if resolved, evalErr := filepath.EvalSymlinks(sourcePath); evalErr == nil {
		sourcePath = resolved
	}

	// 2. Resolve formula (load + resolve inheritance).
	formula, err := ResolveFormula(name, path)
	if err != nil {
		return nil, fmt.Errorf("resolving formula: %w", err)
	}

	// 3. Validate variables.
	if err := ValidateVars(formula, vars); err != nil {
		return nil, fmt.Errorf("validating variables: %w", err)
	}

	// 4. Substitute variables in steps and description.
	formula = SubstituteVars(formula, vars)
	formula.Description = substituteString(formula.Description, buildVals(formula, vars))

	return &CookOutput{Formula: formula, Source: sourcePath}, nil
}

// buildVals merges formula var defaults with provided overrides.
func buildVals(formula *Formula, provided map[string]string) map[string]string {
	vals := make(map[string]string)
	for name, def := range formula.Vars {
		if def.Default != "" {
			vals[name] = def.Default
		}
	}
	for k, v := range provided {
		vals[k] = v
	}
	return vals
}
