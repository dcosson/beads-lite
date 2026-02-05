// Package meow implements MEOW formula parsing and resolution.
package meow

import "fmt"

// FormulaType identifies the kind of formula.
type FormulaType string

const (
	FormulaTypeWorkflow  FormulaType = "workflow"
	FormulaTypeExpansion FormulaType = "expansion"
	FormulaTypeAspect    FormulaType = "aspect"
)

// Formula is the top-level MEOW formula definition.
type Formula struct {
	Formula     string             `json:"formula" toml:"formula"`
	Description string             `json:"description" toml:"description"`
	Version     int                `json:"version" toml:"version"`
	Type        FormulaType        `json:"type" toml:"type"`
	Extends     []string           `json:"extends,omitempty" toml:"extends"`
	Vars        map[string]*VarDef `json:"vars,omitempty" toml:"vars"`
	Steps       []*Step            `json:"steps,omitempty" toml:"steps"`
	Template    []*Step            `json:"template,omitempty" toml:"template"`
	Compose     *ComposeRules      `json:"compose,omitempty" toml:"compose"`
	Advice      []*AdviceRule      `json:"advice,omitempty" toml:"advice"`
	Phase       string             `json:"phase,omitempty" toml:"phase"`
}

// VarDef defines a variable that can be substituted into a formula.
// In TOML, a VarDef can be either a table ([vars.name] with fields) or a
// plain string (vars.name = "value"), where the string becomes the default.
type VarDef struct {
	Description string   `json:"description,omitempty" toml:"description"`
	Default     string   `json:"default,omitempty" toml:"default"`
	Required    bool     `json:"required,omitempty" toml:"required"`
	Enum        []string `json:"enum,omitempty" toml:"enum"`
	Pattern     string   `json:"pattern,omitempty" toml:"pattern"`
	Type        string   `json:"type,omitempty" toml:"type"`
}

// UnmarshalTOML implements toml.Unmarshaler so that a VarDef can be decoded
// from either a plain string (shorthand for default value) or a full table.
func (v *VarDef) UnmarshalTOML(data any) error {
	switch val := data.(type) {
	case string:
		v.Default = val
		return nil
	case map[string]any:
		if s, ok := val["description"].(string); ok {
			v.Description = s
		}
		if s, ok := val["default"].(string); ok {
			v.Default = s
		}
		if b, ok := val["required"].(bool); ok {
			v.Required = b
		}
		if arr, ok := val["enum"].([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					v.Enum = append(v.Enum, s)
				}
			}
		}
		if s, ok := val["pattern"].(string); ok {
			v.Pattern = s
		}
		if s, ok := val["type"].(string); ok {
			v.Type = s
		}
		return nil
	default:
		return fmt.Errorf("expected string or table for VarDef, got %T", data)
	}
}

// Step is a single work item within a formula.
type Step struct {
	ID          string   `json:"id" toml:"id"`
	Title       string   `json:"title" toml:"title"`
	Description string   `json:"description,omitempty" toml:"description"`
	Type        string   `json:"type,omitempty" toml:"type"`
	Priority    string   `json:"priority,omitempty" toml:"priority"`
	Labels      []string `json:"labels,omitempty" toml:"labels"`
	DependsOn   []string `json:"depends_on,omitempty" toml:"depends_on"`
	Needs       []string `json:"needs,omitempty" toml:"needs"`
	Assignee    string   `json:"assignee,omitempty" toml:"assignee"`
	Expand      string   `json:"expand,omitempty" toml:"expand"`
	Children    []*Step  `json:"children,omitempty" toml:"children"`
}

// ComposeRules defines bonding rules for formula composition.
// Stub: parsed without error but not processed in v1.
type ComposeRules struct{}

// AdviceRule defines a step transformation rule.
// Stub: parsed without error but not processed in v1.
type AdviceRule struct{}
