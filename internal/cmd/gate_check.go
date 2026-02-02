package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// GateCheckResultJSON is the JSON output format for a single gate check result.
type GateCheckResultJSON struct {
	GateID    string `json:"gate_id"`
	AwaitType string `json:"await_type"`
	Result    string `json:"result"` // "resolved", "skipped", "pending", "escalate"
	Reason    string `json:"reason"`
}

// commandExecutor runs an external command and returns its stdout.
type commandExecutor func(name string, args ...string) ([]byte, error)

func defaultCommandExecutor(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func newGateCheckCmd(provider *AppProvider) *cobra.Command {
	_, ghErr := exec.LookPath("gh")
	return gateCheckCmd(provider, defaultCommandExecutor, ghErr == nil)
}

// gateCheckCmd builds the gate check cobra command. Separated from newGateCheckCmd
// so tests can inject a mock executor and control gh availability.
func gateCheckCmd(provider *AppProvider, executor commandExecutor, ghAvailable bool) *cobra.Command {
	var typeFilter string
	var dryRun bool
	var escalate bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check open gates and auto-close satisfied ones",
		Long: `Evaluate all open gates and auto-close any whose conditions are satisfied.

For each open gate, evaluates based on await_type:
  human    - Skipped (manual only, never auto-closed)
  timer    - Closes if created_at + timeout has passed
  gh:run   - Closes if GitHub Actions run completed successfully
  gh:pr    - Closes if pull request was merged
  bead     - Closes if referenced bead is closed

Use --dry-run to see what would happen without making changes.
Use --escalate to report failed conditions (e.g., CI failure, PR closed without merge).

Examples:
  bd gate check                    # Check and close all satisfied gates
  bd gate check --type timer       # Only check timer gates
  bd gate check --dry-run          # Report without closing
  bd gate check --escalate         # Also report failed conditions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// List all open gates
			gateType := issuestorage.TypeGate
			openStatus := issuestorage.StatusOpen
			filter := &issuestorage.ListFilter{
				Type:   &gateType,
				Status: &openStatus,
			}

			gates, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing open gates: %w", err)
			}

			// Filter by --type if specified
			if typeFilter != "" {
				var filtered []*issuestorage.Issue
				for _, g := range gates {
					if g.AwaitType == typeFilter {
						filtered = append(filtered, g)
					}
				}
				gates = filtered
			}

			checker := &gateChecker{
				app:         app,
				executor:    executor,
				now:         time.Now(),
				escalate:    escalate,
				ghAvailable: ghAvailable,
			}

			var results []GateCheckResultJSON
			for _, gate := range gates {
				r, shouldClose := checker.evaluate(ctx, gate)

				if shouldClose && !dryRun {
					if closeErr := app.Storage.Modify(ctx, gate.ID, func(i *issuestorage.Issue) error {
						i.Status = issuestorage.StatusClosed
						return nil
					}); closeErr != nil {
						fmt.Fprintf(app.Err, "warning: failed to close gate %s: %v\n", gate.ID, closeErr)
						r.Result = "pending"
						r.Reason = fmt.Sprintf("close failed: %v", closeErr)
					}
				}

				results = append(results, r)
			}

			// JSON output
			if app.JSON {
				if results == nil {
					results = []GateCheckResultJSON{}
				}
				return json.NewEncoder(app.Out).Encode(results)
			}

			// Text output
			if len(results) == 0 {
				fmt.Fprintln(app.Out, "No open gates found.")
				return nil
			}

			for _, r := range results {
				fmt.Fprintf(app.Out, "%s %s [%s] %s: %s\n",
					resultSymbol(r.Result), r.GateID, r.AwaitType, r.Result, r.Reason)
			}

			// Summary line
			counts := map[string]int{}
			for _, r := range results {
				counts[r.Result]++
			}
			fmt.Fprintf(app.Out, "\nChecked %d gate(s):", len(results))
			for _, status := range []string{"resolved", "pending", "skipped", "escalate"} {
				if c := counts[status]; c > 0 {
					fmt.Fprintf(app.Out, " %d %s", c, status)
				}
			}
			fmt.Fprintln(app.Out)

			return nil
		},
	}

	cmd.Flags().StringVar(&typeFilter, "type", "", "Only check gates with this await_type")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report what would close without actually closing")
	cmd.Flags().BoolVar(&escalate, "escalate", false, "Report failed conditions for escalation")

	return cmd
}

func resultSymbol(result string) string {
	switch result {
	case "resolved":
		return "✓"
	case "skipped":
		return "-"
	case "pending":
		return "○"
	case "escalate":
		return "!"
	default:
		return "?"
	}
}

// gateChecker evaluates gate conditions.
type gateChecker struct {
	app         *App
	executor    commandExecutor
	now         time.Time
	escalate    bool
	ghAvailable bool
}

// evaluate checks a single gate and returns the result and whether the gate should be closed.
func (c *gateChecker) evaluate(ctx context.Context, gate *issuestorage.Issue) (GateCheckResultJSON, bool) {
	r := GateCheckResultJSON{
		GateID:    gate.ID,
		AwaitType: gate.AwaitType,
	}

	switch gate.AwaitType {
	case "human":
		r.Result = "skipped"
		r.Reason = "manual gate (human-only)"
		return r, false

	case "timer":
		return c.evaluateTimer(gate, r)

	case "bead":
		return c.evaluateBead(ctx, gate, r)

	case "gh:run":
		return c.evaluateGHRun(gate, r)

	case "gh:pr":
		return c.evaluateGHPR(gate, r)

	default:
		r.Result = "skipped"
		r.Reason = fmt.Sprintf("unknown await_type %q", gate.AwaitType)
		return r, false
	}
}

func (c *gateChecker) evaluateTimer(gate *issuestorage.Issue, r GateCheckResultJSON) (GateCheckResultJSON, bool) {
	if gate.TimeoutNS == 0 {
		r.Result = "pending"
		r.Reason = "no timeout configured"
		return r, false
	}

	deadline := gate.CreatedAt.Add(time.Duration(gate.TimeoutNS))
	if c.now.After(deadline) {
		if c.escalate {
			r.Result = "escalate"
			r.Reason = fmt.Sprintf("timer expired at %s (no prior resolution)", deadline.Format(time.RFC3339))
		} else {
			r.Result = "resolved"
			r.Reason = fmt.Sprintf("deadline passed (%s)", deadline.Format(time.RFC3339))
		}
		return r, true
	}

	r.Result = "pending"
	remaining := deadline.Sub(c.now).Truncate(time.Second)
	r.Reason = fmt.Sprintf("deadline in %s", remaining)
	return r, false
}

func (c *gateChecker) evaluateBead(ctx context.Context, gate *issuestorage.Issue, r GateCheckResultJSON) (GateCheckResultJSON, bool) {
	if gate.AwaitID == "" {
		r.Result = "pending"
		r.Reason = "no await_id configured"
		return r, false
	}

	store, err := c.app.StorageFor(ctx, gate.AwaitID)
	if err != nil {
		r.Result = "pending"
		r.Reason = fmt.Sprintf("routing error for %s: %v", gate.AwaitID, err)
		return r, false
	}

	target, err := store.Get(ctx, gate.AwaitID)
	if err != nil {
		r.Result = "pending"
		r.Reason = fmt.Sprintf("cannot find bead %s: %v", gate.AwaitID, err)
		return r, false
	}

	if target.Status == issuestorage.StatusClosed {
		r.Result = "resolved"
		r.Reason = fmt.Sprintf("bead %s is closed", gate.AwaitID)
		return r, true
	}

	r.Result = "pending"
	r.Reason = fmt.Sprintf("bead %s is %s", gate.AwaitID, target.Status)
	return r, false
}

func (c *gateChecker) evaluateGHRun(gate *issuestorage.Issue, r GateCheckResultJSON) (GateCheckResultJSON, bool) {
	if !c.ghAvailable {
		r.Result = "skipped"
		r.Reason = "gh CLI not available"
		return r, false
	}

	if gate.AwaitID == "" {
		r.Result = "pending"
		r.Reason = "no await_id configured"
		return r, false
	}

	output, err := c.executor("gh", "run", "view", gate.AwaitID, "--json", "status,conclusion")
	if err != nil {
		r.Result = "pending"
		r.Reason = fmt.Sprintf("gh run view failed: %v", err)
		return r, false
	}

	var ghResult struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal(output, &ghResult); err != nil {
		r.Result = "pending"
		r.Reason = fmt.Sprintf("failed to parse gh output: %v", err)
		return r, false
	}

	if ghResult.Status == "completed" && ghResult.Conclusion == "success" {
		r.Result = "resolved"
		r.Reason = "run completed successfully"
		return r, true
	}

	if ghResult.Status == "completed" && (ghResult.Conclusion == "failure" || ghResult.Conclusion == "cancelled") {
		if c.escalate {
			r.Result = "escalate"
		} else {
			r.Result = "pending"
		}
		r.Reason = fmt.Sprintf("run %s with conclusion: %s", ghResult.Status, ghResult.Conclusion)
		return r, false
	}

	r.Result = "pending"
	r.Reason = fmt.Sprintf("run status: %s", ghResult.Status)
	return r, false
}

func (c *gateChecker) evaluateGHPR(gate *issuestorage.Issue, r GateCheckResultJSON) (GateCheckResultJSON, bool) {
	if !c.ghAvailable {
		r.Result = "skipped"
		r.Reason = "gh CLI not available"
		return r, false
	}

	if gate.AwaitID == "" {
		r.Result = "pending"
		r.Reason = "no await_id configured"
		return r, false
	}

	output, err := c.executor("gh", "pr", "view", gate.AwaitID, "--json", "state")
	if err != nil {
		r.Result = "pending"
		r.Reason = fmt.Sprintf("gh pr view failed: %v", err)
		return r, false
	}

	var ghResult struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(output, &ghResult); err != nil {
		r.Result = "pending"
		r.Reason = fmt.Sprintf("failed to parse gh output: %v", err)
		return r, false
	}

	if ghResult.State == "MERGED" {
		r.Result = "resolved"
		r.Reason = "PR merged"
		return r, true
	}

	if ghResult.State == "CLOSED" {
		if c.escalate {
			r.Result = "escalate"
		} else {
			r.Result = "pending"
		}
		r.Reason = "PR closed without merge"
		return r, false
	}

	r.Result = "pending"
	r.Reason = fmt.Sprintf("PR state: %s", ghResult.State)
	return r, false
}
