package cmd

import (
	"encoding/json"
	"fmt"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newGateCmd creates the gate parent command with subcommands.
func newGateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Gate operations for async coordination",
	}

	cmd.AddCommand(newGateWaitCmd(provider))
	cmd.AddCommand(newGateAddWaiterCmd(provider))

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
