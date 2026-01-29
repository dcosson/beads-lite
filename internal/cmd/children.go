package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"beads-lite/internal/storage"

	"github.com/spf13/cobra"
)

// ChildInfo represents a child issue with optional subtree info.
type ChildInfo struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Status   string       `json:"status"`
	Children []*ChildInfo `json:"children,omitempty"`
}

// newChildrenCmd creates the children command.
func newChildrenCmd(provider *AppProvider) *cobra.Command {
	var tree bool

	cmd := &cobra.Command{
		Use:   "children <issue-id>",
		Short: "List an issue's children",
		Long: `List the children of an issue.

By default, lists only direct children. Use --tree to show the full subtree.

Supports prefix matching on issue IDs. If the prefix matches exactly one issue,
that issue's children are displayed.

Examples:
  bd children bd-a1b2       # List direct children
  bd children bd-a1         # Prefix match (if unique)
  bd children bd-a1b2 --tree # Show full subtree`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := args[0]

			// Try exact match first
			issue, err := app.Storage.Get(ctx, query)
			if err == storage.ErrNotFound {
				// Try prefix matching
				issue, err = findByPrefix(app.Storage, ctx, query)
			}
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("no issue found matching %q", query)
				}
				return err
			}

			if len(issue.Children()) == 0 {
				if app.JSON {
					return json.NewEncoder(app.Out).Encode([]IssueListJSON{})
				}
				fmt.Fprintf(app.Out, "No children for %s\n", issue.ID)
				return nil
			}

			if tree {
				return outputChildrenTree(ctx, app, issue)
			}
			return outputChildrenList(ctx, app, issue)
		},
	}

	cmd.Flags().BoolVarP(&tree, "tree", "t", false, "Show full subtree recursively")

	return cmd
}

// outputChildrenList outputs direct children as a simple list.
func outputChildrenList(ctx context.Context, app *App, issue *storage.Issue) error {
	var children []*storage.Issue
	for _, childID := range issue.Children() {
		child, err := app.Storage.Get(ctx, childID)
		if err != nil {
			continue // Skip inaccessible children
		}
		children = append(children, child)
	}

	if app.JSON {
		// Use IssueListJSON format to match original beads
		result := make([]IssueListJSON, len(children))
		for i, child := range children {
			result[i] = ToIssueListJSON(child)
		}
		return json.NewEncoder(app.Out).Encode(result)
	}

	fmt.Fprintf(app.Out, "Children of %s (%d):\n\n", issue.ID, len(children))
	for _, child := range children {
		fmt.Fprintf(app.Out, "  %s  %s  [%s]\n", child.ID, child.Title, child.Status)
	}

	return nil
}

// outputChildrenTree outputs the full subtree recursively.
func outputChildrenTree(ctx context.Context, app *App, issue *storage.Issue) error {
	tree := buildTree(ctx, app, issue)

	if app.JSON {
		return json.NewEncoder(app.Out).Encode(tree)
	}

	fmt.Fprintf(app.Out, "Subtree of %s:\n\n", issue.ID)
	printTree(app, tree, "")

	return nil
}

// buildTree builds a tree of ChildInfo for an issue.
func buildTree(ctx context.Context, app *App, issue *storage.Issue) []*ChildInfo {
	var children []*ChildInfo

	for _, childID := range issue.Children() {
		child, err := app.Storage.Get(ctx, childID)
		if err != nil {
			children = append(children, &ChildInfo{
				ID:     childID,
				Title:  "(not found)",
				Status: "unknown",
			})
			continue
		}

		info := &ChildInfo{
			ID:     child.ID,
			Title:  child.Title,
			Status: string(child.Status),
		}

		// Recursively build subtree
		if len(child.Children()) > 0 {
			info.Children = buildTree(ctx, app, child)
		}

		children = append(children, info)
	}

	return children
}

// printTree prints the tree with indentation.
func printTree(app *App, children []*ChildInfo, prefix string) {
	for i, child := range children {
		isLast := i == len(children)-1

		// Determine the connector
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		fmt.Fprintf(app.Out, "%s%s%s  %s  [%s]\n", prefix, connector, child.ID, child.Title, child.Status)

		// Recurse for children
		if len(child.Children) > 0 {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			printTree(app, child.Children, newPrefix)
		}
	}
}
