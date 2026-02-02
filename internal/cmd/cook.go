package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/meow"

	"github.com/spf13/cobra"
)

// newCookCmd creates the "cook" command — dry-run formula preview.
func newCookCmd(provider *AppProvider) *cobra.Command {
	var vars []string

	cmd := &cobra.Command{
		Use:   "cook <formula-name>",
		Short: "Preview what a formula would create (dry-run)",
		Long: `Preview the issues that would be created by pouring a formula,
without actually creating anything.

This is a dry-run that resolves the formula, substitutes variables,
and shows the resulting issue tree. No issues are created.

Variables can be supplied with --var key=value (repeatable).
Missing required variables cause an error.

Examples:
  bd cook deploy
  bd cook feature-workflow --var component=auth
  bd cook deploy --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			result, err := meow.Cook(args[0], parseVarFlags(vars), meow.DefaultSearchPath(app.ConfigDir))
			if err != nil {
				return fmt.Errorf("cook %s: %w", args[0], err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output: formula info then each step.
			desc := result.Description
			if desc == "" {
				desc = result.Formula.Formula
			}
			fmt.Fprintf(app.Out, "Formula: %s — %s\n", result.Formula.Formula, desc)
			if len(result.Steps) == 0 {
				fmt.Fprintln(app.Out, "  (no steps)")
				return nil
			}
			fmt.Fprintf(app.Out, "\nSteps (%d):\n", len(result.Steps))
			for _, step := range result.Steps {
				deps := ""
				if len(step.DependsOn) > 0 {
					deps = fmt.Sprintf(" → depends on: %s", strings.Join(step.DependsOn, ", "))
				}
				stepType := step.Type
				if stepType == "" {
					stepType = "task"
				}
				fmt.Fprintf(app.Out, "  [%s] %s: %s%s\n", stepType, step.ID, step.Title, deps)
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&vars, "var", nil, "Variable assignment (key=value, repeatable)")

	return cmd
}
