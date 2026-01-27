package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"beads2/internal/storage"

	"github.com/spf13/cobra"
)

// NewCreateCmd creates the create command.
func NewCreateCmd(app *App) *cobra.Command {
	var (
		typeFlag    string
		priority    string
		parent      string
		dependsOn   []string
		labels      []string
		assignee    string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new issue",
		Long: `Create a new issue with the specified title.

Examples:
  bd create "Fix login bug"
  bd create "Add OAuth support" --type feature --priority high
  bd create "Implement caching" --parent bd-a1b2
  bd create "Write tests" --depends-on bd-e5f6
  bd create "Task" --description -   # read description from stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			title := args[0]

			// Parse and validate type
			issueType := storage.TypeTask
			if typeFlag != "" {
				switch strings.ToLower(typeFlag) {
				case "task":
					issueType = storage.TypeTask
				case "bug":
					issueType = storage.TypeBug
				case "feature":
					issueType = storage.TypeFeature
				case "epic":
					issueType = storage.TypeEpic
				case "chore":
					issueType = storage.TypeChore
				default:
					return fmt.Errorf("invalid type %q: must be one of task, bug, feature, epic, chore", typeFlag)
				}
			}

			// Parse and validate priority
			issuePriority := storage.PriorityMedium
			if priority != "" {
				switch strings.ToLower(priority) {
				case "critical":
					issuePriority = storage.PriorityCritical
				case "high":
					issuePriority = storage.PriorityHigh
				case "medium":
					issuePriority = storage.PriorityMedium
				case "low":
					issuePriority = storage.PriorityLow
				default:
					return fmt.Errorf("invalid priority %q: must be one of critical, high, medium, low", priority)
				}
			}

			// Handle description from stdin if "-"
			desc := description
			if description == "-" {
				data, err := io.ReadAll(bufio.NewReader(os.Stdin))
				if err != nil {
					return fmt.Errorf("reading description from stdin: %w", err)
				}
				desc = strings.TrimSpace(string(data))
			}

			// Create the issue
			issue := &storage.Issue{
				Title:       title,
				Description: desc,
				Type:        issueType,
				Priority:    issuePriority,
				Labels:      labels,
				Assignee:    assignee,
			}

			id, err := app.Storage.Create(ctx, issue)
			if err != nil {
				return fmt.Errorf("creating issue: %w", err)
			}

			// Set parent relationship if specified
			if parent != "" {
				if err := app.Storage.SetParent(ctx, id, parent); err != nil {
					// Clean up the created issue on failure
					app.Storage.Delete(context.Background(), id)
					return fmt.Errorf("setting parent %s: %w", parent, err)
				}
			}

			// Add dependencies if specified
			for _, depID := range dependsOn {
				if err := app.Storage.AddDependency(ctx, id, depID); err != nil {
					// Clean up on failure
					app.Storage.Delete(context.Background(), id)
					return fmt.Errorf("adding dependency on %s: %w", depID, err)
				}
			}

			// Output the result
			if app.JSON {
				result := map[string]string{"id": id}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintln(app.Out, id)
			return nil
		},
	}

	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Issue type (task, bug, feature, epic, chore)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Priority (critical, high, medium, low)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent issue ID")
	cmd.Flags().StringSliceVarP(&dependsOn, "depends-on", "d", nil, "Issue ID this depends on (can repeat)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Add label (can repeat)")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assign to user")
	cmd.Flags().StringVar(&description, "description", "", "Full description (use - for stdin)")

	return cmd
}
