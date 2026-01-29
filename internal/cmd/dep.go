package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/storage"

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
	var depType string

	cmd := &cobra.Command{
		Use:   "add <issue-id> <dependency-id>",
		Short: "Add a dependency (issue depends on dependency)",
		Long: `Create a dependency relationship where issue depends on dependency.

This means the dependency must be completed before issue can start.
Both issues are updated: issue.dependencies gets dependency added,
and dependency.dependents gets issue added.

Cycle detection prevents circular dependencies.

Use --type to specify the dependency type (default: blocks).

Examples:
  bd dep add bd-a1b2 bd-c3d4                  # bd-a1b2 depends on bd-c3d4 (type: blocks)
  bd dep add bd-a1b2 bd-c3d4 --type tracks    # bd-a1b2 tracks bd-c3d4
  bd dep add bd-a1b2 bd-c3d4 --type parent-child  # sets parent-child relationship`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]
			dependencyID := args[1]

			// Validate dependency type
			dt := storage.DependencyType(depType)
			if !storage.ValidDependencyTypes[dt] {
				return fmt.Errorf("invalid dependency type %q; valid types: blocks, tracks, related, parent-child, discovered-from, until, caused-by, validates, relates-to, supersedes", depType)
			}

			// Resolve IDs (support prefix matching)
			issue, err := resolveIssue(app.Storage, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			dependency, err := resolveIssue(app.Storage, ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("resolving dependency %s: %w", dependencyID, err)
			}

			// Add the typed dependency (parent-child is handled automatically by AddDependency)
			if err := app.Storage.AddDependency(ctx, issue.ID, dependency.ID, dt); err != nil {
				if err == storage.ErrCycle {
					return fmt.Errorf("cannot add dependency: would create a cycle")
				}
				return fmt.Errorf("adding dependency: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{
					"issue_id":      issue.ID,
					"depends_on_id": dependency.ID,
					"type":          depType,
					"status":        "added",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Added dependency: %s depends on %s (type: %s)\n", issue.ID, dependency.ID, depType)
			return nil
		},
	}

	cmd.Flags().StringVarP(&depType, "type", "t", "blocks", "Dependency type (blocks, tracks, related, parent-child, etc.)")

	return cmd
}

// newDepRemoveCmd creates the "dep remove" subcommand.
func newDepRemoveCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <issue-id> <dependency-id>",
		Short: "Remove a dependency",
		Long: `Remove a dependency relationship between two issues.

Both issues are updated: dependency is removed from issue.dependencies,
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

			// RemoveDependency handles parent-child cleanup automatically
			if err := app.Storage.RemoveDependency(ctx, issue.ID, dependency.ID); err != nil {
				return fmt.Errorf("removing dependency: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{
					"issue_id":      issue.ID,
					"depends_on_id": dependency.ID,
					"status":        "removed",
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
	var direction string
	var filterType string

	cmd := &cobra.Command{
		Use:   "list <issue-id>",
		Short: "List dependencies for an issue",
		Long: `Show all dependencies for an issue.

By default, shows both dependencies and dependents.
Use --direction to control which to show:
  down  Show what this issue depends on (dependencies)
  up    Show what depends on this issue (dependents)

Use --type to filter by dependency type.
Use --tree to show a tree view of transitive dependencies.

Examples:
  bd dep list bd-a1b2                       # Show all deps
  bd dep list bd-a1b2 --direction down      # What this depends on
  bd dep list bd-a1b2 --direction up        # What depends on this
  bd dep list bd-a1b2 --type blocks         # Only blocking deps
  bd dep list bd-a1b2 --tree                # Show dependency tree`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			issueID := args[0]

			// Validate direction
			if direction != "" && direction != "down" && direction != "up" {
				return fmt.Errorf("invalid direction %q; must be 'down' or 'up'", direction)
			}

			// Validate type filter
			var typeFilter *storage.DependencyType
			if filterType != "" {
				dt := storage.DependencyType(filterType)
				if !storage.ValidDependencyTypes[dt] {
					return fmt.Errorf("invalid dependency type %q", filterType)
				}
				typeFilter = &dt
			}

			// Resolve ID (support prefix matching)
			issue, err := resolveIssue(app.Storage, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			if app.JSON {
				return outputDepListJSON(app, ctx, issue, tree, direction, typeFilter)
			}

			return outputDepListText(app, ctx, issue, tree, direction, typeFilter)
		},
	}

	cmd.Flags().BoolVar(&tree, "tree", false, "Show dependency tree (transitive dependencies)")
	cmd.Flags().StringVar(&direction, "direction", "", "Filter direction: 'down' (dependencies) or 'up' (dependents)")
	cmd.Flags().StringVarP(&filterType, "type", "t", "", "Filter by dependency type")

	return cmd
}

