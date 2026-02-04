package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newCloseCmd creates the close command.
func newCloseCmd(provider *AppProvider) *cobra.Command {
	var (
		continueFlag bool
		noAuto       bool
		suggestNext  bool
		reason       string
	)

	cmd := &cobra.Command{
		Use:   "close <issue-id> [issue-id...]",
		Short: "Close one or more issues",
		Long: `Close one or more issues by moving them from open/ to closed/ directory.

Sets status to closed and records the closed_at timestamp.

Examples:
  bd close bd-a1b2
  bd close bd-a1b2 bd-c3d4 bd-e5f6
  bd close --reason "Won't fix" bd-a1b2`,
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
				if err := store.Modify(ctx, issueID, func(i *issuestorage.Issue) error {
					i.Status = issuestorage.StatusClosed
					if reason != "" {
						i.CloseReason = reason
					}
					return nil
				}); err != nil {
					errors = append(errors, fmt.Errorf("closing %s: %w", issueID, err))
				} else {
					closed = append(closed, issueID)
				}
			}

			// Use local storage for molecule/dependent lookups
			store := app.Storage

			// JSON output
			if app.JSON {
				var issues []IssueJSON
				for _, id := range closed {
					s, err := app.StorageFor(ctx, id)
					if err != nil {
						continue
					}
					issue, err := s.Get(ctx, id)
					if err != nil {
						continue
					}
					issues = append(issues, ToIssueJSON(ctx, s, issue, false, false))
				}

				// --continue logic (JSON): wrap in {closed, continue} format
				if continueFlag {
					var continueResult *CloseContinueJSON
					for _, issueID := range closed {
						nextStep := findNextMoleculeStep(ctx, store, issueID)
						// Find molecule root for the closed issue.
						closedIssue, _ := store.Get(ctx, issueID)
						var molID string
						if closedIssue != nil && closedIssue.Parent != "" {
							if root, err := graph.FindMoleculeRoot(ctx, store, issueID); err == nil {
								molID = root.ID
							}
						}
						// Find the closed step's JSON for the continue block.
						var closedStep *MolIssueJSON
						if closedIssue != nil {
							cs := ToMolIssueJSON(closedIssue)
							closedStep = &cs
						}
						autoAdvanced := false
						if nextStep != nil && !noAuto {
							store.Modify(ctx, nextStep.ID, func(i *issuestorage.Issue) error {
								i.Status = issuestorage.StatusInProgress
								return nil
							})
							nextStep.Status = issuestorage.StatusInProgress
							autoAdvanced = true
						}
						var nextStepJSON *MolIssueJSON
						if nextStep != nil {
							ns := ToMolIssueJSON(nextStep)
							nextStepJSON = &ns
						}
						// Check if molecule is complete (no more steps).
						molComplete := nextStep == nil
						continueResult = &CloseContinueJSON{
							AutoAdvanced:     autoAdvanced,
							ClosedStep:       closedStep,
							MoleculeComplete: molComplete,
							MoleculeID:       molID,
							NextStep:         nextStepJSON,
						}
					}
					return json.NewEncoder(app.Out).Encode(CloseWithContinueJSON{
						Closed:   issues,
						Continue: continueResult,
					})
				}

				// --suggest-next logic (JSON)
				if suggestNext {
					for _, issueID := range closed {
						findUnblockedDependents(ctx, store, issueID)
					}
				}

				return json.NewEncoder(app.Out).Encode(issues)
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
						if err := store.Modify(ctx, nextStep.ID, func(i *issuestorage.Issue) error {
							i.Status = issuestorage.StatusInProgress
							return nil
						}); err != nil {
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
	cmd.Flags().StringVar(&reason, "reason", "", "Set the close reason (default: \"Closed\")")

	return cmd
}

// findNextMoleculeStep finds the next ready step in a molecule after the given issue.
// Returns nil if the issue is not part of a molecule or there are no more steps.
func findNextMoleculeStep(ctx context.Context, store issuestorage.IssueStore, issueID string) *issuestorage.Issue {
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
func findUnblockedDependents(ctx context.Context, store issuestorage.IssueStore, issueID string) []map[string]string {
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
		if dep.Type != issuestorage.DepTypeBlocks {
			continue
		}
		dependent, err := store.Get(ctx, dep.ID)
		if err != nil || dependent.Status == issuestorage.StatusClosed {
			continue
		}
		allResolved := true
		blocksType := issuestorage.DepTypeBlocks
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
