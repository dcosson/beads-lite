package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCommentCmd creates the deprecated 'comment' command.
// It is an alias for 'comments' that prints a deprecation warning.
func newCommentCmd(provider *AppProvider) *cobra.Command {
	cmd := newCommentsCmd(provider)

	cmd.Use = "comment <issue-id>"
	cmd.Short = "List comments on an issue (deprecated: use 'comments')"
	cmd.Long = `Deprecated: use 'comments' instead.

List or add comments on issues.

When called with an issue ID, lists all comments on that issue.
Use the 'add' subcommand to add a new comment.`

	// Wrap RunE to print deprecation warning
	origRunE := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		printCommentDeprecationWarning(provider)
		return origRunE(c, args)
	}

	// Wrap subcommand RunEs to also print deprecation warning
	for _, sub := range cmd.Commands() {
		origSubRunE := sub.RunE
		sub.RunE = func(c *cobra.Command, args []string) error {
			printCommentDeprecationWarning(provider)
			return origSubRunE(c, args)
		}
	}

	return cmd
}

func printCommentDeprecationWarning(provider *AppProvider) {
	fmt.Fprintln(provider.Err, `Warning: "comment" is deprecated, use "comments" instead`)
}
