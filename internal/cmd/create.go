package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// newCreateCmd creates the create command.
func newCreateCmd(provider *AppProvider) *cobra.Command {
	var (
		typeFlag    string
		priority    string
		parent      string
		dependsOn   []string
		labels      []string
		assignee    string
		description string
		titleFlag   string
	)

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new issue",
		Long: `Create a new issue with the specified title.

Examples:
  bd create "Fix login bug"
  bd create --title "Fix login bug"
  bd create "Add OAuth support" --type feature --priority high
  bd create "Implement caching" --parent bd-a1b2
  bd create "Write tests" --depends-on bd-e5f6
  bd create "Task" --description -   # read description from stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg, received %d", len(args))
			}

			title := titleFlag
			if len(args) == 1 {
				title = args[0]
			}
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("title is required (provide as argument or --title)")
			}

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
				p, err := parsePriorityInput(priority)
				if err != nil {
					return err
				}
				issuePriority = p
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

			// When --parent is specified, use dot-notation child ID
			if parent != "" {
				childID, err := app.Storage.GetNextChildID(ctx, parent)
				if err != nil {
					return fmt.Errorf("generating child ID for parent %s: %w", parent, err)
				}
				issue.ID = childID
			}

			id, err := app.Storage.Create(ctx, issue)
			if err != nil {
				return fmt.Errorf("creating issue: %w", err)
			}

			// Set parent relationship if specified
			if parent != "" {
				if err := app.Storage.AddDependency(ctx, id, parent, storage.DepTypeParentChild); err != nil {
					// Clean up the created issue on failure
					app.Storage.Delete(context.Background(), id)
					return fmt.Errorf("setting parent %s: %w", parent, err)
				}
			}

			// Add dependencies if specified (default type: blocks)
			for _, depID := range dependsOn {
				if err := app.Storage.AddDependency(ctx, id, depID, storage.DepTypeBlocks); err != nil {
					// Clean up on failure
					app.Storage.Delete(context.Background(), id)
					return fmt.Errorf("adding dependency on %s: %w", depID, err)
				}
			}

			// Output the result
			if app.JSON {
				// Fetch the created issue to get all fields including timestamps
				createdIssue, err := app.Storage.Get(ctx, id)
				if err != nil {
					return fmt.Errorf("fetching created issue: %w", err)
				}
				result := ToIssueJSON(ctx, app.Storage, createdIssue, false, false)
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Warn if no description provided
			if desc == "" {
				fmt.Fprintf(app.Out, "%s Creating issue without description.\n", app.WarnColor("⚠"))
				fmt.Fprintln(app.Out, "  Issues without descriptions lack context for future work.")
				fmt.Fprintln(app.Out, "  Consider adding --description=\"Why this issue exists and what needs to be done\"")
			}

			fmt.Fprintf(app.Out, "%s Created issue: %s\n", app.SuccessColor("✓"), id)
			fmt.Fprintf(app.Out, "  Title: %s\n", title)
			fmt.Fprintf(app.Out, "  Priority: %s\n", issuePriority.Display())
			fmt.Fprintf(app.Out, "  Status: %s\n", storage.StatusOpen)
			return nil
		},
	}

	cmd.Flags().StringVar(&titleFlag, "title", "", "Issue title (required if no positional title is provided)")
	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Issue type (task, bug, feature, epic, chore)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Priority (0-4 or P0-P4)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent issue ID")
	cmd.Flags().StringSliceVarP(&dependsOn, "depends-on", "d", nil, "Issue ID this depends on (can repeat)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Add label (can repeat)")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assign to user")
	cmd.Flags().StringVar(&description, "description", "", "Full description (use - for stdin)")

	return cmd
}

func parsePriorityInput(s string) (storage.Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
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
