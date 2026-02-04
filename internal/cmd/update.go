package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"beads-lite/internal/config"
	"beads-lite/internal/issuestorage"

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
		claim        bool
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
  bd update bd-a1b2 --description -  # read from stdin
  bd update bd-a1b2 --claim          # assign to self + set in-progress`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			// Route to correct storage
			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			// Pre-parse flags that can fail before taking the lock.
			var parsedPriority issuestorage.Priority
			if cmd.Flags().Changed("priority") {
				p, err := parsePriority(priority)
				if err != nil {
					return err
				}
				parsedPriority = p
			}

			var parsedType issuestorage.IssueType
			if cmd.Flags().Changed("type") {
				t, err := parseType(typeFlag, getCustomValues(app, "types.custom"))
				if err != nil {
					return err
				}
				parsedType = t
			}

			var parsedStatus issuestorage.Status
			if cmd.Flags().Changed("status") {
				s, err := parseStatus(status, getCustomValues(app, "status.custom"))
				if err != nil {
					if strings.Contains(err.Error(), "tombstone") {
						// Tombstone rejection: print to stderr but exit 0 (matches reference)
						fmt.Fprintf(app.Err, "Error updating %s: validate field update: %v\n", issueID, err)
						return nil
					}
					return err
				}
				parsedStatus = s
			}

			var desc string
			if cmd.Flags().Changed("description") {
				desc = description
				if description == "-" {
					data, err := io.ReadAll(bufio.NewReader(os.Stdin))
					if err != nil {
						return fmt.Errorf("reading description from stdin: %w", err)
					}
					desc = strings.TrimSpace(string(data))
				}
			}

			var actor string
			if cmd.Flags().Changed("claim") && claim {
				a, err := resolveActor(app)
				if err != nil {
					return fmt.Errorf("claiming issue: %w", err)
				}
				actor = a
			}

			// Handle parent changes first — AddDependency/RemoveDependency
			// have their own locking.
			if cmd.Flags().Changed("parent") {
				// Need current issue to check existing parent for removal.
				issue, err := store.Get(ctx, issueID)
				if err != nil {
					return fmt.Errorf("getting issue %s: %w", issueID, err)
				}
				if parent == "" {
					// Remove parent
					if issue.Parent != "" {
						if err := store.RemoveDependency(ctx, issueID, issue.Parent); err != nil {
							return fmt.Errorf("removing parent: %w", err)
						}
					}
				} else {
					// Set parent - verify it exists
					if _, err := store.Get(ctx, parent); err != nil {
						if err == issuestorage.ErrNotFound {
							return fmt.Errorf("parent issue not found: %s", parent)
						}
						return fmt.Errorf("getting parent issue: %w", err)
					}
					if err := store.AddDependency(ctx, issueID, parent, issuestorage.DepTypeParentChild); err != nil {
						if err == issuestorage.ErrCycle {
							return fmt.Errorf("cannot set parent: would create a cycle")
						}
						return fmt.Errorf("setting parent: %w", err)
					}
				}
			}

			// Check if there are any non-parent field changes.
			hasFieldChanges := cmd.Flags().Changed("title") ||
				cmd.Flags().Changed("description") ||
				cmd.Flags().Changed("priority") ||
				cmd.Flags().Changed("type") ||
				cmd.Flags().Changed("status") ||
				cmd.Flags().Changed("assignee") ||
				(cmd.Flags().Changed("claim") && claim) ||
				len(addLabels) > 0 || len(removeLabels) > 0

			if !hasFieldChanges && !cmd.Flags().Changed("parent") {
				return fmt.Errorf("no changes specified")
			}

			// Apply all non-parent field changes atomically.
			if hasFieldChanges {
				if err := store.Modify(ctx, issueID, func(issue *issuestorage.Issue) error {
					if cmd.Flags().Changed("claim") && claim {
						if issue.Assignee != "" {
							return fmt.Errorf("cannot claim %s: already assigned to %q", issueID, issue.Assignee)
						}
						issue.Assignee = actor
						issue.Status = issuestorage.StatusInProgress
					}
					if cmd.Flags().Changed("title") {
						issue.Title = title
					}
					if cmd.Flags().Changed("description") {
						issue.Description = desc
					}
					if cmd.Flags().Changed("priority") {
						issue.Priority = parsedPriority
					}
					if cmd.Flags().Changed("type") {
						issue.Type = parsedType
					}
					if cmd.Flags().Changed("status") {
						issue.Status = parsedStatus
					}
					if cmd.Flags().Changed("assignee") {
						issue.Assignee = assignee
					}
					if len(addLabels) > 0 || len(removeLabels) > 0 {
						labels := issue.Labels
						if labels == nil {
							labels = []string{}
						}
						for _, toRemove := range removeLabels {
							labels = removeFromSlice(labels, toRemove)
						}
						for _, toAdd := range addLabels {
							if !contains(labels, toAdd) {
								labels = append(labels, toAdd)
							}
						}
						issue.Labels = labels
					}
					return nil
				}); err != nil {
					return fmt.Errorf("updating issue: %w", err)
				}
			}

			// Output the result
			if app.JSON {
				// Fetch the updated issue to return full details
				updatedIssue, err := store.Get(ctx, issueID)
				if err != nil {
					return fmt.Errorf("fetching updated issue: %w", err)
				}
				result := ToIssueJSON(ctx, store, updatedIssue, false, false)
				// Original beads doesn't include parent field in update output
				result.Parent = ""
				// Return as array to match original beads format
				return json.NewEncoder(app.Out).Encode([]IssueJSON{result})
			}

			fmt.Fprintf(app.Out, "%s Updated issue: %s\n", app.SuccessColor("✓"), issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&description, "description", "", "New description (use - for stdin)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "New priority (0-4 or P0-P4)")
	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "New type (task, bug, feature, epic, chore, gate)")
	cmd.Flags().StringVarP(&status, "status", "s", "", "New status ("+statusNames(nil)+")")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assign to user (empty string to unassign)")
	cmd.Flags().StringVar(&parent, "parent", "", "Set parent issue (empty string to remove parent)")
	cmd.Flags().StringSliceVar(&addLabels, "add-label", nil, "Add label (can repeat)")
	cmd.Flags().StringSliceVar(&removeLabels, "remove-label", nil, "Remove label (can repeat)")
	cmd.Flags().BoolVar(&claim, "claim", false, "Claim issue: assign to current actor and set status to in-progress")

	return cmd
}

