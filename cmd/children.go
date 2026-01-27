// Package cmd implements the CLI commands for beads.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"beads2/storage"

	"github.com/spf13/cobra"
)

var (
	childrenTree bool
)

var childrenCmd = &cobra.Command{
	Use:   "children <parent-id>",
	Short: "List children of an issue",
	Long:  `List the immediate children of an issue, or show the full subtree with --tree.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := GetApp()
		parentID := args[0]

		ctx := context.Background()

		// Get the parent issue to access its children
		parent, err := app.Storage.Get(ctx, parentID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("issue not found: %s", parentID)
			}
			return err
		}

		if childrenTree {
			return printTree(ctx, app, parent, 0)
		}

		return printChildren(ctx, app, parent)
	},
}

// printChildren prints the immediate children of an issue.
func printChildren(ctx context.Context, app *App, parent *storage.Issue) error {
	if len(parent.Children) == 0 {
		if app.JSON {
			return json.NewEncoder(app.Out).Encode([]interface{}{})
		}
		fmt.Fprintf(app.Out, "No children for %s\n", parent.ID)
		return nil
	}

	var children []*storage.Issue
	for _, childID := range parent.Children {
		child, err := app.Storage.Get(ctx, childID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// Skip missing children (data inconsistency)
				continue
			}
			return err
		}
		children = append(children, child)
	}

	if app.JSON {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(children)
	}

	for _, child := range children {
		fmt.Fprintf(app.Out, "%s %s [%s]\n", statusSymbol(child.Status), child.ID, child.Title)
	}
	return nil
}

// printTree recursively prints the full subtree of an issue.
func printTree(ctx context.Context, app *App, issue *storage.Issue, depth int) error {
	if app.JSON && depth == 0 {
		// For JSON output, build the full tree structure
		tree, err := buildTree(ctx, app, issue)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(tree)
	}

	if !app.JSON {
		// Print the current issue with indentation
		indent := ""
		for i := 0; i < depth; i++ {
			indent += "  "
		}
		prefix := ""
		if depth > 0 {
			prefix = "└─ "
		}
		fmt.Fprintf(app.Out, "%s%s%s %s [%s]\n", indent, prefix, statusSymbol(issue.Status), issue.ID, issue.Title)
	}

	// Recursively print children
	for _, childID := range issue.Children {
		child, err := app.Storage.Get(ctx, childID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				continue
			}
			return err
		}
		if err := printTree(ctx, app, child, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// TreeNode represents an issue and its children for JSON output.
type TreeNode struct {
	ID       string      `json:"id"`
	Title    string      `json:"title"`
	Status   string      `json:"status"`
	Children []*TreeNode `json:"children,omitempty"`
}

// buildTree builds a nested tree structure for JSON output.
func buildTree(ctx context.Context, app *App, issue *storage.Issue) (*TreeNode, error) {
	node := &TreeNode{
		ID:     issue.ID,
		Title:  issue.Title,
		Status: string(issue.Status),
	}

	for _, childID := range issue.Children {
		child, err := app.Storage.Get(ctx, childID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				continue
			}
			return nil, err
		}
		childNode, err := buildTree(ctx, app, child)
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, childNode)
	}

	return node, nil
}

// statusSymbol returns a symbol for the issue status.
func statusSymbol(status storage.Status) string {
	switch status {
	case storage.StatusOpen:
		return "○"
	case storage.StatusInProgress:
		return "◐"
	case storage.StatusBlocked:
		return "●"
	case storage.StatusDeferred:
		return "◇"
	case storage.StatusClosed:
		return "✓"
	default:
		return "?"
	}
}

func init() {
	childrenCmd.Flags().BoolVar(&childrenTree, "tree", false, "Show full subtree")
	rootCmd.AddCommand(childrenCmd)
}
