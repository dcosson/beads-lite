package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"beads2/internal/storage"

	"github.com/spf13/cobra"
)

// newDepCmd creates the dep command with subcommands.
func newDepCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage issue dependencies",
		Long: `Manage dependencies between issues.

Dependencies represent "A needs B to be done first" relationships.
When A depends on B, B must be completed before A can start.

Subcommands:
  add     Create a dependency (A depends on B)
  remove  Remove a dependency
  list    Show dependencies for an issue`,
	}

	cmd.AddCommand(newDepAddCmd(provider))
	cmd.AddCommand(newDepRemoveCmd(provider))
	cmd.AddCommand(newDepListCmd(provider))

	return cmd
}

// newDepAddCmd creates the "dep add" subcommand.
func newDepAddCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <issue-id> <dependency-id>",
		Short: "Add a dependency (issue depends on dependency)",
		Long: `Create a dependency relationship where issue depends on dependency.

This means the dependency must be completed before issue can start.
Both issues are updated: issue.depends_on gets dependency added,
and dependency.dependents gets issue added.

Cycle detection prevents circular dependencies.

Examples:
  bd dep add bd-a1b2 bd-c3d4   # bd-a1b2 depends on bd-c3d4`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]
			dependencyID := args[1]

			// Resolve IDs (support prefix matching)
			issue, err := resolveIssue(app.Storage, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			dependency, err := resolveIssue(app.Storage, ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("resolving dependency %s: %w", dependencyID, err)
			}

			// Add the dependency
			if err := app.Storage.AddDependency(ctx, issue.ID, dependency.ID); err != nil {
				if err == storage.ErrCycle {
					return fmt.Errorf("cannot add dependency: would create a cycle")
				}
				return fmt.Errorf("adding dependency: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{
					"issue":      issue.ID,
					"dependency": dependency.ID,
					"status":     "added",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Added dependency: %s depends on %s\n", issue.ID, dependency.ID)
			return nil
		},
	}

	return cmd
}

// newDepRemoveCmd creates the "dep remove" subcommand.
func newDepRemoveCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <issue-id> <dependency-id>",
		Short: "Remove a dependency",
		Long: `Remove a dependency relationship between two issues.

Both issues are updated: dependency is removed from issue.depends_on,
and issue is removed from dependency.dependents.

Examples:
  bd dep remove bd-a1b2 bd-c3d4   # Remove bd-a1b2's dependency on bd-c3d4`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]
			dependencyID := args[1]

			// Resolve IDs (support prefix matching)
			issue, err := resolveIssue(app.Storage, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			dependency, err := resolveIssue(app.Storage, ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("resolving dependency %s: %w", dependencyID, err)
			}

			// Remove the dependency
			if err := app.Storage.RemoveDependency(ctx, issue.ID, dependency.ID); err != nil {
				return fmt.Errorf("removing dependency: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{
					"issue":      issue.ID,
					"dependency": dependency.ID,
					"status":     "removed",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Removed dependency: %s no longer depends on %s\n", issue.ID, dependency.ID)
			return nil
		},
	}

	return cmd
}

// newDepListCmd creates the "dep list" subcommand.
func newDepListCmd(provider *AppProvider) *cobra.Command {
	var tree bool

	cmd := &cobra.Command{
		Use:   "list <issue-id>",
		Short: "List dependencies for an issue",
		Long: `Show all dependencies for an issue.

By default, shows direct dependencies (what this issue depends on)
and dependents (what depends on this issue).

Use --tree to show a tree view of transitive dependencies.

Examples:
  bd dep list bd-a1b2          # Show direct dependencies
  bd dep list bd-a1b2 --tree   # Show dependency tree`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			// Resolve ID (support prefix matching)
			issue, err := resolveIssue(app.Storage, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			if app.JSON {
				return outputDepListJSON(app, ctx, issue, tree)
			}

			return outputDepListText(app, ctx, issue, tree)
		},
	}

	cmd.Flags().BoolVar(&tree, "tree", false, "Show dependency tree (transitive dependencies)")

	return cmd
}

// outputDepListJSON outputs dependency list in JSON format.
func outputDepListJSON(app *App, ctx context.Context, issue *storage.Issue, tree bool) error {
	result := map[string]interface{}{
		"id":         issue.ID,
		"title":      issue.Title,
		"depends_on": issue.DependsOn,
		"dependents": issue.Dependents,
	}

	if tree {
		// Build tree structure
		depTree := buildDepTree(app.Storage, ctx, issue, make(map[string]bool))
		result["tree"] = depTree
	}

	return json.NewEncoder(app.Out).Encode(result)
}

