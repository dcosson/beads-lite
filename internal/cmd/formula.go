package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"beads-lite/internal/meow"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

// newFormulaCmd creates the formula command group with subcommands.
func newFormulaCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formula",
		Short: "Manage MEOW formulas (molecule templates)",
		Long: `Manage MEOW formulas — the source templates for molecules.

Formulas are file-based templates (YAML/JSON/TOML) searched in priority order:
  1. <configDir>/formulas/     (project-level)
  2. ~/.beads/formulas/        (user-level)
  3. $GT_ROOT/.beads/formulas/ (orchestrator-level)

Subcommands:
  list      List available formulas from all search paths
  show      Show formula details, steps, and composition rules
  convert   Convert formula between JSON and TOML formats`,
	}

	cmd.AddCommand(newFormulaListCmd(provider))
	cmd.AddCommand(newFormulaShowCmd(provider))
	cmd.AddCommand(newFormulaConvertCmd(provider))

	return cmd
}

// newFormulaListCmd creates the "formula list" subcommand.
func newFormulaListCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available formulas from all search paths",
		Long: `List available formulas from all search paths.

Shows formula name, type, phase, description, and source path.
Higher-priority paths shadow lower-priority ones with the same name.

Examples:
  bd formula list
  bd formula list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			entries, err := meow.ListFormulas(app.FormulaPath)
			if err != nil {
				return fmt.Errorf("formula list: %w", err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Fprintln(app.Out, "No formulas found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Formulas (%d found)\n", len(entries))

			// Group by type.
			byType := make(map[meow.FormulaType][]meow.FormulaEntry)
			for _, e := range entries {
				byType[e.Type] = append(byType[e.Type], e)
			}

			// Print in fixed order: workflow, expansion, aspect.
			typeOrder := []meow.FormulaType{
				meow.FormulaTypeWorkflow,
				meow.FormulaTypeExpansion,
				meow.FormulaTypeAspect,
			}
			typeIcons := map[meow.FormulaType]string{
				meow.FormulaTypeWorkflow:  "Workflow",
				meow.FormulaTypeExpansion: "Expansion",
				meow.FormulaTypeAspect:    "Aspect",
			}
			for _, ft := range typeOrder {
				group := byType[ft]
				if len(group) == 0 {
					continue
				}
				sort.Slice(group, func(i, j int) bool {
					return group[i].Name < group[j].Name
				})
				fmt.Fprintf(app.Out, "\n%s:\n", typeIcons[ft])
				for _, e := range group {
					varInfo := ""
					if e.Vars > 0 {
						varInfo = fmt.Sprintf(" (%d vars)", e.Vars)
					}
					desc := truncateDesc(e.Description, 60)
					fmt.Fprintf(app.Out, "  %-25s %s%s\n", e.Name, desc, varInfo)
				}
			}
			return nil
		},
	}

	return cmd
}

// newFormulaShowCmd creates the "formula show" subcommand.
func newFormulaShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show formula details, steps, and composition rules",
		Long: `Display full formula details including resolved inheritance.

Shows description, type, phase, version, variables, steps with
dependencies, and composition rules.

