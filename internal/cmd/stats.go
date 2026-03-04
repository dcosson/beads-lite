package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"
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
	var (
		createdAfter  string
		createdBefore string
		idsCSV        string
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show statistics",
		Long: `Display statistics about issues in the beads storage.

By default, stats are computed over all issues.

Use --created-after/--created-before to scope by creation time.
Use --ids to scope stats to specific issue IDs (comma-separated).

Examples:
  bd stats
  bd stats --created-after 2026-03-01 --created-before 2026-03-31
  bd stats --ids bd-abc,bd-def,bd-ghi`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			var summary StatsSummary

			selectedIssues, err := collectStatsIssues(ctx, app.Storage, idsCSV, createdAfter, createdBefore)
			if err != nil {
				return err
			}

			// Build global closed set for ready checks (ready can depend on issues outside selection).
			closedStatus := issuestorage.StatusClosed
			closedForReady, err := app.Storage.List(ctx, &issuestorage.ListFilter{Status: &closedStatus})
			if err != nil {
				return fmt.Errorf("listing closed issues for ready checks: %w", err)
			}

			closedSet := make(map[string]bool)
			for _, issue := range closedForReady {
				closedSet[issue.ID] = true
			}

			for _, issue := range selectedIssues {
				switch issue.Status {
				case issuestorage.StatusOpen:
					summary.OpenIssues++
					// Check if ready (no unclosed blocking deps, including inherited)
					cascade := cascadeEnabled(app)
					blocked, err := graph.IsEffectivelyBlocked(ctx, app.Storage, issue, closedSet, cascade)
					if err != nil {
						return fmt.Errorf("checking blocked status for %s: %w", issue.ID, err)
					}
					if !blocked {
						summary.ReadyIssues++
					}
				case issuestorage.StatusInProgress:
					summary.InProgressIssues++
				case issuestorage.StatusBlocked:
					summary.BlockedIssues++
				case issuestorage.StatusDeferred:
					summary.DeferredIssues++
				case issuestorage.StatusPinned:
					summary.PinnedIssues++
				case issuestorage.StatusClosed:
					summary.ClosedIssues++
				case issuestorage.StatusTombstone:
					summary.TombstoneIssues++
				}
			}

			summary.TotalIssues = len(selectedIssues)

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

	cmd.Flags().StringVar(&createdAfter, "created-after", "", "Filter stats by created_at >= this time (YYYY-MM-DD or RFC3339; timezone optional for local time)")
	cmd.Flags().StringVar(&createdBefore, "created-before", "", "Filter stats by created_at <= this time (YYYY-MM-DD or RFC3339; timezone optional for local time)")
	cmd.Flags().StringVar(&idsCSV, "ids", "", "Comma-separated issue IDs to include")

	return cmd
}

func collectStatsIssues(
	ctx context.Context,
	store issuestorage.IssueStore,
	idsCSV, createdAfter, createdBefore string,
) ([]*issuestorage.Issue, error) {
	var createdAfterTime *time.Time
	var createdBeforeTime *time.Time
	if createdAfter != "" {
		t, err := parseListCreatedTime(createdAfter, false)
		if err != nil {
			return nil, fmt.Errorf("invalid --created-after value %q: %w", createdAfter, err)
		}
		createdAfterTime = &t
	}
	if createdBefore != "" {
		t, err := parseListCreatedTime(createdBefore, true)
		if err != nil {
			return nil, fmt.Errorf("invalid --created-before value %q: %w", createdBefore, err)
		}
		createdBeforeTime = &t
	}
	if createdAfterTime != nil && createdBeforeTime != nil && createdAfterTime.After(*createdBeforeTime) {
		return nil, fmt.Errorf("--created-after must be earlier than or equal to --created-before")
	}

	if idsCSV != "" {
		allIssues, err := listAllIssuesForStats(ctx, store, &issuestorage.ListFilter{})
		if err != nil {
			return nil, err
		}
		issueByID := make(map[string]*issuestorage.Issue, len(allIssues))
		childrenByParent := make(map[string][]string)
		for _, issue := range allIssues {
			issueByID[issue.ID] = issue
			if issue.Parent != "" {
				childrenByParent[issue.Parent] = append(childrenByParent[issue.Parent], issue.ID)
			}
		}

		ids := parseCSVItems(idsCSV)
		if len(ids) == 0 {
			return nil, fmt.Errorf("--ids must include at least one issue ID")
		}

		// Expand requested IDs to include all descendants recursively.
		seen := make(map[string]bool, len(allIssues))
		stack := make([]string, 0, len(ids))
		for _, id := range ids {
			if _, ok := issueByID[id]; !ok {
				return nil, fmt.Errorf("loading issue %q for --ids: %w", id, issuestorage.ErrNotFound)
			}
			stack = append(stack, id)
		}
		for len(stack) > 0 {
			n := len(stack) - 1
			id := stack[n]
			stack = stack[:n]
			if seen[id] {
				continue
			}
			seen[id] = true
			stack = append(stack, childrenByParent[id]...)
		}

		issues := make([]*issuestorage.Issue, 0, len(ids))
		for id := range seen {
			issue := issueByID[id]
			if issue == nil {
				continue
			}
			if createdAfterTime != nil && issue.CreatedAt.Before(*createdAfterTime) {
				continue
			}
			if createdBeforeTime != nil && issue.CreatedAt.After(*createdBeforeTime) {
				continue
			}
			issues = append(issues, issue)
		}
		return issues, nil
	}

	filter := &issuestorage.ListFilter{
		CreatedAfter:  createdAfterTime,
		CreatedBefore: createdBeforeTime,
	}

	return listAllIssuesForStats(ctx, store, filter)
}

func listAllIssuesForStats(
	ctx context.Context,
	store issuestorage.IssueStore,
	filter *issuestorage.ListFilter,
) ([]*issuestorage.Issue, error) {
	openIssues, err := store.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing open issues: %w", err)
	}

	closedFilter := *filter
	closedStatus := issuestorage.StatusClosed
	closedFilter.Status = &closedStatus
	closedIssues, err := store.List(ctx, &closedFilter)
	if err != nil {
		return nil, fmt.Errorf("listing closed issues: %w", err)
	}

	tombFilter := *filter
	tombStatus := issuestorage.StatusTombstone
	tombFilter.Status = &tombStatus
	tombIssues, err := store.List(ctx, &tombFilter)
	if err != nil {
		return nil, fmt.Errorf("listing tombstoned issues: %w", err)
	}

	issues := make([]*issuestorage.Issue, 0, len(openIssues)+len(closedIssues)+len(tombIssues))
	issues = append(issues, openIssues...)
	issues = append(issues, closedIssues...)
	issues = append(issues, tombIssues...)
	return issues, nil
}

func parseCSVItems(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			items = append(items, p)
		}
	}
	return items
}
