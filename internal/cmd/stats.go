package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/storage"
	"github.com/spf13/cobra"
)

// StatsSummary represents the summary stats matching original beads format.
type StatsSummary struct {
	AverageLeadTimeHours    float64 `json:"average_lead_time_hours"`
	BlockedIssues           int     `json:"blocked_issues"`
	ClosedIssues            int     `json:"closed_issues"`
	DeferredIssues          int     `json:"deferred_issues"`
	EpicsEligibleForClosure int     `json:"epics_eligible_for_closure"`
	InProgressIssues        int     `json:"in_progress_issues"`
	OpenIssues              int     `json:"open_issues"`
	PinnedIssues            int     `json:"pinned_issues"`
	ReadyIssues             int     `json:"ready_issues"`
	TombstoneIssues         int     `json:"tombstone_issues"`
	TotalIssues             int     `json:"total_issues"`
}

// StatsResult wraps the summary in a top-level object.
type StatsResult struct {
	Summary StatsSummary `json:"summary"`
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

			var summary StatsSummary

			// Get open issues (includes all non-closed statuses)
			openIssues, err := app.Storage.List(ctx, nil)
			if err != nil {
				return fmt.Errorf("listing open issues: %w", err)
			}

			// Build closed set for ready check
			closedStatus := storage.StatusClosed
			closedIssues, err := app.Storage.List(ctx, &storage.ListFilter{Status: &closedStatus})
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}
			closedSet := make(map[string]bool)
			for _, issue := range closedIssues {
				closedSet[issue.ID] = true
			}

			for _, issue := range openIssues {
				switch issue.Status {
				case storage.StatusOpen:
					summary.OpenIssues++
					// Check if ready (no unclosed blocking deps)
					if isReady(issue, closedSet) {
						summary.ReadyIssues++
					}
				case storage.StatusInProgress:
					summary.InProgressIssues++
				case storage.StatusBlocked:
					summary.BlockedIssues++
				case storage.StatusDeferred:
					summary.DeferredIssues++
				}
			}

			// Count tombstoned issues
			tombstoneStatus := storage.StatusTombstone
			tombstones, err := app.Storage.List(ctx, &storage.ListFilter{Status: &tombstoneStatus})
			if err != nil {
				return fmt.Errorf("listing tombstoned issues: %w", err)
			}
			summary.TombstoneIssues = len(tombstones)

			summary.ClosedIssues = len(closedIssues)
			summary.TotalIssues = len(openIssues) + summary.ClosedIssues + summary.TombstoneIssues

			// Calculate average lead time (simplified - would need closed_at tracking)
			// For now, just use a placeholder calculation
			if summary.ClosedIssues > 0 {
				// This would need actual closed_at - created_at calculation
				summary.AverageLeadTimeHours = 0.001 // Placeholder
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(StatsResult{Summary: summary})
			}

			// Human-readable output
			openTotal := summary.OpenIssues + summary.InProgressIssues + summary.BlockedIssues + summary.DeferredIssues
			fmt.Fprintf(app.Out, "Open issues:     %d\n", openTotal)
			if summary.InProgressIssues > 0 {
				fmt.Fprintf(app.Out, "  In progress:   %d\n", summary.InProgressIssues)
			}
			if summary.BlockedIssues > 0 {
				fmt.Fprintf(app.Out, "  Blocked:       %d\n", summary.BlockedIssues)
			}
			if summary.DeferredIssues > 0 {
				fmt.Fprintf(app.Out, "  Deferred:      %d\n", summary.DeferredIssues)
			}
			fmt.Fprintf(app.Out, "Closed issues:   %d\n", summary.ClosedIssues)
			fmt.Fprintf(app.Out, "Total:           %d\n", summary.TotalIssues)

			return nil
		},
	}

	return cmd
}
