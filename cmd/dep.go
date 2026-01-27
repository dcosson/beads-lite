// Package cmd implements the CLI commands for beads.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"beads2/storage"

	"github.com/spf13/cobra"
)

// depCmd is the parent command for dependency operations.
var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage issue dependencies",
	Long:  `Add, remove, and list dependencies between issues.`,
}

// depAddCmd adds a dependency relationship.
var depAddCmd = &cobra.Command{
	Use:   "add <from> <to>",
	Short: "Add a dependency (from depends on to)",
	Long: `Add a dependency relationship where the first issue depends on the second.

Example:
  bd dep add bd-a1b2 bd-c3d4   # a1b2 depends on c3d4`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := GetApp()
		ctx := cmd.Context()

		dependentID := args[0]
		dependencyID := args[1]

		if err := app.Storage.AddDependency(ctx, dependentID, dependencyID); err != nil {
			if err == storage.ErrNotFound {
				return fmt.Errorf("issue not found")
			}
			if err == storage.ErrCycle {
				return fmt.Errorf("cannot add dependency: would create a cycle")
			}
			return err
		}

		if app.JSON {
			return json.NewEncoder(app.Out).Encode(map[string]string{
				"status":     "ok",
				"dependent":  dependentID,
				"dependency": dependencyID,
			})
		}

		fmt.Fprintf(app.Out, "%s now depends on %s\n", dependentID, dependencyID)
		return nil
	},
}

// depRemoveCmd removes a dependency relationship.
var depRemoveCmd = &cobra.Command{
	Use:   "remove <from> <to>",
	Short: "Remove a dependency",
	Long: `Remove a dependency relationship between two issues.

Example:
  bd dep remove bd-a1b2 bd-c3d4`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := GetApp()
		ctx := cmd.Context()

		dependentID := args[0]
		dependencyID := args[1]

		if err := app.Storage.RemoveDependency(ctx, dependentID, dependencyID); err != nil {
			if err == storage.ErrNotFound {
				return fmt.Errorf("issue not found")
			}
			return err
		}

		if app.JSON {
			return json.NewEncoder(app.Out).Encode(map[string]string{
				"status":     "ok",
				"dependent":  dependentID,
				"dependency": dependencyID,
			})
		}

		fmt.Fprintf(app.Out, "Removed dependency: %s no longer depends on %s\n", dependentID, dependencyID)
		return nil
	},
}

// depListCmd lists an issue's dependencies.
var depListCmd = &cobra.Command{
	Use:   "list <id>",
	Short: "List an issue's dependencies",
	Long: `Show what an issue depends on and what it blocks.

Example:
  bd dep list bd-a1b2           # show depends_on and blocks
  bd dep list bd-a1b2 --tree    # show full dependency tree`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := GetApp()
		ctx := cmd.Context()

		issueID := args[0]
		showTree, _ := cmd.Flags().GetBool("tree")

		issue, err := app.Storage.Get(ctx, issueID)
		if err != nil {
			if err == storage.ErrNotFound {
				return fmt.Errorf("issue not found: %s", issueID)
			}
			return err
		}

		if showTree {
			return printDependencyTree(ctx, app, issue)
		}

		return printDependencyList(ctx, app, issue)
	},
}

// DepInfo holds dependency information for JSON output.
type DepInfo struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	DependsOn []string `json:"depends_on,omitempty"`
	Blocks    []string `json:"blocks,omitempty"`
}

// DepTreeNode represents a node in the dependency tree for JSON output.
type DepTreeNode struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Status   storage.Status `json:"status"`
	Children []*DepTreeNode `json:"children,omitempty"`
}