// filterDeps filters a dependency slice by type, if typeFilter is non-nil.
func filterDeps(deps []storage.Dependency, typeFilter *storage.DependencyType) []storage.Dependency {
	if typeFilter == nil {
		return deps
	}
	var filtered []storage.Dependency
	for _, d := range deps {
		if d.Type == *typeFilter {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// outputDepListJSON outputs dependency list in JSON format.
// Returns an array of enriched dependency objects (matching show --json format).
func outputDepListJSON(app *App, ctx context.Context, issue *storage.Issue, tree bool, direction string, typeFilter *storage.DependencyType) error {
	showDown := direction == "" || direction == "down"
	showUp := direction == "" || direction == "up"

	// Default to "down" (dependencies) when no direction specified and both would show
	// For original beads compatibility, dep list returns dependencies by default
	if direction == "" {
		showDown = true
		showUp = false
	}

	var deps []storage.Dependency
	if showDown {
		deps = filterDeps(issue.Dependencies, typeFilter)
	} else if showUp {
		deps = filterDeps(issue.Dependents, typeFilter)
	}

	// Return array of enriched dependencies (like show --json format)
	result := enrichDependencies(ctx, app.Storage, deps)

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
	for _, dep := range issue.Dependencies {
		depIssue, err := store.Get(ctx, dep.ID)
		if err != nil {
			node.Children = append(node.Children, &depTreeNode{
				ID:     dep.ID,
				Title:  "(error loading)",
				Status: "unknown",
			})
			continue
		}
		node.Children = append(node.Children, buildDepTree(store, ctx, depIssue, visited))
	}

	return node
}

// outputDepListText outputs dependency list in text format.
func outputDepListText(app *App, ctx context.Context, issue *storage.Issue, tree bool, direction string, typeFilter *storage.DependencyType) error {
	fmt.Fprintf(app.Out, "%s: %s\n", issue.ID, issue.Title)
	fmt.Fprintln(app.Out, strings.Repeat("-", len(issue.ID)+len(issue.Title)+2))

	if tree {
		return outputDepTree(app, ctx, issue, 0, make(map[string]bool))
	}

	showDown := direction == "" || direction == "down"
	showUp := direction == "" || direction == "up"

	// Direct dependencies
	if showDown {
		deps := filterDeps(issue.Dependencies, typeFilter)
		if len(deps) == 0 {
			fmt.Fprintln(app.Out, "\nDependencies: (none)")
		} else {
			fmt.Fprintln(app.Out, "\nDependencies:")
			for _, d := range deps {
				dep, err := app.Storage.Get(ctx, d.ID)
				if err != nil {
					fmt.Fprintf(app.Out, "  → %s [%s] (error: %v)\n", d.ID, d.Type, err)
				} else {
					status := statusIndicator(dep.Status)
					fmt.Fprintf(app.Out, "  → %s %s: %s [%s]\n", status, dep.ID, dep.Title, d.Type)
				}
			}
		}
	}

	// Dependents (what depends on this)
	if showUp {
		deps := filterDeps(issue.Dependents, typeFilter)
		if len(deps) == 0 {
			fmt.Fprintln(app.Out, "\nDependents: (none)")
		} else {
			fmt.Fprintln(app.Out, "\nDependents:")
			for _, d := range deps {
				dep, err := app.Storage.Get(ctx, d.ID)
				if err != nil {
					fmt.Fprintf(app.Out, "  ← %s [%s] (error: %v)\n", d.ID, d.Type, err)
				} else {
					status := statusIndicator(dep.Status)
					fmt.Fprintf(app.Out, "  ← %s %s: %s [%s]\n", status, dep.ID, dep.Title, d.Type)
				}
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
		if len(issue.Dependencies) > 0 {
			fmt.Fprintf(app.Out, "%s  └─ (cycle detected)\n", indent)
		}
		return nil
	}
	visited[issue.ID] = true

	// Recursively show dependencies
	for _, dep := range issue.Dependencies {
		depIssue, err := app.Storage.Get(ctx, dep.ID)
		if err != nil {
			fmt.Fprintf(app.Out, "%s  └─ %s (error: %v)\n", indent, dep.ID, err)
			continue
		}
		outputDepTree(app, ctx, depIssue, depth+1, visited)
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
