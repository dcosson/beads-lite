package meow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// varPattern matches {{word}} placeholders for variable substitution.
var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// ResolveFormula loads a formula by name and resolves its inheritance chain,
// merges needs into depends_on, and returns the fully resolved formula.
func ResolveFormula(name string, path FormulaSearchPath) (*Formula, error) {
	f, err := LoadFormula(name, path)
	if err != nil {
		return nil, err
	}

	// Step 1: Resolve inheritance (Extends chain).
	visited := map[string]bool{f.Formula: true}
	if err := resolveInheritance(f, path, visited); err != nil {
		return nil, err
	}

	// Merge needs → depends_on on all steps.
	mergeNeeds(f)

	// Steps 2-6 are stubs in v1 (no-ops).
	// 2. Apply control flow operators
	// 3. Apply advice rules
	// 4. Apply inline step expansions
	// 5. Apply composition expansions
	// 6. Apply aspects

	return f, nil
}

// resolveInheritance processes the Extends list: loads each parent, merges
// fields, and detects cycles via the visited set.
func resolveInheritance(formula *Formula, path FormulaSearchPath, visited map[string]bool) error {
	if len(formula.Extends) == 0 {
		return nil
	}

	for _, parentName := range formula.Extends {
		if visited[parentName] {
			return fmt.Errorf("formula inheritance cycle detected: %q already visited", parentName)
		}
		visited[parentName] = true

		parent, err := LoadFormula(parentName, path)
		if err != nil {
			return fmt.Errorf("loading parent formula %q: %w", parentName, err)
		}

		// Recursively resolve the parent's own inheritance first.
		if err := resolveInheritance(parent, path, visited); err != nil {
			return err
		}

		// Merge parent into formula: parent fields first, child overrides.
		mergeFormula(formula, parent)
	}

	// Clear Extends after resolution.
	formula.Extends = nil
	return nil
}

// mergeFormula merges parent fields into child. Child fields take precedence.
// Steps are prepended (parent steps come before child steps).
// Vars are merged (child vars override parent vars with the same key).
func mergeFormula(child, parent *Formula) {
	// Description: keep child if non-empty.
	if child.Description == "" {
		child.Description = parent.Description
	}

	// Phase: keep child if non-empty.
	if child.Phase == "" {
		child.Phase = parent.Phase
	}

	// Vars: merge parent vars, child overrides.
	if parent.Vars != nil {
		if child.Vars == nil {
			child.Vars = make(map[string]*VarDef)
		}
		for k, v := range parent.Vars {
			if _, exists := child.Vars[k]; !exists {
				child.Vars[k] = v
			}
		}
	}

	// Steps: parent steps come before child steps.
	if len(parent.Steps) > 0 {
		child.Steps = append(parent.Steps, child.Steps...)
	}

	// Template: same as steps.
	if len(parent.Template) > 0 {
		child.Template = append(parent.Template, child.Template...)
	}
}

// mergeNeeds converts Needs into DependsOn on all steps, then clears Needs.
func mergeNeeds(f *Formula) {
	for _, s := range f.Steps {
		mergeStepNeeds(s)
	}
	for _, s := range f.Template {
		mergeStepNeeds(s)
	}
}

// mergeStepNeeds merges a single step's Needs into DependsOn, including children.
func mergeStepNeeds(s *Step) {
	if len(s.Needs) > 0 {
		s.DependsOn = append(s.DependsOn, s.Needs...)
		s.Needs = nil
	}
	for _, c := range s.Children {
		mergeStepNeeds(c)
	}
}

// ValidateVars checks that all required vars have values and that enum/pattern
// constraints are satisfied. Returns a descriptive error listing all violations.
func ValidateVars(formula *Formula, provided map[string]string) error {
	var missing []string
	var violations []string

	for name, def := range formula.Vars {
		value, ok := provided[name]
		if !ok {
			value = def.Default
		}

		// Check required vars have values.
		if def.Required && value == "" {
			missing = append(missing, name)
			continue
		}

		// Skip further validation if no value provided and not required.
		if value == "" {
			continue
		}

		// Check enum constraints.
		if len(def.Enum) > 0 {
			found := false
			for _, allowed := range def.Enum {
				if value == allowed {
					found = true
					break
				}
			}
			if !found {
				violations = append(violations, fmt.Sprintf(
					"variable %q value %q not in allowed values: %s",
					name, value, strings.Join(def.Enum, ", ")))
			}
		}

		// Check pattern constraints.
		if def.Pattern != "" {
			re, err := regexp.Compile(def.Pattern)
			if err != nil {
				violations = append(violations, fmt.Sprintf(
					"variable %q has invalid pattern %q: %v", name, def.Pattern, err))
			} else if !re.MatchString(value) {
				violations = append(violations, fmt.Sprintf(
					"variable %q value %q does not match pattern %q",
					name, value, def.Pattern))
			}
		}
	}

	if len(missing) == 0 && len(violations) == 0 {
		return nil
	}

	var parts []string
	if len(missing) > 0 {
		sort.Strings(missing)
		parts = append(parts, fmt.Sprintf(
			"missing required variables: %s\nProvide them with: %s",
			strings.Join(missing, ", "),
			formatVarHints(missing)))
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		parts = append(parts, strings.Join(violations, "\n"))
	}

	return fmt.Errorf("%s", strings.Join(parts, "\n"))
}

func formatVarHints(names []string) string {
	hints := make([]string, len(names))
	for i, n := range names {
		hints[i] = "--var " + n + "=<value>"
	}
	return strings.Join(hints, " ")
}

// SubstituteVars applies variable defaults and replaces {{name}} patterns in
// all step text fields. Returns a new formula without mutating the input.
// Unknown placeholders are left as-is.
func SubstituteVars(formula *Formula, provided map[string]string) *Formula {
	// Deep-copy via JSON round-trip.
	out := deepCopyFormula(formula)

	// Build the final values map: defaults first, then provided overrides.
	vals := make(map[string]string)
	for name, def := range out.Vars {
		if def.Default != "" {
			vals[name] = def.Default
		}
	}
	for k, v := range provided {
		vals[k] = v
	}

	// Replace in all steps.
	for _, s := range out.Steps {
		substituteStep(s, vals)
	}
	for _, s := range out.Template {
		substituteStep(s, vals)
	}

	return out
}

func substituteStep(s *Step, vals map[string]string) {
	s.Title = substituteString(s.Title, vals)
	s.Description = substituteString(s.Description, vals)
	s.Assignee = substituteString(s.Assignee, vals)
	for _, c := range s.Children {
		substituteStep(c, vals)
	}
}

// substituteString replaces {{name}} with the value from vals.
// Unknown names are left as-is.
func substituteString(s string, vals map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-2] // strip {{ and }}
		if v, ok := vals[name]; ok {
			return v
		}
		return match
	})
}

// deepCopyFormula returns a deep copy of the formula via JSON round-trip.
func deepCopyFormula(f *Formula) *Formula {
	data, err := json.Marshal(f)
	if err != nil {
		// Shouldn't happen for a valid formula struct.
		panic(fmt.Sprintf("meow: failed to marshal formula for deep copy: %v", err))
	}
	out := &Formula{}
	if err := json.Unmarshal(data, out); err != nil {
		panic(fmt.Sprintf("meow: failed to unmarshal formula for deep copy: %v", err))
	}
	return out
}
