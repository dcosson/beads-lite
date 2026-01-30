package meow

import "fmt"

// CookResult is the dry-run preview of what pour/wisp would create.
type CookResult struct {
	Root  CookIssue   `json:"root"`
	Steps []CookIssue `json:"steps"`
}

// CookIssue describes a single issue that would be created.
type CookIssue struct {
	StepID      string   `json:"step_id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"` // Step IDs (not issue IDs)
	Assignee    string   `json:"assignee,omitempty"`
	Ephemeral   bool     `json:"ephemeral,omitempty"`
}

// Cook resolves a formula and builds a preview of the issues that pour/wisp
// would create, without touching storage. This is purely in-memory.
func Cook(name string, vars map[string]string, path FormulaSearchPath) (*CookResult, error) {
	// 1. Resolve formula (load + resolve inheritance).
	formula, err := ResolveFormula(name, path)
	if err != nil {
		return nil, fmt.Errorf("resolving formula: %w", err)
	}

	// 2. Validate variables.
	if err := ValidateVars(formula, vars); err != nil {
		return nil, fmt.Errorf("validating variables: %w", err)
	}

	// 3. Substitute variables.
	formula = SubstituteVars(formula, vars)

	// 4. Build root preview.
	// SubstituteVars only handles step fields, so substitute the description
	// separately for the root title.
	rootTitle := substituteString(formula.Description, buildVals(formula, vars))
	if rootTitle == "" {
		rootTitle = formula.Formula
	}
	root := CookIssue{
		StepID: "_root",
		Title:  rootTitle,
		Type:   "epic",
	}

	// 5. Build step previews.
	steps := make([]CookIssue, 0, len(formula.Steps))
	for _, step := range formula.Steps {
		issueType := "task"
		if step.Type != "" {
			issueType = step.Type
		}

		ci := CookIssue{
			StepID:      step.ID,
			Title:       step.Title,
			Description: step.Description,
			Type:        issueType,
			Priority:    step.Priority,
			Labels:      step.Labels,
			DependsOn:   step.DependsOn,
			Assignee:    step.Assignee,
		}
		steps = append(steps, ci)
	}

	return &CookResult{
		Root:  root,
		Steps: steps,
	}, nil
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
