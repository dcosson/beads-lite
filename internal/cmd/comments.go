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

// newCommentsCmd creates the comments command.
// `bd comments <issue-id>` lists comments (default behavior).
// `bd comments add <issue-id> <message>` adds a comment.
func newCommentsCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments <issue-id>",
		Short: "List comments on an issue",
		Long: `List or add comments on issues.

When called with an issue ID, lists all comments on that issue.
Use the 'add' subcommand to add a new comment.

Examples:
  bd comments bd-a1b2
  bd comments bd-a1b2 --json`,
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

			issue, err := store.Get(ctx, issueID)
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("issue %s not found", issueID)
				}
				return fmt.Errorf("getting issue: %w", err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(issue.Comments)
			}

			if len(issue.Comments) == 0 {
				fmt.Fprintln(app.Out, "No comments")
				return nil
			}

			for _, c := range issue.Comments {
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

	cmd.AddCommand(newCommentsAddCmd(provider))

	return cmd
}

// newCommentsAddCmd creates the comments add subcommand.
func newCommentsAddCmd(provider *AppProvider) *cobra.Command {
	var author string
	var file string

	cmd := &cobra.Command{
		Use:   "add <issue-id> [message]",
		Short: "Add a comment to an issue",
		Long: `Add a comment to an issue.

The message can be provided as the second argument, read from a file with -f,
or read from stdin using - as the message.

Examples:
  bd comments add bd-a1b2 "This is a comment"
  bd comments add bd-a1b2 -f notes.txt
  bd comments add bd-a1b2 -             # read from stdin
  echo "text" | bd comments add bd-a1b2 -`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			var message string

			if file != "" {
				// Read from file
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading comment from file: %w", err)
				}
				message = strings.TrimSpace(string(data))
			} else if len(args) < 2 {
				return fmt.Errorf("comment message required: provide as argument or use -f flag")
			} else if args[1] == "-" {
				// Read from stdin
				data, err := io.ReadAll(bufio.NewReader(os.Stdin))
				if err != nil {
					return fmt.Errorf("reading comment from stdin: %w", err)
				}
				message = strings.TrimSpace(string(data))
			} else {
				message = args[1]
			}

			if message == "" {
				return fmt.Errorf("comment message cannot be empty")
			}

			comment := &storage.Comment{
				Author:    author,
				Body:      message,
				CreatedAt: time.Now(),
			}

			commentStore, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			if err := commentStore.AddComment(ctx, issueID, comment); err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("issue %s not found", issueID)
				}
				return fmt.Errorf("adding comment: %w", err)
			}

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
	cmd.Flags().StringVarP(&file, "file", "f", "", "Read comment from file")

	return cmd
}
