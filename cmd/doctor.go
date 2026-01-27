package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// DoctorResult represents the output of the doctor command.
type DoctorResult struct {
	Problems []string `json:"problems"`
	Fixed    bool     `json:"fixed"`
}

// NewDoctorCmd creates the doctor command.
func NewDoctorCmd(app *App) *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check for and fix inconsistencies",
		Long: `Check for and fix inconsistencies in the beads storage.

Checks for:
- Status field doesn't match directory (open/ vs closed/)
- Broken dependency references (depends_on/blocks point to non-existent issues)
- Broken parent/child references
- Orphaned lock files
- Malformed JSON files
- Asymmetric relationships (A depends on B but B doesn't list A as dependent)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			problems, err := app.Storage.Doctor(ctx, fix)
			if err != nil {
				return fmt.Errorf("doctor failed: %w", err)
			}

			if app.JSON {
				result := DoctorResult{
					Problems: problems,
					Fixed:    fix,
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			if len(problems) == 0 {
				fmt.Fprintln(app.Out, "No problems found.")
				return nil
			}

			if fix {
				fmt.Fprintf(app.Out, "Fixed %d problems:\n", len(problems))
			} else {
				fmt.Fprintf(app.Out, "Found %d problems:\n", len(problems))
			}

			for _, problem := range problems {
				fmt.Fprintf(app.Out, "  - %s\n", problem)
			}

			if !fix && len(problems) > 0 {
				fmt.Fprintln(app.Out, "\nRun 'bd doctor --fix' to fix these issues.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Fix problems (default is check only)")

	return cmd
}
