package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newListCmd creates the list command.
func newListCmd(provider *AppProvider) *cobra.Command {
	var (
		status    string
		priority  string
		issueType string
		molType   string
		labels    []string
		parent    string
		assignee  string
		all       bool
		closed    bool
		roots     bool
		format    string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues with filtering",
		Long: `List issues with various filters.

By default, lists open issues (up to 50). Use flags to filter by status,
type, priority, labels, parent, or assignee. Use --limit to change the
maximum number of results, or --limit 0 / --all to return all results.

Examples:
  bd list                      # List open issues (up to 50)
  bd list --limit 0            # List all open issues (no limit)
  bd list --all                # List all issues (open and closed)
  bd list --closed             # List only closed issues
  bd list --status=in-progress # List in-progress issues
  bd list --type=bug           # List bugs
  bd list --priority=high      # List high priority issues
  bd list --label=urgent,v2    # List issues with both labels
  bd list --parent=be-abc      # List children of issue be-abc
  bd list --roots              # List root issues (no parent)
  bd list --assignee=alice     # List issues assigned to alice`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Build filter
			filter := &issuestorage.ListFilter{}

			// Status handling
			if all {
				// No status filter - list everything
				filter.Status = nil
			} else if closed {
				s := issuestorage.StatusClosed
				filter.Status = &s
			} else if status != "" {
				// Allow tombstone as a valid filter for list (even though
				// update rejects it — listing tombstones is an admin query)
				if strings.ToLower(status) == "all" {
					// --status=all behaves like --all: list open + closed, skip deleted
					filter.Status = nil
					all = true
				} else if strings.ToLower(status) == "tombstone" {
					s := issuestorage.StatusTombstone
					filter.Status = &s
				} else {
					s, err := parseStatus(status, getCustomValues(app, "status.custom"))
					if err != nil {
						return err
					}
					filter.Status = &s
				}
			} else {
				// Default: list open issues
				s := issuestorage.StatusOpen
				filter.Status = &s
			}

			if priority != "" {
				p := issuestorage.Priority(priority)
				filter.Priority = &p
			}

			if issueType != "" {
				t := issuestorage.IssueType(issueType)
				filter.Type = &t
			}

			if molType != "" {
				if !issuestorage.ValidateMolType(molType) {
					return fmt.Errorf("invalid mol-type %q: must be one of swarm, patrol, work", molType)
				}
				mt := issuestorage.MolType(molType)
				filter.MolType = &mt
			}

			if len(labels) > 0 {
				filter.Labels = labels
			}

			if assignee != "" {
				filter.Assignee = &assignee
			}

			if roots {
				// Empty string signals "root only" (no parent)
				empty := ""
				filter.Parent = &empty
			} else if parent != "" {
				filter.Parent = &parent
			}

			// Get open issues with filter
			issues, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("listing issues: %w", err)
			}

			// If --all flag, we need to also get closed issues
			if all && filter.Status == nil {
				// Get closed issues with same filters
				closedFilter := *filter
				closedStatus := issuestorage.StatusClosed
				closedFilter.Status = &closedStatus
				closedIssues, err := app.Storage.List(ctx, &closedFilter)
				if err != nil {
					return fmt.Errorf("listing closed issues: %w", err)
				}
				issues = append(issues, closedIssues...)
			}

			// Apply limit
			limited := limit > 0 && len(issues) > limit
			if limited {
				issues = issues[:limit]
			}

			// JSON output
			if app.JSON {
				result := make([]IssueListJSON, len(issues))
				for i, issue := range issues {
					result[i] = ToIssueListJSON(issue)
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output (--format is accepted but not implemented, matching original beads)
			if len(issues) == 0 {
				fmt.Fprintln(app.Out, "No issues found.")
				return nil
			}

			for _, issue := range issues {
				icon, iconColor := statusIcon(issue.Status)
				coloredIcon := app.Colorize(icon, iconColor)

				priDisplay := issue.Priority.Display()
				priColor := priorityColor(issue.Priority)
				var priBracket string
				if issue.Status == issuestorage.StatusClosed {
					priBracket = "[" + app.Colorize(priDisplay, priColor) + "]"
				} else {
					priBracket = "[" + app.Colorize("● "+priDisplay, priColor) + "]"
				}

				typeStr := string(issue.Type)
				typeColor := issueTypeColor(issue.Type)
				typeBracket := "[" + app.Colorize(typeStr, typeColor) + "]"

				assigneeStr := ""
				if issue.Assignee != "" {
					assigneeStr = " @" + issue.Assignee
				}

				fmt.Fprintf(app.Out, "%s %s %s %s%s - %s\n",
					coloredIcon, issue.ID, priBracket, typeBracket, assigneeStr, issue.Title)
			}

			if limited {
				fmt.Fprintf(app.Out, "\nShowing %d issues (use --limit 0 for all)\n", limit)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status ("+statusNames(nil)+")")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority (critical, high, medium, low)")
	cmd.Flags().StringVarP(&issueType, "type", "t", "", "Filter by type (task, bug, feature, epic, chore)")
	cmd.Flags().StringVar(&molType, "mol-type", "", "Filter by molecule type (swarm, patrol, work)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Filter by labels (comma-separated or repeated, must have all)")
	cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent issue ID")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	cmd.Flags().BoolVar(&all, "all", false, "List all issues (open and closed)")
	cmd.Flags().BoolVar(&closed, "closed", false, "List only closed issues")
	cmd.Flags().BoolVar(&roots, "roots", false, "List only root issues (no parent)")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Output format (not implemented, accepts any value)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of issues to return (0 for all)")

	return cmd
}

// statusIcon returns the icon and ANSI color code for a status.
func statusIcon(s issuestorage.Status) (icon string, colorCode string) {
	switch s {
	case issuestorage.StatusOpen:
		return "○", ""
	case issuestorage.StatusInProgress:
		return "◐", "38;5;214"
	case issuestorage.StatusClosed:
		return "✓", "90"
	case issuestorage.StatusBlocked:
		return "✕", "31"
	case issuestorage.StatusDeferred:
		return "?", "90"
	default:
		return "?", ""
	}
}

// priorityColor returns the ANSI color code for a priority level.
func priorityColor(p issuestorage.Priority) string {
	switch p {
	case issuestorage.PriorityCritical:
		return "31"
	case issuestorage.PriorityHigh:
		return "38;5;208"
	case issuestorage.PriorityMedium:
		return "38;5;214"
	case issuestorage.PriorityLow:
		return "33"
	case issuestorage.PriorityBacklog:
		return ""
	default:
		return ""
	}
}

// issueTypeColor returns the ANSI color code for an issue type.
func issueTypeColor(t issuestorage.IssueType) string {
	switch t {
	case issuestorage.TypeEpic:
		return "35"
	case issuestorage.TypeBug:
		return "31"
	case issuestorage.TypeFeature:
		return "32"
	default:
		return ""
	}
}