Examples:
  bd formula show deploy
  bd formula show feature-workflow --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			name := args[0]

			// Load the raw formula first (for source info).
			raw, err := meow.LoadFormula(name, app.FormulaPath)
			if err != nil {
				return fmt.Errorf("formula show: %w", err)
			}

			// Resolve inheritance for the full picture.
			resolved, err := meow.ResolveFormula(name, app.FormulaPath)
			if err != nil {
				return fmt.Errorf("formula show: resolving %s: %w", name, err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(resolved)
			}

			// Header
			fmt.Fprintf(app.Out, "Formula: %s\n", resolved.Formula)
			fmt.Fprintf(app.Out, "  Description: %s\n", resolved.Description)
			fmt.Fprintf(app.Out, "  Type:        %s\n", resolved.Type)
			if resolved.Phase != "" {
				fmt.Fprintf(app.Out, "  Phase:       %s\n", resolved.Phase)
			}
			fmt.Fprintf(app.Out, "  Version:     %d\n", resolved.Version)

			// Inheritance chain
			if len(raw.Extends) > 0 {
				fmt.Fprintf(app.Out, "  Extends:     %s\n", strings.Join(raw.Extends, " → "))
			}

			// Variables
			if len(resolved.Vars) > 0 {
				fmt.Fprintf(app.Out, "\nVariables (%d):\n", len(resolved.Vars))
				// Sort variable names for stable output.
				varNames := make([]string, 0, len(resolved.Vars))
				for k := range resolved.Vars {
					varNames = append(varNames, k)
				}
				sort.Strings(varNames)
				for _, name := range varNames {
					v := resolved.Vars[name]
					req := ""
					if v.Required {
						req = " (required)"
					}
					fmt.Fprintf(app.Out, "  %-16s %s%s\n", name, v.Description, req)
					if v.Default != "" {
						fmt.Fprintf(app.Out, "    default: %s\n", v.Default)
					}
					if len(v.Enum) > 0 {
						fmt.Fprintf(app.Out, "    enum: %s\n", strings.Join(v.Enum, ", "))
					}
					if v.Pattern != "" {
						fmt.Fprintf(app.Out, "    pattern: %s\n", v.Pattern)
					}
				}
			}

			// Steps
			steps := resolved.Steps
			if len(steps) == 0 {
				steps = resolved.Template
			}
			if len(steps) > 0 {
				fmt.Fprintf(app.Out, "\nSteps (%d):\n", len(steps))
				for _, s := range steps {
					deps := ""
					if len(s.DependsOn) > 0 {
						deps = fmt.Sprintf(" → depends on: %s", strings.Join(s.DependsOn, ", "))
					}
					fmt.Fprintf(app.Out, "  [%s] %s: %s%s\n", stepType(s), s.ID, s.Title, deps)
				}
			}

			// Composition rules (stubs in v1, but display if present)
			if resolved.Compose != nil {
				fmt.Fprintln(app.Out, "\nCompose: (defined)")
			}
			if len(resolved.Advice) > 0 {
				fmt.Fprintf(app.Out, "\nAdvice rules: %d\n", len(resolved.Advice))
			}

			return nil
		},
	}

	return cmd
}

// truncateDesc truncates a description to maxLen characters, appending "..."
// if truncation occurs. Newlines are replaced with spaces for single-line display.
func truncateDesc(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}

// stepType returns the type for display, defaulting to "task".
func stepType(s *meow.Step) string {
	if s.Type != "" {
		return s.Type
	}
	return "task"
}

// newFormulaConvertCmd creates the "formula convert" subcommand.
func newFormulaConvertCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert <name>",
		Short: "Convert formula between JSON and TOML formats",
		Long: `Convert a formula file from JSON to TOML or vice versa.

The source format is detected from the existing file. The converted
file is written alongside the original with the new extension.

Examples:
  bd formula convert deploy        # JSON → TOML or TOML → JSON
  bd formula convert feature-flow  # auto-detects source format`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			name := args[0]

			srcPath, srcFormat, err := meow.FindFormulaFile(name, app.FormulaPath)
			if err != nil {
				return fmt.Errorf("formula convert: %w", err)
			}

			formula, err := meow.LoadFormula(name, app.FormulaPath)
			if err != nil {
				return fmt.Errorf("formula convert: %w", err)
			}

			var dstPath string
			var dstData []byte

			switch srcFormat {
			case "json":
				// Convert to TOML
				var buf bytes.Buffer
				enc := toml.NewEncoder(&buf)
				if err := enc.Encode(formula); err != nil {
					return fmt.Errorf("formula convert: encoding TOML: %w", err)
				}
				dstData = buf.Bytes()
				dstPath = strings.TrimSuffix(srcPath, ".formula.json") + ".formula.toml"
			case "toml":
				// Convert to JSON
				data, err := json.MarshalIndent(formula, "", "  ")
				if err != nil {
					return fmt.Errorf("formula convert: encoding JSON: %w", err)
				}
				dstData = append(data, '\n')
				dstPath = strings.TrimSuffix(srcPath, ".formula.toml") + ".formula.json"
			}

			if err := os.WriteFile(dstPath, dstData, 0o644); err != nil {
				return fmt.Errorf("formula convert: writing %s: %w", dstPath, err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(map[string]string{
					"source":      srcPath,
					"destination": dstPath,
					"from_format": srcFormat,
				})
			}

			fmt.Fprintf(app.Out, "Converted: %s → %s\n", srcPath, dstPath)
			return nil
		},
	}

	return cmd
}
