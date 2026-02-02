package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// GateListJSON is the JSON output format for gate list command.
type GateListJSON struct {
	AwaitID   string   `json:"await_id,omitempty"`
	AwaitType string   `json:"await_type,omitempty"`
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	TimeoutNS int64    `json:"timeout_ns,omitempty"`
	Title     string   `json:"title"`
	Waiters   []string `json:"waiters,omitempty"`
}

// newGateCmd creates the gate parent command with subcommands.
func newGateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Manage gate issues",
		Long:  `Commands for managing gate issues (async coordination primitives).`,
	}

	cmd.AddCommand(newGateListCmd(provider))

	return cmd
}

// newGateListCmd creates the gate list subcommand.
func newGateListCmd(provider *AppProvider) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List gate issues",
		Long: `List gate issues (type=gate).

By default, lists only open gates. Use --all to include closed gates.

Examples:
  bd gate list          # List open gates
  bd gate list --all    # List all gates (open and closed)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			gateType := issuestorage.TypeGate
			filter := &issuestorage.ListFilter{
				Type: &gateType,
			}

			if all {
				// No status filter - list all gates
				filter.Status = nil
			} else {
				// Default: open gates only
				s := issuestorage.StatusOpen
				filter.Status = &s
			}

			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing gates: %w", err)
			}

			// If --all, also get closed gates
			if all && filter.Status == nil {
				closedFilter := *filter
				closedStatus := issuestorage.StatusClosed
				closedFilter.Status = &closedStatus
				closedIssues, err := app.Storage.List(ctx, &closedFilter)
				if err != nil {
					return fmt.Errorf("listing closed gates: %w", err)
				}
				issues = append(issues, closedIssues...)
			}

			// JSON output
			if app.JSON {
				result := make([]GateListJSON, len(issues))
				for i, issue := range issues {
					result[i] = GateListJSON{
						AwaitID:   issue.AwaitID,
						AwaitType: issue.AwaitType,
						ID:        issue.ID,
						Status:    string(issue.Status),
						TimeoutNS: issue.TimeoutNS,
						Title:     issue.Title,
						Waiters:   issue.Waiters,
					}
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output
			if len(issues) == 0 {
				fmt.Fprintln(app.Out, "No gates found.")
				return nil
			}

			// Table header
			fmt.Fprintf(app.Out, "%-12s %-20s %-10s %-12s %-8s %s\n",
				"ID", "Title", "Await", "Target", "Status", "Waiters")

			for _, issue := range issues {
				title := issue.Title
				if len(title) > 20 {
					title = title[:17] + "..."
				}

				target := issue.AwaitID
				if target == "" {
					target = "-"
				}
				if len(target) > 12 {
					target = target[:9] + "..."
				}

				awaitType := issue.AwaitType
				if awaitType == "" {
					awaitType = "-"
				}

				fmt.Fprintf(app.Out, "%-12s %-20s %-10s %-12s %-8s %d\n",
					issue.ID, title, awaitType, target,
					string(issue.Status), len(issue.Waiters))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Include closed gates")

	return cmd
}
