package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// BlockedIssueJSON represents a blocked issue with blocked_by info for JSON output.
type BlockedIssueJSON struct {
	BlockedBy         []string                   `json:"blocked_by"`
	BlockedByCount    int                        `json:"blocked_by_count"`
	InheritedBlockers []InheritedBlockerShowJSON `json:"inherited_blockers,omitempty"`
	CreatedAt         string                 `json:"created_at"`
	CreatedBy         string                 `json:"created_by,omitempty"`
	ID                string                 `json:"id"`
	IssueType         string                 `json:"issue_type"`
	Priority          int                    `json:"priority"`
	Status            string                 `json:"status"`
	Title             string                 `json:"title"`
	UpdatedAt         string                 `json:"updated_at"`
}

// newBlockedCmd creates the blocked command.
func newBlockedCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked",
		Short: "List blocked issues and what they're waiting on",
		Long: `List issues that are blocked and show what they're waiting on.

An issue is blocked if:
- It has dependencies (depends_on) that are not closed
- Or it has blockers (blocked_by) that are not closed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// List all open issues
			openStatus := issuestorage.StatusOpen
			filter := &issuestorage.ListFilter{
				Status: &openStatus,
			}

			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing issues: %w", err)
			}

			// Get closed issues to check dependency status
			closedStatus := issuestorage.StatusClosed
			closedFilter := &issuestorage.ListFilter{
				Status: &closedStatus,
			}
			closedIssues, err := app.Storage.List(ctx, closedFilter)
			if err != nil {
				return fmt.Errorf("listing closed issues: %w", err)
			}
			closedSet := make(map[string]bool)
			for _, issue := range closedIssues {
				closedSet[issue.ID] = true
			}

			// Read cascade config flag
			cascade := cascadeEnabled(app)

			// Find blocked issues and what they're waiting on
			blocked := []BlockedIssueJSON{} // Initialize as empty slice (marshals to [] not null)
			for _, issue := range issues {
				result, err := graph.EffectiveBlockers(ctx, app.Storage, issue, closedSet, cascade)
				if err != nil {
					return fmt.Errorf("checking blockers for %s: %w", issue.ID, err)
				}
				if !result.HasBlockers() {
					continue
				}

				var inheritedJSON []InheritedBlockerShowJSON
				for _, ib := range result.Inherited {
					inheritedJSON = append(inheritedJSON, InheritedBlockerShowJSON{
						AncestorID: ib.AncestorID,
						BlockerID:  ib.BlockerID,
					})
				}

				allIDs := result.AllBlockerIDs()
				blocked = append(blocked, BlockedIssueJSON{
					BlockedBy:         result.Direct,
					BlockedByCount:    len(allIDs),
					InheritedBlockers: inheritedJSON,
					CreatedAt:         formatTime(issue.CreatedAt),
					CreatedBy:         issue.CreatedBy,
					ID:                issue.ID,
					IssueType:         string(issue.Type),
					Priority:          priorityToInt(issue.Priority),
					Status:            string(issue.Status),
					Title:             issue.Title,
					UpdatedAt:         formatTime(issue.UpdatedAt),
				})
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(blocked)
			}

			if len(blocked) == 0 {
				fmt.Fprintln(app.Out, "No blocked issues found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Blocked issues (%d):\n\n", len(blocked))
			for _, bi := range blocked {
				fmt.Fprintf(app.Out, "  %s  %s\n", bi.ID, bi.Title)
				if len(bi.BlockedBy) > 0 {
					fmt.Fprintf(app.Out, "    Waiting on: %v\n", bi.BlockedBy)
				}
				for _, ib := range bi.InheritedBlockers {
					fmt.Fprintf(app.Out, "    Parent blocked: %s blocked by %s\n", ib.AncestorID, ib.BlockerID)
				}
				fmt.Fprintln(app.Out)
			}

			return nil
		},
	}

	return cmd
}

