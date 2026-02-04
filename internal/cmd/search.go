package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/issuestorage"
	"github.com/spf13/cobra"
)

// newSearchCmd creates the search command.
func newSearchCmd(provider *AppProvider) *cobra.Command {
	var (
		titleOnly bool
		status    string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search issue titles and descriptions",
		Long: `Search for issues matching the given query.

By default, searches both open and closed issues in title and description.
Use --status to filter by a specific status.
Use --title-only to search only in titles.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := strings.ToLower(args[0])

			var matches []*issuestorage.Issue
			var issues []*issuestorage.Issue

			if status != "" {
				s, err := parseStatus(status, getCustomValues(app, "status.custom"))
				if err != nil {
					return err
				}
				filter := &issuestorage.ListFilter{Status: &s}
				issues, err = app.Storage.List(ctx, filter)
				if err != nil {
					return fmt.Errorf("listing issues: %w", err)
				}
			} else {
				openIssues, err := app.Storage.List(ctx, nil)
				if err != nil {
					return fmt.Errorf("listing open issues: %w", err)
				}
				issues = append(issues, openIssues...)

				closedStatus := issuestorage.StatusClosed
				closedIssues, err := app.Storage.List(ctx, &issuestorage.ListFilter{Status: &closedStatus})
				if err != nil {
					return fmt.Errorf("listing closed issues: %w", err)
				}
				issues = append(issues, closedIssues...)
			}

			for _, issue := range issues {
				if matchesQuery(issue, query, titleOnly) {
					matches = append(matches, issue)
				}
			}

			if app.JSON {
				results := make([]IssueListJSON, len(matches))
				for i, issue := range matches {
					results[i] = ToIssueListJSON(issue)
				}
				return json.NewEncoder(app.Out).Encode(results)
			}

			if len(matches) == 0 {
				fmt.Fprintln(app.Out, "No matches found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Found %d matches:\n", len(matches))
			for _, issue := range matches {
				statusStr := ""
				if issue.Status != issuestorage.StatusOpen {
					statusStr = fmt.Sprintf(" [%s]", issue.Status)
				}
				fmt.Fprintf(app.Out, "  %s  %s%s\n", issue.ID, issue.Title, statusStr)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status ("+statusNames(nil)+")")
	cmd.Flags().BoolVar(&titleOnly, "title-only", false, "Only search titles")

	return cmd
}

func matchesQuery(issue *issuestorage.Issue, query string, titleOnly bool) bool {
	if strings.Contains(strings.ToLower(issue.Title), query) {
		return true
	}
	if !titleOnly && strings.Contains(strings.ToLower(issue.Description), query) {
		return true
	}
	return false
}
