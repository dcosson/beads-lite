package meow

import (
	"context"
	"fmt"
	"os"

	"beads-lite/internal/issuestorage"
)

// PourOptions configures a Pour or Wisp operation.
type PourOptions struct {
	FormulaName string
	Vars        map[string]string
	Ephemeral   bool // false = pour (persistent), true = wisp (ephemeral)
	SearchPath  FormulaSearchPath
}

// PourResult contains the issues created by a Pour or Wisp operation.
type PourResult struct {
	NewEpicID string            `json:"new_epic_id"`
	IDMapping map[string]string `json:"id_mapping"`
	Created   int               `json:"created"`
	Attached  int               `json:"attached"`
	Phase     string            `json:"phase"`
}

// Pour resolves a formula, validates and substitutes variables, then creates
// a root epic issue and child issues in the storage backend.
// When opts.Ephemeral is true the operation is a "wisp" — all created issues
// are marked ephemeral.
func Pour(ctx context.Context, store issuestorage.IssueStore, opts PourOptions) (*PourResult, error) {
	// 1. Resolve formula (load + resolve inheritance).
	formula, err := ResolveFormula(opts.FormulaName, opts.SearchPath)
	if err != nil {
		return nil, fmt.Errorf("resolving formula: %w", err)
	}

	// 2. Validate variables.
	if err := ValidateVars(formula, opts.Vars); err != nil {
		return nil, fmt.Errorf("validating variables: %w", err)
	}

	// 3. Substitute variables.
	formula = SubstituteVars(formula, opts.Vars)

	// 4. Warn if pouring a vapor-phase formula.
	if formula.Phase == "vapor" && !opts.Ephemeral {
		fmt.Fprintf(os.Stderr, "warning: formula %q has phase \"vapor\"; consider using wisp instead of pour\n", formula.Formula)
	}

	// 5. Resolve actor for created_by.
	actor := ResolveUser()

	// 6. Create root epic issue.
	rootIssue := &issuestorage.Issue{
		Type:        issuestorage.TypeEpic,
		Title:       formula.Formula,
		Description: formula.Description,
		Ephemeral:   opts.Ephemeral,
		CreatedBy:   actor,
	}
	rootID, err := store.Create(ctx, rootIssue)
	if err != nil {
		return nil, fmt.Errorf("creating root issue: %w", err)
	}

	// 7. Pass 1 — create all child issues and record step-ID → issue-ID mapping.
	stepToIssue := make(map[string]string, len(formula.Steps))
	childIDs := make([]string, 0, len(formula.Steps))

	for _, step := range formula.Steps {
		childID, err := store.GetNextChildID(ctx, rootID)
		if err != nil {
			return nil, fmt.Errorf("getting next child ID for step %q: %w", step.ID, err)
		}

		issueType := issuestorage.TypeTask
		if step.Type != "" {
			issueType = issuestorage.IssueType(step.Type)
		}

		child := &issuestorage.Issue{
			ID:          childID,
			Type:        issueType,
			Title:       step.Title,
			Description: step.Description,
			Priority:    issuestorage.Priority(step.Priority),
			Labels:      step.Labels,
			Assignee:    step.Assignee,
			Ephemeral:   opts.Ephemeral,
			CreatedBy:   actor,
		}
		if _, err := store.Create(ctx, child); err != nil {
			return nil, fmt.Errorf("creating child issue for step %q: %w", step.ID, err)
		}

		// Wire parent-child dependency.
		if err := store.AddDependency(ctx, childID, rootID, issuestorage.DepTypeParentChild); err != nil {
			return nil, fmt.Errorf("adding parent-child dep for step %q: %w", step.ID, err)
		}

		stepToIssue[step.ID] = childID
		childIDs = append(childIDs, childID)
	}

	// 8. Pass 2 — wire DependsOn as DepTypeBlocks dependencies.
	for _, step := range formula.Steps {
		if len(step.DependsOn) == 0 {
			continue
		}
		childIssueID := stepToIssue[step.ID]
		for _, depStepID := range step.DependsOn {
			depIssueID, ok := stepToIssue[depStepID]
			if !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", step.ID, depStepID)
			}
			if err := store.AddDependency(ctx, childIssueID, depIssueID, issuestorage.DepTypeBlocks); err != nil {
				return nil, fmt.Errorf("adding blocks dep from step %q to %q: %w", step.ID, depStepID, err)
			}
		}
	}

	// Build id_mapping: formula name → root ID, formula.step → child ID.
	idMapping := make(map[string]string, 1+len(stepToIssue))
	idMapping[opts.FormulaName] = rootID
	for _, step := range formula.Steps {
		if childID, ok := stepToIssue[step.ID]; ok {
			idMapping[opts.FormulaName+"."+step.ID] = childID
		}
	}

	phase := "liquid"
	if opts.Ephemeral {
		phase = "vapor"
	}

	return &PourResult{
		NewEpicID: rootID,
		IDMapping: idMapping,
		Created:   1 + len(childIDs),
		Attached:  0,
		Phase:     phase,
	}, nil
}
