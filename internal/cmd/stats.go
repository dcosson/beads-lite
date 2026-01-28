package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/storage"
	"github.com/spf13/cobra"
)

// StatsResult represents the output of the stats command.
type StatsResult struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Deferred   int `json:"deferred"`
	Closed     int `json:"closed"`
	Total      int `json:"total"`
}

// newStatsCmd creates the stats command.
func newStatsCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show statistics",
		Long:  `Display statistics about issues in the beads storage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			var result StatsResult

			// Get open issues (includes all non-closed statuses)
			openIssues, err := app.Storage.List(ctx, nil)
			if err != nil {
				return fmt.Errorf("listing open issues: %w", err)
			}

			for _, issue := range openIssues {
				switch issue.Status {
				case storage.StatusOpen:
					result.Open++
				case storage.StatusInProgress:
					result.InProgress++
				case storage.StatusBlocked:
					result.Blocked++
				case storage.StatusDeferred:
					result.Deferred++
				}
			}

			// Get closed issues
			closedStatus := storage.StatusClosed
			closedIssues, err := app.Storage.List(ctx, &storage.ListFilter{Status: &closedStatus})
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}
			result.Closed = len(closedIssues)

			result.Total = len(openIssues) + result.Closed

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Human-readable output
			openTotal := result.Open + result.InProgress + result.Blocked + result.Deferred
			fmt.Fprintf(app.Out, "Open issues:     %d\n", openTotal)
			if result.InProgress > 0 {
				fmt.Fprintf(app.Out, "  In progress:   %d\n", result.InProgress)
			}
			if result.Blocked > 0 {
				fmt.Fprintf(app.Out, "  Blocked:       %d\n", result.Blocked)
			}
			if result.Deferred > 0 {
				fmt.Fprintf(app.Out, "  Deferred:      %d\n", result.Deferred)
			}
			fmt.Fprintf(app.Out, "Closed issues:   %d\n", result.Closed)
			fmt.Fprintf(app.Out, "Total:           %d\n", result.Total)

			return nil
		},
	}

	return cmd
}
