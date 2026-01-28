package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// newUpdateCmd creates the update command.
func newUpdateCmd(provider *AppProvider) *cobra.Command {
	var (
		title        string
		description  string
		priority     string
		typeFlag     string
		status       string
		assignee     string
		parent       string
		addLabels    []string
		removeLabels []string
	)

	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "Update an existing issue",
		Long: `Update fields of an existing issue.

Examples:
  bd update bd-a1b2 --title "New title"
  bd update bd-a1b2 --priority 0
  bd update bd-a1b2 --status in-progress
  bd update bd-a1b2 --add-label urgent --remove-label backlog
  bd update bd-a1b2 --assignee alice
  bd update bd-a1b2 --assignee ""     # unassign
  bd update bd-a1b2 --parent bd-c3d4 # set parent
  bd update bd-a1b2 --parent ""      # remove parent
  bd update bd-a1b2 --description -  # read from stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			// Fetch the existing issue
			issue, err := app.Storage.Get(ctx, issueID)
			if err != nil {
				return fmt.Errorf("getting issue %s: %w", issueID, err)
			}

			// Track if any changes were made
			changed := false

			// Update title if specified
			if cmd.Flags().Changed("title") {
				issue.Title = title
				changed = true
			}

			// Update description if specified
			if cmd.Flags().Changed("description") {
				desc := description
				if description == "-" {
					data, err := io.ReadAll(bufio.NewReader(os.Stdin))
					if err != nil {
						return fmt.Errorf("reading description from stdin: %w", err)
					}
					desc = strings.TrimSpace(string(data))
				}
				issue.Description = desc
				changed = true
			}

			// Update priority if specified
			if cmd.Flags().Changed("priority") {
				p, err := parsePriority(priority)
				if err != nil {
					return err
				}
				issue.Priority = p
				changed = true
			}

			// Update type if specified
			if cmd.Flags().Changed("type") {
				t, err := parseType(typeFlag)
				if err != nil {
					return err
				}
				issue.Type = t
				changed = true
			}

			// Update status if specified
			if cmd.Flags().Changed("status") {
				s, err := parseStatus(status)
				if err != nil {
					return err
				}
				issue.Status = s
				changed = true
			}

			// Update assignee if specified (including empty string to unassign)
			if cmd.Flags().Changed("assignee") {
				issue.Assignee = assignee
				changed = true
			}

			// Update parent if specified
			if cmd.Flags().Changed("parent") {
				if parent == "" {
					// Remove parent
					if issue.Parent != "" {
						if err := app.Storage.RemoveParent(ctx, issueID); err != nil {
							return fmt.Errorf("removing parent: %w", err)
						}
					}
				} else {
					// Set parent - verify it exists
					if _, err := app.Storage.Get(ctx, parent); err != nil {
						if err == storage.ErrNotFound {
							return fmt.Errorf("parent issue not found: %s", parent)
						}
						return fmt.Errorf("getting parent issue: %w", err)
					}
					if err := app.Storage.SetParent(ctx, issueID, parent); err != nil {
						if err == storage.ErrCycle {
							return fmt.Errorf("cannot set parent: would create a cycle")
						}
						return fmt.Errorf("setting parent: %w", err)
					}
				}
				// Re-fetch issue since SetParent/RemoveParent modify storage directly
				issue, err = app.Storage.Get(ctx, issueID)
				if err != nil {
					return fmt.Errorf("re-fetching issue after parent update: %w", err)
				}
				changed = true
			}

			// Handle label modifications
			if len(addLabels) > 0 || len(removeLabels) > 0 {
				labels := issue.Labels
				if labels == nil {
					labels = []string{}
				}

				// Remove labels first
				for _, toRemove := range removeLabels {
					labels = removeFromSlice(labels, toRemove)
				}

				// Then add new labels (avoiding duplicates)
				for _, toAdd := range addLabels {
					if !contains(labels, toAdd) {
						labels = append(labels, toAdd)
					}
				}

				issue.Labels = labels
				changed = true
			}

			// Only update if something changed
			if !changed {
				return fmt.Errorf("no changes specified")
			}

			// Save the updated issue
			if err := app.Storage.Update(ctx, issue); err != nil {
				return fmt.Errorf("updating issue: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{"id": issueID, "status": "updated"}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "%s Updated issue: %s\n", app.SuccessColor("✓"), issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&description, "description", "", "New description (use - for stdin)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "New priority (0-4 or P0-P4)")
	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "New type (task, bug, feature, epic, chore)")
	cmd.Flags().StringVarP(&status, "status", "s", "", "New status (open, in-progress, blocked, deferred, closed)")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assign to user (empty string to unassign)")
	cmd.Flags().StringVar(&parent, "parent", "", "Set parent issue (empty string to remove parent)")
	cmd.Flags().StringSliceVar(&addLabels, "add-label", nil, "Add label (can repeat)")
	cmd.Flags().StringSliceVar(&removeLabels, "remove-label", nil, "Remove label (can repeat)")

	return cmd
}

func parsePriority(s string) (storage.Priority, error) {
	switch strings.ToLower(s) {
	case "0", "p0":
		return storage.PriorityCritical, nil
	case "1", "p1":
		return storage.PriorityHigh, nil
	case "2", "p2":
		return storage.PriorityMedium, nil
	case "3", "p3":
		return storage.PriorityLow, nil
	case "4", "p4":
		return storage.PriorityBacklog, nil
	default:
		return "", fmt.Errorf("invalid priority %q (expected 0-4 or P0-P4, not words like high/medium/low)", s)
	}
}

func parseType(s string) (storage.IssueType, error) {
	switch strings.ToLower(s) {
	case "task":
		return storage.TypeTask, nil
	case "bug":
		return storage.TypeBug, nil
	case "feature":
		return storage.TypeFeature, nil
	case "epic":
		return storage.TypeEpic, nil
	case "chore":
		return storage.TypeChore, nil
	default:
		return "", fmt.Errorf("invalid type %q: must be one of task, bug, feature, epic, chore", s)
	}
}

func parseStatus(s string) (storage.Status, error) {
	switch strings.ToLower(s) {
	case "open":
		return storage.StatusOpen, nil
	case "in-progress", "in_progress", "inprogress":
		return storage.StatusInProgress, nil
	case "blocked":
		return storage.StatusBlocked, nil
	case "deferred":
		return storage.StatusDeferred, nil
	case "closed":
		return storage.StatusClosed, nil
	default:
		return "", fmt.Errorf("invalid status %q: must be one of open, in-progress, blocked, deferred, closed", s)
	}
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
