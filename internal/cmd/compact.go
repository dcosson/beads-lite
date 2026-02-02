package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newCompactCmd creates the compact command.
func newCompactCmd(provider *AppProvider) *cobra.Command {
	var (
		before    string
		olderThan string
		dryRun    bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "compact",
		Short: "Remove old closed issues to reduce repository size",
		Long: `Remove old closed issues to reduce repository size.

By default, prompts for confirmation before deleting. Use --force to skip.
Use --dry-run to preview what would be deleted without making changes.

Filter which closed issues to remove:
  --before DATE      Delete issues closed before DATE (YYYY-MM-DD)
  --older-than DUR   Delete issues closed more than DUR ago (e.g., 30d, 1w, 6m)

If no filter is specified, all closed issues will be targeted.

Examples:
  bd compact --dry-run                  # Preview all closed issues that would be removed
  bd compact --older-than 30d           # Remove issues closed more than 30 days ago
  bd compact --before 2024-01-01        # Remove issues closed before Jan 1, 2024
  bd compact --older-than 6m --force    # Remove issues older than 6 months without confirmation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Parse time filters
			var cutoff time.Time
			if before != "" && olderThan != "" {
				return fmt.Errorf("cannot specify both --before and --older-than")
			}

			if before != "" {
				t, err := time.Parse("2006-01-02", before)
				if err != nil {
					return fmt.Errorf("invalid --before date %q: expected YYYY-MM-DD format", before)
				}
				cutoff = t
			} else if olderThan != "" {
				dur, err := parseDuration(olderThan)
				if err != nil {
					return fmt.Errorf("invalid --older-than duration %q: %w", olderThan, err)
				}
				cutoff = time.Now().Add(-dur)
			}

			// Get all closed issues
			closedStatus := issuestorage.StatusClosed
			issues, err := app.Storage.List(ctx, &issuestorage.ListFilter{Status: &closedStatus})
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}

			// Filter by cutoff time if specified
			var toDelete []*issuestorage.Issue
			for _, issue := range issues {
				if issue.ClosedAt == nil {
					// Closed issues should have ClosedAt set, but handle edge case
					continue
				}
				if !cutoff.IsZero() && !issue.ClosedAt.Before(cutoff) {
					continue
				}
				toDelete = append(toDelete, issue)
			}

			if len(toDelete) == 0 {
				if app.JSON {
					return json.NewEncoder(app.Out).Encode(map[string]interface{}{
						"deleted": []string{},
						"count":   0,
					})
				}
				fmt.Fprintln(app.Out, "No closed issues match the criteria.")
				return nil
			}

			// Dry run: just show what would be deleted
			if dryRun {
				if app.JSON {
					ids := make([]string, len(toDelete))
					for i, issue := range toDelete {
						ids[i] = issue.ID
					}
					return json.NewEncoder(app.Out).Encode(map[string]interface{}{
						"would_delete": ids,
						"count":        len(toDelete),
					})
				}

				fmt.Fprintf(app.Out, "Would delete %d closed issue(s):\n\n", len(toDelete))
				for _, issue := range toDelete {
					closedAt := "unknown"
					if issue.ClosedAt != nil {
						closedAt = issue.ClosedAt.Format("2006-01-02")
					}
					fmt.Fprintf(app.Out, "  %s  %s  (closed: %s)\n", issue.ID, issue.Title, closedAt)
				}
				return nil
			}

			// Confirmation prompt unless --force is used
			if !force {
				fmt.Fprintf(app.Out, "This will permanently delete %d closed issue(s).\n", len(toDelete))
				fmt.Fprint(app.Out, "Continue? [y/N] ")

				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading confirmation: %w", err)
				}

				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					fmt.Fprintln(app.Out, "Cancelled")
					return nil
				}
			}

			// Delete the issues
			var deleted []string
			var errors []error
			for _, issue := range toDelete {
				if err := app.Storage.Delete(ctx, issue.ID); err != nil {
					errors = append(errors, fmt.Errorf("deleting %s: %w", issue.ID, err))
				} else {
					deleted = append(deleted, issue.ID)
				}
			}

			// Output results
			if app.JSON {
				result := map[string]interface{}{
					"deleted": deleted,
					"count":   len(deleted),
				}
				if len(errors) > 0 {
					errStrings := make([]string, len(errors))
					for i, e := range errors {
						errStrings[i] = e.Error()
					}
					result["errors"] = errStrings
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output
			fmt.Fprintf(app.Out, "Deleted %d issue(s)\n", len(deleted))

			// Report errors if any
			if len(errors) > 0 {
				for _, e := range errors {
					fmt.Fprintf(app.Err, "Error: %v\n", e)
				}
				return errors[0]
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&before, "before", "", "Delete issues closed before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Delete issues closed more than this duration ago (e.g., 30d, 1w, 6m)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be deleted without making changes")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// parseDuration parses a human-friendly duration string.
// Supports: d (days), w (weeks), m (months), y (years)
// Examples: "30d", "2w", "6m", "1y"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short: %s", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid number: %s", numStr)
	}

	if num <= 0 {
		return 0, fmt.Errorf("duration must be positive: %d", num)
	}

	switch unit {
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	case 'm':
		// Approximate months as 30 days
		return time.Duration(num) * 30 * 24 * time.Hour, nil
	case 'y':
		// Approximate years as 365 days
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit: %c (use d, w, m, or y)", unit)
	}
}
