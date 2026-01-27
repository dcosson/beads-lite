package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads2/internal/storage"
	"github.com/spf13/cobra"
)

// SearchResult represents a single search result.
type SearchResult struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// newSearchCmd creates the search command.
func newSearchCmd(provider *AppProvider) *cobra.Command {
	var (
		all       bool
		titleOnly bool
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search issue titles and descriptions",
		Long: `Search for issues matching the given query.

By default, searches only open issues in both title and description.
Use --all to include closed issues.
Use --title-only to search only in titles.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := strings.ToLower(args[0])

			var matches []*storage.Issue

			// Get open issues
			openIssues, err := app.Storage.List(ctx, nil)
			if err != nil {
				return fmt.Errorf("listing open issues: %w", err)
			}

			for _, issue := range openIssues {
				if matchesQuery(issue, query, titleOnly) {
					matches = append(matches, issue)
				}
			}

			// Get closed issues if --all flag is set
			if all {
				closedStatus := storage.StatusClosed
				closedIssues, err := app.Storage.List(ctx, &storage.ListFilter{Status: &closedStatus})
				if err != nil {
					return fmt.Errorf("listing closed issues: %w", err)
				}

				for _, issue := range closedIssues {
					if matchesQuery(issue, query, titleOnly) {
						matches = append(matches, issue)
					}
				}
			}

			if app.JSON {
				results := make([]SearchResult, len(matches))
				for i, issue := range matches {
					results[i] = SearchResult{
						ID:     issue.ID,
						Title:  issue.Title,
						Status: string(issue.Status),
					}
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
				if issue.Status != storage.StatusOpen {
					statusStr = fmt.Sprintf(" [%s]", issue.Status)
				}
				fmt.Fprintf(app.Out, "  %s  %s%s\n", issue.ID, issue.Title, statusStr)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Include closed issues")
	cmd.Flags().BoolVar(&titleOnly, "title-only", false, "Only search titles")

	return cmd
}

func matchesQuery(issue *storage.Issue, query string, titleOnly bool) bool {
	if strings.Contains(strings.ToLower(issue.Title), query) {
		return true
	}
	if !titleOnly && strings.Contains(strings.ToLower(issue.Description), query) {
		return true
	}
	return false
}
