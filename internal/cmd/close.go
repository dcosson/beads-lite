package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// newCloseCmd creates the close command.
func newCloseCmd(provider *AppProvider) *cobra.Command {
	var (
		continueFlag bool
		noAuto       bool
		suggestNext  bool
	)

	cmd := &cobra.Command{
		Use:   "close <issue-id> [issue-id...]",
		Short: "Close one or more issues",
		Long: `Close one or more issues by moving them from open/ to closed/ directory.

Sets status to closed and records the closed_at timestamp.

Examples:
  bd close bd-a1b2
  bd close bd-a1b2 bd-c3d4 bd-e5f6`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			var closed []string
			var errors []error

			for _, issueID := range args {
				store, err := app.StorageFor(ctx, issueID)
				if err != nil {
					errors = append(errors, fmt.Errorf("routing %s: %w", issueID, err))
					continue
				}
				if err := store.Close(ctx, issueID); err != nil {
					errors = append(errors, fmt.Errorf("closing %s: %w", issueID, err))
				} else {
					closed = append(closed, issueID)
				}
			}

			// Use local storage for molecule/dependent lookups
			store := app.Storage

			// JSON output
			if app.JSON {
				result := map[string]interface{}{
					"closed": closed,
				}
				if len(errors) > 0 {
					errStrings := make([]string, len(errors))
					for i, e := range errors {
						errStrings[i] = e.Error()
					}
					result["errors"] = errStrings
				}

				// --continue logic (JSON)
				if continueFlag {
					for _, issueID := range closed {
						nextStep := findNextMoleculeStep(ctx, store, issueID)
						if nextStep == nil {
							result["next_step"] = nil
							continue
						}
						stepInfo := map[string]string{
							"id":    nextStep.ID,
							"title": nextStep.Title,
						}
						if !noAuto {
							nextStep.Status = storage.StatusInProgress
							if err := store.Update(ctx, nextStep); err == nil {
								stepInfo["status"] = string(storage.StatusInProgress)
							}
						}
						result["next_step"] = stepInfo
					}
				}

				// --suggest-next logic (JSON)
				if suggestNext {
					var unblocked []map[string]string
					for _, issueID := range closed {
						unblocked = append(unblocked, findUnblockedDependents(ctx, store, issueID)...)
					}
					result["unblocked"] = unblocked
				}

				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output
			for _, id := range closed {
				fmt.Fprintf(app.Out, "Closed %s\n", id)
			}

			// --continue logic (text)
			if continueFlag {
				for _, issueID := range closed {
					nextStep := findNextMoleculeStep(ctx, store, issueID)
					if nextStep == nil {
						fmt.Fprintf(app.Out, "No more steps\n")
						continue
					}
					if noAuto {
						fmt.Fprintf(app.Out, "Next step: %s %s\n", nextStep.ID, nextStep.Title)
					} else {
						nextStep.Status = storage.StatusInProgress
						if err := store.Update(ctx, nextStep); err != nil {
							fmt.Fprintf(app.Err, "Error advancing to next step: %v\n", err)
						} else {
							fmt.Fprintf(app.Out, "Advanced to %s %s\n", nextStep.ID, nextStep.Title)
						}
					}
				}
			}

			// --suggest-next logic (text)
			if suggestNext {
				for _, issueID := range closed {
					unblocked := findUnblockedDependents(ctx, store, issueID)
					for _, u := range unblocked {
						fmt.Fprintf(app.Out, "Unblocked: %s %s\n", u["id"], u["title"])
					}
				}
			}

			// Return first error if any
			if len(errors) > 0 {
				for _, e := range errors {
					fmt.Fprintf(app.Err, "Error: %v\n", e)
				}
				return errors[0]
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&continueFlag, "continue", false, "Auto-advance to next molecule step")
	cmd.Flags().BoolVar(&noAuto, "no-auto", false, "With --continue: show next step without claiming it")
	cmd.Flags().BoolVar(&suggestNext, "suggest-next", false, "Show newly unblocked issues after close")

	return cmd
}

// findNextMoleculeStep finds the next ready step in a molecule after the given issue.
// Returns nil if the issue is not part of a molecule or there are no more steps.
func findNextMoleculeStep(ctx context.Context, store storage.Storage, issueID string) *storage.Issue {
	issue, err := store.Get(ctx, issueID)
	if err != nil || issue.Parent == "" {
		return nil
	}

	root, err := graph.FindMoleculeRoot(ctx, store, issueID)
	if err != nil {
		return nil
	}

	children, err := graph.CollectMoleculeChildren(ctx, store, root.ID)
	if err != nil || len(children) == 0 {
		return nil
	}

	closedSet, err := graph.BuildClosedSet(ctx, store)
	if err != nil {
		return nil
	}

	ordered, err := graph.TopologicalOrder(children)
	if err != nil {
		return nil
	}

	return graph.FindNextStep(ordered, issueID, closedSet)
}

// findUnblockedDependents returns dependents of the given issue that are newly unblocked.
func findUnblockedDependents(ctx context.Context, store storage.Storage, issueID string) []map[string]string {
	issue, err := store.Get(ctx, issueID)
	if err != nil {
		return nil
	}

	closedSet, err := graph.BuildClosedSet(ctx, store)
	if err != nil {
		return nil
	}

	var unblocked []map[string]string
	for _, dep := range issue.Dependents {
		if dep.Type != storage.DepTypeBlocks {
			continue
		}
		dependent, err := store.Get(ctx, dep.ID)
		if err != nil || dependent.Status == storage.StatusClosed {
			continue
		}
		allResolved := true
		blocksType := storage.DepTypeBlocks
		for _, blockingID := range dependent.DependencyIDs(&blocksType) {
			if !closedSet[blockingID] {
				allResolved = false
				break
			}
		}
		if allResolved {
			unblocked = append(unblocked, map[string]string{
				"id":    dependent.ID,
				"title": dependent.Title,
			})
		}
	}
	return unblocked
}