// depTreeNode represents a node in the dependency tree for JSON output.
type depTreeNode struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Status   string         `json:"status"`
	Children []*depTreeNode `json:"dependencies,omitempty"`
}

// buildDepTree recursively builds a dependency tree.
func buildDepTree(store storage.Storage, ctx context.Context, issue *storage.Issue, visited map[string]bool) *depTreeNode {
	node := &depTreeNode{
		ID:     issue.ID,
		Title:  issue.Title,
		Status: string(issue.Status),
	}

	// Prevent cycles
	if visited[issue.ID] {
		return node
	}
	visited[issue.ID] = true

	// Recursively add dependencies
	for _, depID := range issue.DependsOn {
		dep, err := store.Get(ctx, depID)
		if err != nil {
			node.Children = append(node.Children, &depTreeNode{
				ID:     depID,
				Title:  "(error loading)",
				Status: "unknown",
			})
			continue
		}
		node.Children = append(node.Children, buildDepTree(store, ctx, dep, visited))
	}

	return node
}

// outputDepListText outputs dependency list in text format.
func outputDepListText(app *App, ctx context.Context, issue *storage.Issue, tree bool) error {
	fmt.Fprintf(app.Out, "%s: %s\n", issue.ID, issue.Title)
	fmt.Fprintln(app.Out, strings.Repeat("-", len(issue.ID)+len(issue.Title)+2))

	if tree {
		return outputDepTree(app, ctx, issue, 0, make(map[string]bool))
	}

	// Direct dependencies
	if len(issue.DependsOn) == 0 {
		fmt.Fprintln(app.Out, "\nDepends On: (none)")
	} else {
		fmt.Fprintln(app.Out, "\nDepends On:")
		for _, depID := range issue.DependsOn {
			dep, err := app.Storage.Get(ctx, depID)
			if err != nil {
				fmt.Fprintf(app.Out, "  → %s (error: %v)\n", depID, err)
			} else {
				status := statusIndicator(dep.Status)
				fmt.Fprintf(app.Out, "  → %s %s: %s\n", status, dep.ID, dep.Title)
			}
		}
	}

	// Dependents (what depends on this)
	if len(issue.Dependents) == 0 {
		fmt.Fprintln(app.Out, "\nDependents: (none)")
	} else {
		fmt.Fprintln(app.Out, "\nDependents (blocked by this):")
		for _, depID := range issue.Dependents {
			dep, err := app.Storage.Get(ctx, depID)
			if err != nil {
				fmt.Fprintf(app.Out, "  ← %s (error: %v)\n", depID, err)
			} else {
				status := statusIndicator(dep.Status)
				fmt.Fprintf(app.Out, "  ← %s %s: %s\n", status, dep.ID, dep.Title)
			}
		}
	}

	return nil
}

// outputDepTree outputs a tree view of dependencies.
func outputDepTree(app *App, ctx context.Context, issue *storage.Issue, depth int, visited map[string]bool) error {
	if depth == 0 {
		fmt.Fprintln(app.Out, "\nDependency Tree:")
	}

	indent := strings.Repeat("  ", depth)
	status := statusIndicator(issue.Status)

	if depth > 0 {
		fmt.Fprintf(app.Out, "%s└─ %s %s: %s\n", indent, status, issue.ID, issue.Title)
	}

	// Prevent cycles
	if visited[issue.ID] {
		if len(issue.DependsOn) > 0 {
			fmt.Fprintf(app.Out, "%s  └─ (cycle detected)\n", indent)
		}
		return nil
	}
	visited[issue.ID] = true

	// Recursively show dependencies
	for _, depID := range issue.DependsOn {
		dep, err := app.Storage.Get(ctx, depID)
		if err != nil {
			fmt.Fprintf(app.Out, "%s  └─ %s (error: %v)\n", indent, depID, err)
			continue
		}
		outputDepTree(app, ctx, dep, depth+1, visited)
	}

	return nil
}

// statusIndicator returns a symbol for the issue status.
func statusIndicator(status storage.Status) string {
	switch status {
	case storage.StatusClosed:
		return "✓"
	case storage.StatusInProgress:
		return "●"
	case storage.StatusBlocked:
		return "✗"
	default:
		return "○"
	}
}

// resolveIssue finds an issue by ID or prefix.
func resolveIssue(store storage.Storage, ctx context.Context, idOrPrefix string) (*storage.Issue, error) {
	// Try exact match first
	issue, err := store.Get(ctx, idOrPrefix)
	if err == nil {
		return issue, nil
	}
	if err != storage.ErrNotFound {
		return nil, err
	}

	// Try prefix matching
	return findByPrefix(store, ctx, idOrPrefix)
}
