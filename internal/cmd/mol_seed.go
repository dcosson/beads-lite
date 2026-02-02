package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/meow"

	"github.com/spf13/cobra"
)

// patrolFormulas are the three patrol formulas checked by --patrol.
var patrolFormulas = []string{
	"mol-deacon-patrol",
	"mol-witness-patrol",
	"mol-refinery-patrol",
}

// SeedResult holds the preflight check result for a single formula.
type SeedResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Source string `json:"source,omitempty"`
	Error  string `json:"error,omitempty"`
}

// newMolSeedCmd creates the "mol seed" subcommand — preflight formula health check.
func newMolSeedCmd(provider *AppProvider) *cobra.Command {
	var patrol bool
	var vars []string

	cmd := &cobra.Command{
		Use:   "seed [formula-name]",
		Short: "Preflight check: verify formula files are accessible and valid",
		Long: `Verify that formula files can be found and successfully parsed/cooked.
No issues are created, no state is changed — this is a pure health check.

With --patrol: checks all three patrol formulas (mol-deacon-patrol,
mol-witness-patrol, mol-refinery-patrol).

Without --patrol: checks a single named formula.

Searches: .beads/formulas/, ~/.beads/formulas/, $GT_ROOT/.beads/formulas/

Examples:
  bd mol seed my-workflow
  bd mol seed my-workflow --var component=auth
  bd mol seed --patrol
  bd mol seed --patrol --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			searchPath := meow.DefaultSearchPath(app.ConfigDir)
			parsedVars := parseVarFlags(vars)

			var names []string
			if patrol {
				names = patrolFormulas
			} else {
				if len(args) == 0 {
					return fmt.Errorf("requires a formula name argument (or use --patrol)")
				}
				names = []string{args[0]}
			}

			results := make([]SeedResult, 0, len(names))
			var failures int
			for _, name := range names {
				cooked, cookErr := meow.Cook(name, parsedVars, searchPath)
				if cookErr != nil {
					results = append(results, SeedResult{
						Name:  name,
						OK:    false,
						Error: cookErr.Error(),
					})
					failures++
				} else {
					results = append(results, SeedResult{
						Name:   name,
						OK:     true,
						Source: cooked.Source,
					})
				}
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(results)
			}

			// Text output.
			for _, r := range results {
				if r.OK {
					fmt.Fprintf(app.Out, "✓ %s (%s)\n", r.Name, r.Source)
				} else {
					fmt.Fprintf(app.Out, "✗ %s: %s\n", r.Name, r.Error)
				}
			}

			if failures > 0 {
				return fmt.Errorf("%d of %d formulas failed preflight check", failures, len(names))
			}

			if patrol {
				fmt.Fprintln(app.Out, "✓ All patrol formulas accessible")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&patrol, "patrol", false, "Check all three patrol formulas")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "Variable assignment (key=value, repeatable)")

	return cmd
}
