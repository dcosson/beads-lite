package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// newCommentCmd creates the comment command with subcommands.
func newCommentCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage issue comments",
		Long:  `Add and list comments on issues.`,
	}

	cmd.AddCommand(newCommentAddCmd(provider))
	cmd.AddCommand(newCommentListCmd(provider))

	return cmd
}

// newCommentAddCmd creates the comment add subcommand.
func newCommentAddCmd(provider *AppProvider) *cobra.Command {
	var author string

	cmd := &cobra.Command{
		Use:   "add <issue-id> <message>",
		Short: "Add a comment to an issue",
		Long: `Add a comment to an issue.

The message can be provided as the second argument, or use - to read from stdin.

Examples:
  bd comment add bd-a1b2 "This is a comment"
  bd comment add bd-a1b2 -          # read comment from stdin
  echo "Comment text" | bd comment add bd-a1b2 -`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]
			message := args[1]

			// Handle reading from stdin if "-"
			if message == "-" {
				data, err := io.ReadAll(bufio.NewReader(os.Stdin))
				if err != nil {
					return fmt.Errorf("reading comment from stdin: %w", err)
				}
				message = strings.TrimSpace(string(data))
			}

			if message == "" {
				return fmt.Errorf("comment message cannot be empty")
			}

			// Create the comment
			comment := &storage.Comment{
				Author:    author,
				Body:      message,
				CreatedAt: time.Now(),
			}

			if err := app.Storage.AddComment(ctx, issueID, comment); err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("issue %s not found", issueID)
				}
				return fmt.Errorf("adding comment: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]interface{}{
					"issue_id": issueID,
					"comment":  comment,
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Added comment to %s\n", issueID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&author, "author", "a", "", "Comment author")

	return cmd
}

// newCommentListCmd creates the comment list subcommand.
func newCommentListCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <issue-id>",
		Short: "List comments on an issue",
		Long: `List all comments on an issue.

Examples:
  bd comment list bd-a1b2
  bd comment list bd-a1b2 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			// Get the issue to retrieve its comments
			issue, err := app.Storage.Get(ctx, issueID)
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("issue %s not found", issueID)
				}
				return fmt.Errorf("getting issue: %w", err)
			}

			// Output the result
			if app.JSON {
				return json.NewEncoder(app.Out).Encode(issue.Comments)
			}

			if len(issue.Comments) == 0 {
				fmt.Fprintln(app.Out, "No comments")
				return nil
			}

			for _, c := range issue.Comments {
				// Format: [timestamp] author: body
				timestamp := c.CreatedAt.Format("2006-01-02 15:04")
				if c.Author != "" {
					fmt.Fprintf(app.Out, "[%s] %s: %s\n", timestamp, c.Author, c.Body)
				} else {
					fmt.Fprintf(app.Out, "[%s] %s\n", timestamp, c.Body)
				}
			}

			return nil
		},
	}

	return cmd
}
