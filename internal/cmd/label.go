package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newLabelCmd creates the label command with subcommands.
func newLabelCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage issue labels",
		Long: `Manage labels on issues.

Subcommands:
  add     Add a label to an issue
  remove  Remove a label from an issue
  list    List labels on an issue`,
	}

	cmd.AddCommand(newLabelAddCmd(provider))
	cmd.AddCommand(newLabelRemoveCmd(provider))
	cmd.AddCommand(newLabelListCmd(provider))

	return cmd
}

// newLabelAddCmd creates the "label add" subcommand.
func newLabelAddCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <issue-id> <label>",
		Short: "Add a label to an issue",
		Long: `Add a label to an issue. If the label already exists, the command is a no-op.

Examples:
  bd label add bd-a1b2 urgent
  bd label add bd-a1b2 bug-fix`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]
			label := args[1]

			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			issue, err := resolveIssue(store, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			if err := store.Modify(ctx, issue.ID, func(issue *issuestorage.Issue) error {
				labels := issue.Labels
				if labels == nil {
					labels = []string{}
				}
				if !contains(labels, label) {
					labels = append(labels, label)
					issue.Labels = labels
				}
				return nil
			}); err != nil {
				return fmt.Errorf("updating issue: %w", err)
			}

			if app.JSON {
				updatedIssue, err := store.Get(ctx, issue.ID)
				if err != nil {
					return fmt.Errorf("fetching updated issue: %w", err)
				}
				result := ToIssueJSON(ctx, store, updatedIssue, false, false)
				result.Parent = ""
				return json.NewEncoder(app.Out).Encode([]IssueJSON{result})
			}

			// Re-read for display output.
			issue, _ = store.Get(ctx, issue.ID)
			fmt.Fprintf(app.Out, "%s Added label %q to %s\n", app.SuccessColor("✓"), label, issue.ID)
			fmt.Fprintf(app.Out, "  Labels: %s\n", formatLabelList(issue.Labels))
			return nil
		},
	}

	return cmd
}

// newLabelRemoveCmd creates the "label remove" subcommand.
func newLabelRemoveCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <issue-id> <label>",
		Short: "Remove a label from an issue",
		Long: `Remove a label from an issue. Succeeds silently if the label is not present.

Examples:
  bd label remove bd-a1b2 urgent
  bd label remove bd-a1b2 bug-fix`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]
			label := args[1]

			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			issue, err := resolveIssue(store, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			if err := store.Modify(ctx, issue.ID, func(issue *issuestorage.Issue) error {
				issue.Labels = removeFromSlice(issue.Labels, label)
				return nil
			}); err != nil {
				return fmt.Errorf("updating issue: %w", err)
			}

			if app.JSON {
				updatedIssue, err := store.Get(ctx, issue.ID)
				if err != nil {
					return fmt.Errorf("fetching updated issue: %w", err)
				}
				result := ToIssueJSON(ctx, store, updatedIssue, false, false)
				result.Parent = ""
				return json.NewEncoder(app.Out).Encode([]IssueJSON{result})
			}

			// Re-read for display output.
			issue, _ = store.Get(ctx, issue.ID)
			fmt.Fprintf(app.Out, "%s Removed label %q from %s\n", app.SuccessColor("✓"), label, issue.ID)
			fmt.Fprintf(app.Out, "  Labels: %s\n", formatLabelList(issue.Labels))
			return nil
		},
	}

	return cmd
}

// newLabelListCmd creates the "label list" subcommand.
func newLabelListCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <issue-id>",
		Short: "List labels on an issue",
		Long: `List all labels on an issue.

Examples:
  bd label list bd-a1b2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			issue, err := resolveIssue(store, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			if app.JSON {
				result := ToIssueJSON(ctx, store, issue, false, false)
				result.Parent = ""
				return json.NewEncoder(app.Out).Encode([]IssueJSON{result})
			}

			if len(issue.Labels) == 0 {
				fmt.Fprintf(app.Out, "%s: (no labels)\n", issue.ID)
			} else {
				fmt.Fprintf(app.Out, "%s: %s\n", issue.ID, formatLabelList(issue.Labels))
			}
			return nil
		},
	}

	return cmd
}

// formatLabelList formats a slice of labels for display.
func formatLabelList(labels []string) string {
	if len(labels) == 0 {
		return "(none)"
	}
	return strings.Join(labels, ", ")
}