func printDependencyList(ctx context.Context, app *App, issue *storage.Issue) error {
	if app.JSON {
		info := DepInfo{
			ID:        issue.ID,
			Title:     issue.Title,
			DependsOn: issue.DependsOn,
			Blocks:    issue.Blocks,
		}
		return json.NewEncoder(app.Out).Encode(info)
	}

	fmt.Fprintf(app.Out, "%s: %s\n\n", issue.ID, issue.Title)

	if len(issue.DependsOn) > 0 {
		fmt.Fprintln(app.Out, "Depends on:")
		for _, depID := range issue.DependsOn {
			dep, err := app.Storage.Get(ctx, depID)
			if err != nil {
				fmt.Fprintf(app.Out, "  %s (not found)\n", depID)
				continue
			}
			statusMark := statusIcon(dep.Status)
			fmt.Fprintf(app.Out, "  %s %s: %s\n", statusMark, dep.ID, dep.Title)
		}
	} else {
		fmt.Fprintln(app.Out, "Depends on: (none)")
	}

	fmt.Fprintln(app.Out)

	if len(issue.Blocks) > 0 {
		fmt.Fprintln(app.Out, "Blocks:")
		for _, blockID := range issue.Blocks {
			blocked, err := app.Storage.Get(ctx, blockID)
			if err != nil {
				fmt.Fprintf(app.Out, "  %s (not found)\n", blockID)
				continue
			}
			statusMark := statusIcon(blocked.Status)
			fmt.Fprintf(app.Out, "  %s %s: %s\n", statusMark, blocked.ID, blocked.Title)
		}
	} else {
		fmt.Fprintln(app.Out, "Blocks: (none)")
	}

	return nil
}

func printDependencyTree(ctx context.Context, app *App, issue *storage.Issue) error {
	if app.JSON {
		tree, err := buildDependencyTree(ctx, app, issue.ID, make(map[string]bool))
		if err != nil {
			return err
		}
		return json.NewEncoder(app.Out).Encode(tree)
	}

	fmt.Fprintf(app.Out, "%s: %s\n\n", issue.ID, issue.Title)
	fmt.Fprintln(app.Out, "Dependency Tree (what this issue depends on):")
	if len(issue.DependsOn) == 0 {
		fmt.Fprintln(app.Out, "  (no dependencies)")
	} else {
		printTreeRecursive(ctx, app, issue.DependsOn, "", make(map[string]bool), "depends_on")
	}

	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Blocking Tree (what depends on this issue):")
	if len(issue.Dependents) == 0 {
		fmt.Fprintln(app.Out, "  (no dependents)")
	} else {
		printTreeRecursive(ctx, app, issue.Dependents, "", make(map[string]bool), "dependents")
	}

	return nil
}

func buildDependencyTree(ctx context.Context, app *App, issueID string, visited map[string]bool) (*DepTreeNode, error) {
	if visited[issueID] {
		return &DepTreeNode{ID: issueID, Title: "(cycle)"}, nil
	}
	visited[issueID] = true

	issue, err := app.Storage.Get(ctx, issueID)
	if err != nil {
		return &DepTreeNode{ID: issueID, Title: "(not found)"}, nil
	}

	node := &DepTreeNode{
		ID:     issue.ID,
		Title:  issue.Title,
		Status: issue.Status,
	}

	for _, depID := range issue.DependsOn {
		childVisited := make(map[string]bool)
		for k, v := range visited {
			childVisited[k] = v
		}
		child, err := buildDependencyTree(ctx, app, depID, childVisited)
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, child)
	}

	return node, nil
}

func printTreeRecursive(ctx context.Context, app *App, ids []string, prefix string, visited map[string]bool, direction string) {
	for i, id := range ids {
		isLast := i == len(ids)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		if visited[id] {
			fmt.Fprintf(app.Out, "%s%s%s (cycle)\n", prefix, connector, id)
			continue
		}
		visited[id] = true

		issue, err := app.Storage.Get(ctx, id)
		if err != nil {
			fmt.Fprintf(app.Out, "%s%s%s (not found)\n", prefix, connector, id)
			continue
		}

		statusMark := statusIcon(issue.Status)
		fmt.Fprintf(app.Out, "%s%s%s %s: %s\n", prefix, connector, statusMark, issue.ID, issue.Title)

		// Get the children based on direction
		var children []string
		if direction == "depends_on" {
			children = issue.DependsOn
		} else {
			children = issue.Dependents
		}

		if len(children) > 0 {
			childVisited := make(map[string]bool)
			for k, v := range visited {
				childVisited[k] = v
			}
			printTreeRecursive(ctx, app, children, childPrefix, childVisited, direction)
		}
	}
}

func statusIcon(status storage.Status) string {
	switch status {
	case storage.StatusClosed:
		return "[x]"
	case storage.StatusInProgress:
		return "[~]"
	case storage.StatusBlocked:
		return "[!]"
	case storage.StatusDeferred:
		return "[-]"
	default:
		return "[ ]"
	}
}

func init() {
	depListCmd.Flags().Bool("tree", false, "Show full dependency tree")

	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
	depCmd.AddCommand(depListCmd)

	rootCmd.AddCommand(depCmd)
}
