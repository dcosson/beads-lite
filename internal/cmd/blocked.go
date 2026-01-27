package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"beads2/internal/storage"

	"github.com/spf13/cobra"
)

// BlockedIssue represents an issue and what it's blocked by.
type BlockedIssue struct {
	Issue     *storage.Issue `json:"issue"`
	WaitingOn []string       `json:"waiting_on"`
}

// NewBlockedCmd creates the blocked command.
func NewBlockedCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked",
		Short: "List blocked issues and what they're waiting on",
		Long: `List issues that are blocked and show what they're waiting on.

An issue is blocked if:
- It has dependencies (depends_on) that are not closed
- Or it has blockers (blocked_by) that are not closed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlocked(cmd.Context(), app)
		},
	}

	return cmd
}

func runBlocked(ctx context.Context, app *App) error {
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
	var blocked []BlockedIssue
	for _, issue := range issues {
		waitingOn := getWaitingOn(issue, closedSet)
		if len(waitingOn) > 0 {
			blocked = append(blocked, BlockedIssue{
				Issue:     issue,
				WaitingOn: waitingOn,
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
		fmt.Fprintf(app.Out, "  %s  %s\n", bi.Issue.ID, bi.Issue.Title)
		fmt.Fprintf(app.Out, "    Waiting on: %v\n\n", bi.WaitingOn)
	}

	return nil
}

// getWaitingOn returns a list of issue IDs that this issue is waiting on.
// This includes both depends_on and blocked_by that are not closed.
func getWaitingOn(issue *storage.Issue, closedSet map[string]bool) []string {
	var waitingOn []string
	seen := make(map[string]bool)

	// Check depends_on
	for _, dep := range issue.DependsOn {
		if !closedSet[dep] && !seen[dep] {
			waitingOn = append(waitingOn, dep)
			seen[dep] = true
		}
	}

	// Check blocked_by
	for _, blocker := range issue.BlockedBy {
		if !closedSet[blocker] && !seen[blocker] {
			waitingOn = append(waitingOn, blocker)
			seen[blocker] = true
		}
	}

	return waitingOn
}