func parsePriority(s string) (issuestorage.Priority, error) {
	switch strings.ToLower(s) {
	case "0", "p0":
		return issuestorage.PriorityCritical, nil
	case "1", "p1":
		return issuestorage.PriorityHigh, nil
	case "2", "p2":
		return issuestorage.PriorityMedium, nil
	case "3", "p3":
		return issuestorage.PriorityLow, nil
	case "4", "p4":
		return issuestorage.PriorityBacklog, nil
	default:
		return "", fmt.Errorf("invalid priority %q (expected 0-4 or P0-P4, not words like high/medium/low)", s)
	}
}

func parseType(s string, customTypes []string) (issuestorage.IssueType, error) {
	switch strings.ToLower(s) {
	case "task":
		return issuestorage.TypeTask, nil
	case "bug":
		return issuestorage.TypeBug, nil
	case "feature":
		return issuestorage.TypeFeature, nil
	case "epic":
		return issuestorage.TypeEpic, nil
	case "chore":
		return issuestorage.TypeChore, nil
	case "gate":
		return issuestorage.TypeGate, nil
	case "molecule":
		return issuestorage.TypeMolecule, nil
	default:
		lower := strings.ToLower(s)
		for _, ct := range customTypes {
			if strings.ToLower(ct) == lower {
				return issuestorage.IssueType(s), nil
			}
		}
		builtins := "task, bug, feature, epic, chore, gate, molecule"
		if len(customTypes) > 0 {
			builtins += ", " + strings.Join(customTypes, ", ")
		}
		return "", fmt.Errorf("invalid type %q: must be one of %s", s, builtins)
	}
}

func parseStatus(s string, customStatuses []string) (issuestorage.Status, error) {
	switch strings.ToLower(s) {
	case "open":
		return issuestorage.StatusOpen, nil
	case "in-progress", "in_progress", "inprogress":
		return issuestorage.StatusInProgress, nil
	case "blocked":
		return issuestorage.StatusBlocked, nil
	case "deferred":
		return issuestorage.StatusDeferred, nil
	case "hooked":
		return issuestorage.StatusHooked, nil
	case "pinned":
		return issuestorage.StatusPinned, nil
	case "closed":
		return issuestorage.StatusClosed, nil
	case "tombstone":
		return "", fmt.Errorf("cannot set status to tombstone directly; use 'bd delete' instead")
	default:
		lower := strings.ToLower(s)
		for _, cs := range customStatuses {
			if strings.ToLower(cs) == lower {
				return issuestorage.Status(s), nil
			}
		}
		return "", fmt.Errorf("invalid status %q: must be one of %s", s, statusNames(customStatuses))
	}
}

// statusNames returns a comma-separated string of all valid status names
// (built-in + custom).
func statusNames(customStatuses []string) string {
	names := make([]string, 0, len(issuestorage.BuiltinStatuses)+len(customStatuses))
	for _, s := range issuestorage.BuiltinStatuses {
		names = append(names, string(s))
	}
	names = append(names, customStatuses...)
	return strings.Join(names, ", ")
}

// getCustomValues reads a comma-separated config key and returns the split values.
func getCustomValues(app *App, key string) []string {
	if app == nil || app.ConfigStore == nil {
		return nil
	}
	v, ok := app.ConfigStore.Get(key)
	if !ok {
		return nil
	}
	return config.SplitCustomValues(v)
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
