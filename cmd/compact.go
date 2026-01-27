package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"beads2/storage"

	"github.com/spf13/cobra"
)

// NewCompactCmd creates the compact command.
func NewCompactCmd(app *App) *cobra.Command {
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

Examples:
  bd compact --before 2024-01-01          # delete closed before date
  bd compact --older-than 90d             # delete closed older than 90 days
  bd compact --dry-run                    # show what would be deleted`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate flags
			if before == "" && olderThan == "" {
				return fmt.Errorf("must specify --before or --older-than")
			}
			if before != "" && olderThan != "" {
				return fmt.Errorf("cannot specify both --before and --older-than")
			}

			// Parse cutoff time
			var cutoff time.Time
			if before != "" {
				t, err := time.Parse("2006-01-02", before)
				if err != nil {
					return fmt.Errorf("invalid date format for --before (use YYYY-MM-DD): %w", err)
				}
				cutoff = t
			} else {
				duration, err := parseDuration(olderThan)
				if err != nil {
					return fmt.Errorf("invalid duration format for --older-than: %w", err)
				}
				cutoff = time.Now().Add(-duration)
			}

			// List all closed issues
			status := storage.StatusClosed
			filter := &storage.ListFilter{Status: &status}
			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}

			// Find issues to delete
			var toDelete []*storage.Issue
			for _, issue := range issues {
				if issue.ClosedAt != nil && issue.ClosedAt.Before(cutoff) {
					toDelete = append(toDelete, issue)
				}
			}

			if len(toDelete) == 0 {
				if app.JSON {
					return json.NewEncoder(app.Out).Encode(map[string]interface{}{
						"deleted": []string{},
						"count":   0,
					})
				}
				fmt.Fprintln(app.Out, "No issues to delete")
				return nil
			}

			// Show what would be deleted
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
				fmt.Fprintf(app.Out, "Would delete %d issue(s):\n", len(toDelete))
				for _, issue := range toDelete {
					closedAt := ""
					if issue.ClosedAt != nil {
						closedAt = issue.ClosedAt.Format("2006-01-02")
					}
					fmt.Fprintf(app.Out, "  %s  %s  (closed %s)\n", issue.ID, issue.Title, closedAt)
				}
				return nil
			}

			// Confirm deletion unless --force
			if !force {
				fmt.Fprintf(app.Err, "About to delete %d closed issue(s). Continue? [y/N] ", len(toDelete))
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading confirmation: %w", err)
				}
				response = strings.TrimSpace(strings.ToLower(response))
				if response != "y" && response != "yes" {
					fmt.Fprintln(app.Out, "Aborted")
					return nil
				}
			}

			// Delete issues
			deleted := make([]string, 0, len(toDelete))
			for _, issue := range toDelete {
				if err := app.Storage.Delete(ctx, issue.ID); err != nil {
					fmt.Fprintf(app.Err, "Warning: failed to delete %s: %v\n", issue.ID, err)
					continue
				}
				deleted = append(deleted, issue.ID)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(map[string]interface{}{
					"deleted": deleted,
					"count":   len(deleted),
				})
			}

			fmt.Fprintf(app.Out, "Deleted %d issue(s)\n", len(deleted))
			return nil
		},
	}

	cmd.Flags().StringVar(&before, "before", "", "Delete issues closed before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Delete issues closed more than this duration ago (e.g., 90d, 6m, 1y)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// parseDuration parses a duration string that supports days (d), months (m), and years (y).
// Examples: "90d", "6m", "1y", "2w"
func parseDuration(s string) (time.Duration, error) {
	// Pattern: number followed by unit
	re := regexp.MustCompile(`^(\d+)([dwmy])$`)
	matches := re.FindStringSubmatch(strings.ToLower(s))
	if matches == nil {
		// Try standard Go duration parsing as fallback
		return time.ParseDuration(s)
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(value) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}
