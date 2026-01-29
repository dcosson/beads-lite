package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// BlockedIssueJSON represents a blocked issue with blocked_by info for JSON output.
type BlockedIssueJSON struct {
	BlockedBy      []string `json:"blocked_by"`
	BlockedByCount int      `json:"blocked_by_count"`
	CreatedAt      string   `json:"created_at"`
	CreatedBy      string   `json:"created_by,omitempty"`
	ID             string   `json:"id"`
	IssueType      string   `json:"issue_type"`
	Priority       int      `json:"priority"`
	Status         string   `json:"status"`
	Title          string   `json:"title"`
	UpdatedAt      string   `json:"updated_at"`
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
			openStatus := storage.StatusOpen
			filter := &storage.ListFilter{
				Status: &openStatus,
			}

			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing issues: %w", err)
			}

			// Get closed issues to check dependency status
			closedStatus := storage.StatusClosed
			closedFilter := &storage.ListFilter{
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

			// Find blocked issues and what they're waiting on
			var blocked []BlockedIssueJSON
			name, _ := getGitUser()
			for _, issue := range issues {
				waitingOn := getWaitingOn(issue, closedSet)
				if len(waitingOn) > 0 {
					blocked = append(blocked, BlockedIssueJSON{
						BlockedBy:      waitingOn,
						BlockedByCount: len(waitingOn),
						CreatedAt:      formatTime(issue.CreatedAt),
						CreatedBy:      name,
						ID:             issue.ID,
						IssueType:      string(issue.Type),
						Priority:       priorityToInt(issue.Priority),
						Status:         string(issue.Status),
						Title:          issue.Title,
						UpdatedAt:      formatTime(issue.UpdatedAt),
					})
				}
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
				fmt.Fprintf(app.Out, "    Waiting on: %v\n\n", bi.BlockedBy)
			}

			return nil
		},
	}

	return cmd
}

// getWaitingOn returns a list of issue IDs that this issue is waiting on.
// Only "blocks" type dependencies prevent readiness.
func getWaitingOn(issue *storage.Issue, closedSet map[string]bool) []string {
	var waitingOn []string
	seen := make(map[string]bool)

	for _, dep := range issue.Dependencies {
		if dep.Type == storage.DepTypeBlocks && !closedSet[dep.ID] && !seen[dep.ID] {
			waitingOn = append(waitingOn, dep.ID)
			seen[dep.ID] = true
		}
	}

	return waitingOn
}
