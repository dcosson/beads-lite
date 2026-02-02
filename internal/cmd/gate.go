package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// newGateCmd creates the gate command group.
func newGateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Gate commands for async coordination primitives",
	}

	cmd.AddCommand(newGateShowCmd(provider))
	cmd.AddCommand(newGateListCmd(provider))
	cmd.AddCommand(newGateWaitCmd(provider))
	cmd.AddCommand(newGateAddWaiterCmd(provider))
	cmd.AddCommand(newGateResolveCmd(provider))
	cmd.AddCommand(newGateCheckCmd(provider))

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

// newGateResolveCmd creates the "gate resolve" subcommand.
func newGateResolveCmd(provider *AppProvider) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "resolve <gate-id>",
		Short: "Resolve (close) a gate",
		Long: `Resolve a gate by closing it. This is a convenience wrapper around
the normal close path that validates the issue is a gate.

Resolving a gate = closing it. If --reason is provided, the close reason
is set on the gate before closing.

Examples:
  bd gate resolve bl-abc123
  bd gate resolve bl-abc123 --reason "CI passed"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			gateID := args[0]

			store, err := app.StorageFor(ctx, gateID)
			if err != nil {
				return fmt.Errorf("routing %s: %w", gateID, err)
			}

			// Load the gate issue and validate
			issue, err := store.Get(ctx, gateID)
			if err != nil {
				return fmt.Errorf("getting %s: %w", gateID, err)
			}

			if issue.Type != issuestorage.TypeGate {
				return fmt.Errorf("%s is not a gate (type is %q)", gateID, issue.Type)
			}

			if issue.Status == issuestorage.StatusClosed {
				return fmt.Errorf("gate %s is already closed", gateID)
			}

			// Close the gate (same path as bd close)
			if err := store.Close(ctx, gateID); err != nil {
				return fmt.Errorf("closing gate %s: %w", gateID, err)
			}

			// If --reason was provided, update the close reason on the now-closed issue
			if reason != "" {
				closed, err := store.Get(ctx, gateID)
				if err != nil {
					return fmt.Errorf("updating close reason for %s: %w", gateID, err)
				}
				closed.CloseReason = reason
				if err := store.Update(ctx, closed); err != nil {
					return fmt.Errorf("updating close reason for %s: %w", gateID, err)
				}
			}

			// Output
			if app.JSON {
				resolved, err := store.Get(ctx, gateID)
				if err != nil {
					return fmt.Errorf("reading closed gate %s: %w", gateID, err)
				}
				return json.NewEncoder(app.Out).Encode(ToIssueJSON(ctx, store, resolved, false, false))
			}

			fmt.Fprintf(app.Out, "Resolved gate %s\n", gateID)
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason for resolving the gate")

	return cmd
}
