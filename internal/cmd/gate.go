package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newGateCmd creates the gate command group.
func newGateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Gate commands for async coordination primitives",
	}

	cmd.AddCommand(newGateShowCmd(provider))
	cmd.AddCommand(newGateWaitCmd(provider))
	cmd.AddCommand(newGateAddWaiterCmd(provider))

	return cmd
}

// newGateShowCmd creates the gate show subcommand.
func newGateShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <gate-id>",
		Short: "Show details of a gate",
		Long: `Display detailed information about a gate issue.

The issue must have type "gate". Supports prefix matching on gate IDs.

Examples:
  bd gate show bl-abc123     # Exact ID match
  bd gate show bl-abc        # Prefix match (if unique)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := args[0]

			// Route to correct storage
			store, err := app.StorageFor(ctx, query)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", query, err)
			}

			// Try exact match first
			issue, err := store.Get(ctx, query)
			if err == nil && issue.Status == issuestorage.StatusTombstone {
				err = issuestorage.ErrNotFound
			}
			if err == issuestorage.ErrNotFound {
				issue, err = findByPrefix(store, ctx, query)
			}
			if err != nil {
				if err == issuestorage.ErrNotFound {
					return fmt.Errorf("no issue found matching %q", query)
				}
				return err
			}

			// Verify it's a gate
			if issue.Type != issuestorage.TypeGate {
				return fmt.Errorf("issue %s is type %q, not \"gate\"", issue.ID, issue.Type)
			}

			// JSON output uses existing ToIssueJSON path
			if app.JSON {
				return outputIssueJSON(app, ctx, issue)
			}

			// Text output matching the spec format
			fmt.Fprintf(app.Out, "Gate: %s\n", issue.ID)
			fmt.Fprintf(app.Out, "Title: %s\n", issue.Title)
			fmt.Fprintf(app.Out, "Status: %s\n", issue.Status)

			if issue.AwaitType != "" || issue.AwaitID != "" {
				fmt.Fprintf(app.Out, "Await: %s %s\n", issue.AwaitType, issue.AwaitID)
			}

			if issue.TimeoutNS != 0 {
				d := time.Duration(issue.TimeoutNS)
				fmt.Fprintf(app.Out, "Timeout: %s\n", d.String())
			}

			if len(issue.Waiters) > 0 {
				fmt.Fprintf(app.Out, "Waiters: %s\n", strings.Join(issue.Waiters, ", "))
			}

			fmt.Fprintf(app.Out, "Created: %s\n", issue.CreatedAt.Format(time.RFC3339))

			return nil
		},
	}

	return cmd
}

// newGateWaitCmd creates the "gate wait" command.
// Usage: bd gate wait <gate-id> --notify <agent-id>
func newGateWaitCmd(provider *AppProvider) *cobra.Command {
	var notify string

	cmd := &cobra.Command{
		Use:   "wait <gate-id>",
		Short: "Register a waiter on a gate",
		Long: `Register an address to be notified when a gate clears.

This is fire-and-forget registration — it appends the address to the gate's
waiters list (with deduplication) and returns immediately.

Examples:
  bd gate wait bl-g1 --notify beads_lite/polecats/onyx
  bd gate wait bl-g1 --notify beads_lite/crew/planning`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if notify == "" {
				return fmt.Errorf("--notify is required")
			}
			return gateAddWaiter(provider, cmd, args[0], notify)
		},
	}

	cmd.Flags().StringVar(&notify, "notify", "", "Address to notify when gate clears (required)")
	_ = cmd.MarkFlagRequired("notify")

	return cmd
}

// newGateAddWaiterCmd creates the "gate add-waiter" command.
// Usage: bd gate add-waiter <gate-id> <waiter>
func newGateAddWaiterCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-waiter <gate-id> <waiter>",
		Short: "Register a waiter on a gate (positional-arg form)",
		Long: `Register an address to be notified when a gate clears.

This is the positional-arg form of "gate wait --notify". Both commands
are functionally identical.

Examples:
  bd gate add-waiter bl-g1 beads_lite/polecats/onyx`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return gateAddWaiter(provider, cmd, args[0], args[1])
		},
	}

	return cmd
}

// gateAddWaiter is the shared implementation for gate wait --notify and gate add-waiter.
func gateAddWaiter(provider *AppProvider, cmd *cobra.Command, gateID, waiter string) error {
	app, err := provider.Get()
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	store, err := app.StorageFor(ctx, gateID)
	if err != nil {
		return fmt.Errorf("routing issue %s: %w", gateID, err)
	}

	issue, err := store.Get(ctx, gateID)
	if err != nil {
		return fmt.Errorf("getting issue %s: %w", gateID, err)
	}

	if issue.Type != issuestorage.TypeGate {
		return fmt.Errorf("issue %s is type %q, not gate", gateID, issue.Type)
	}

	// Check for duplicate
	for _, w := range issue.Waiters {
		if w == waiter {
			if app.JSON {
				result := ToIssueJSON(ctx, store, issue, false, false)
				return json.NewEncoder(app.Out).Encode(result)
			}
			fmt.Fprintf(app.Out, "%s already waiting on %s\n", waiter, gateID)
			return nil
		}
	}

	// Append and persist
	issue.Waiters = append(issue.Waiters, waiter)
	if err := store.Update(ctx, issue); err != nil {
		return fmt.Errorf("updating issue: %w", err)
	}

	if app.JSON {
		updated, err := store.Get(ctx, gateID)
		if err != nil {
			return fmt.Errorf("fetching updated issue: %w", err)
		}
		result := ToIssueJSON(ctx, store, updated, false, false)
		return json.NewEncoder(app.Out).Encode(result)
	}

	fmt.Fprintf(app.Out, "%s Added %s to waiters for %s\n", app.SuccessColor("✓"), waiter, gateID)
	return nil
}
