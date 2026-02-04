package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newCreateCmd creates the create command.
func newCreateCmd(provider *AppProvider) *cobra.Command {
	var (
		typeFlag    string
		priority    string
		parent      string
		deps        []string
		labels      []string
		assignee    string
		description string
		titleFlag   string
		molType     string
		idFlag      string
		forceFlag   bool
		ephemeral   bool
		actorFlag   string
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
  bd create "Write tests" --deps bd-e5f6
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

			// Validate --id flag
			if idFlag != "" && parent != "" {
				return fmt.Errorf("--id and --parent cannot be combined")
			}
			if idFlag != "" {
				if err := validateCustomID(idFlag, app, forceFlag); err != nil {
					return err
				}
			}

			// Merge --label alias into --labels
			labelAlias, _ := cmd.Flags().GetStringSlice("label")
			if len(labelAlias) > 0 {
				labels = append(labels, labelAlias...)
			}

			// Parse and validate type
			issueType := issuestorage.TypeTask
			if typeFlag != "" {
				t, err := parseType(typeFlag, getCustomValues(app, "types.custom"))
				if err != nil {
					return err
				}
				issueType = t
			}

			// Parse and validate priority
			issuePriority := issuestorage.PriorityMedium
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

			// Enforce required description if configured
			if app.ConfigStore != nil {
				if v, ok := app.ConfigStore.Get("create.require-description"); ok && v == "true" {
					if strings.TrimSpace(desc) == "" {
						return fmt.Errorf("description is required (create.require-description is enabled)")
					}
				}
			}

			// Parse and validate mol_type
			var issueMolType issuestorage.MolType
			if molType != "" {
				if !issuestorage.ValidateMolType(molType) {
					return fmt.Errorf("invalid mol-type %q: must be one of swarm, patrol, work", molType)
				}
				issueMolType = issuestorage.MolType(molType)
			}

			// Resolve actor identity for created_by/owner
			var actor string
			if actorFlag != "" {
				actor = actorFlag
			} else {
				actor, _ = resolveActor(app)
			}
			owner := resolveOwner()

			// Create the issue
			issue := &issuestorage.Issue{
				Title:       title,
				Description: desc,
				Type:        issueType,
				MolType:     issueMolType,
				Priority:    issuePriority,
				CreatedBy:   actor,
				Owner:       owner,
				Labels:      labels,
				Assignee:    assignee,
				Ephemeral:   ephemeral,
			}

			// When --id is specified, use the explicit ID
			if idFlag != "" {
				// Check if the ID exists as a tombstone and clean it up
				existing, err := app.Storage.Get(ctx, idFlag)
				if err == nil && existing.Status == issuestorage.StatusTombstone {
					app.Storage.Delete(ctx, idFlag)
				}
				issue.ID = idFlag
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
				if err := app.Storage.AddDependency(ctx, id, parent, issuestorage.DepTypeParentChild); err != nil {
					// Clean up the created issue on failure
					app.Storage.Delete(context.Background(), id)
					return fmt.Errorf("setting parent %s: %w", parent, err)
				}
			}

			// Add dependencies if specified (default type: blocks)
			for _, dep := range deps {
				depType, depID, err := parseCreateDependency(dep)
				if err != nil {
					app.Storage.Delete(context.Background(), id)
					return err
				}
				if err := app.Storage.AddDependency(ctx, id, depID, depType); err != nil {
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
			fmt.Fprintf(app.Out, "  Status: %s\n", issuestorage.StatusOpen)
			return nil
		},
	}

	cmd.Flags().StringVar(&titleFlag, "title", "", "Issue title (required if no positional title is provided)")
	cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Issue type (task, bug, feature, epic, chore, gate)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Priority (0-4 or P0-P4)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent issue ID")
	cmd.Flags().StringSliceVarP(&deps, "deps", "d", nil, "Dependencies in format 'type:id' or 'id' (can repeat)")
	cmd.Flags().StringSliceVarP(&labels, "labels", "l", nil, "Labels (comma-separated or repeat flag)")
	cmd.Flags().StringSlice("label", nil, "Alias for --labels")
	cmd.Flags().MarkHidden("label")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assign to user")
	cmd.Flags().StringVar(&description, "description", "", "Full description (use - for stdin)")
	cmd.Flags().StringVar(&molType, "mol-type", "", "Molecule type (swarm, patrol, work)")
	cmd.Flags().StringVar(&idFlag, "id", "", "Explicit issue ID (must match configured prefix)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Bypass prefix validation for --id")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", false, "Mark issue as ephemeral (not exported to JSONL)")
	cmd.Flags().StringVar(&actorFlag, "actor", "", "Override actor identity for created_by")

	return cmd
}

func parsePriorityInput(s string) (issuestorage.Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
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
		return -1, fmt.Errorf("invalid priority %q (expected 0-4 or P0-P4, not words like high/medium/low)", s)
	}
}

func parseCreateDependency(input string) (issuestorage.DependencyType, string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", fmt.Errorf("dependency cannot be empty")
	}

	depType := issuestorage.DepTypeBlocks
	depID := trimmed

	if strings.Count(trimmed, ":") > 1 {
		return "", "", fmt.Errorf("invalid dependency %q (expected 'type:id' or 'id')", input)
	}
	if strings.Contains(trimmed, ":") {
		parts := strings.SplitN(trimmed, ":", 2)
		typePart := strings.ToLower(strings.TrimSpace(parts[0]))
		idPart := strings.TrimSpace(parts[1])
		if typePart == "" || idPart == "" {
			return "", "", fmt.Errorf("invalid dependency %q (expected 'type:id' or 'id')", input)
		}
		depType = issuestorage.DependencyType(typePart)
		depID = idPart
	}

	if !issuestorage.ValidDependencyTypes[depType] {
		return "", "", fmt.Errorf("invalid dependency type %q; valid types: %s", depType, validDependencyTypeList())
	}

	return depType, depID, nil
}

// validateCustomID checks that the given ID is well-formed and uses
// an allowed prefix. When force is true, prefix validation is skipped.
func validateCustomID(id string, app *App, force bool) error {
	if !strings.Contains(id, "-") {
		return fmt.Errorf("custom ID %q must contain a hyphen (e.g. \"prefix-name\")", id)
	}

	if force {
		return nil
	}

	prefix := id[:strings.Index(id, "-")]

	// Build the set of valid prefixes
	issuePrefix := "bd" // default
	if app.ConfigStore != nil {
		if v, ok := app.ConfigStore.Get("issue_prefix"); ok {
			issuePrefix = v
		}
	}

	validPrefixes := map[string]bool{issuePrefix: true}
	for _, p := range getCustomValues(app, "allowed_prefixes") {
		validPrefixes[p] = true
	}

	if !validPrefixes[prefix] {
		allowed := make([]string, 0, len(validPrefixes))
		for p := range validPrefixes {
			allowed = append(allowed, p)
		}
		sort.Strings(allowed)
		return fmt.Errorf("custom ID prefix %q is not allowed; valid prefixes: %s", prefix, strings.Join(allowed, ", "))
	}

	return nil
}

func validDependencyTypeList() string {
	types := make([]string, 0, len(issuestorage.ValidDependencyTypes))
	for depType := range issuestorage.ValidDependencyTypes {
		types = append(types, string(depType))
	}
	sort.Strings(types)
	return strings.Join(types, ", ")
}
