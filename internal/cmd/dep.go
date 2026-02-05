package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/routing"

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
			dt := issuestorage.DependencyType(depType)
			if !issuestorage.ValidDependencyTypes[dt] {
				return fmt.Errorf("invalid dependency type %q; valid types: blocks, tracks, related, parent-child, discovered-from, until, caused-by, validates, relates-to, supersedes", depType)
			}

			// Route to correct storage for the issue
			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			// Resolve IDs (support prefix matching)
			issue, err := resolveIssue(store, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			// Route dependency separately — it may live in a different rig
			depStore, err := app.StorageFor(ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("routing dependency %s: %w", dependencyID, err)
			}

			dependency, err := resolveIssue(depStore, ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("resolving dependency %s: %w", dependencyID, err)
			}

			// Reject adding dependencies on tombstoned issues
			if issue.Status == issuestorage.StatusTombstone {
				return fmt.Errorf("cannot add dependency: issue %s is tombstoned", issue.ID)
			}
			if dependency.Status == issuestorage.StatusTombstone {
				return fmt.Errorf("cannot add dependency on tombstoned issue %s", dependency.ID)
			}

			// Add the typed dependency.
			if app.Router.SameStore(issue.ID, dependency.ID) {
				// Same rig: use store's AddDependency (handles cycles, parent-child atomically)
				if err := store.AddDependency(ctx, issue.ID, dependency.ID, dt); err != nil {
					if err == issuestorage.ErrCycle {
						return fmt.Errorf("cannot add dependency: would create a cycle")
					}
					return fmt.Errorf("adding dependency: %w", err)
				}
			} else {
				// Cross-store: parent-child must stay same-rig
				if dt == issuestorage.DepTypeParentChild {
					return fmt.Errorf("cannot add parent-child dependency across different rigs")
				}
				if err := addCrossStoreDep(ctx, routing.NewGetter(app.Router, app.Storage), store, depStore, issue.ID, dependency.ID, dt); err != nil {
					return err
				}
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

			fmt.Fprintf(app.Out, "%s Added dependency: %s depends on %s (type: %s)\n", app.SuccessColor("✓"), issue.ID, dependency.ID, depType)
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

			// Route to correct storage
			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			// Resolve IDs (support prefix matching)
			issue, err := resolveIssue(store, ctx, issueID)
			if err != nil {
				return fmt.Errorf("resolving issue %s: %w", issueID, err)
			}

			// Route dependency separately — it may live in a different rig
			depStore, err := app.StorageFor(ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("routing dependency %s: %w", dependencyID, err)
			}

			dependency, err := resolveIssue(depStore, ctx, dependencyID)
			if err != nil {
				return fmt.Errorf("resolving dependency %s: %w", dependencyID, err)
			}

			// Remove the dependency.
			if app.Router.SameStore(issue.ID, dependency.ID) {
				// Same rig: use store's RemoveDependency (handles parent-child cleanup)
				if err := store.RemoveDependency(ctx, issue.ID, dependency.ID); err != nil {
					return fmt.Errorf("removing dependency: %w", err)
				}
			} else {
				// Cross-store: update each side independently
				if err := removeCrossStoreDep(ctx, store, depStore, issue.ID, dependency.ID); err != nil {
					return fmt.Errorf("removing dependency: %w", err)
				}
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
			var typeFilter *issuestorage.DependencyType
			if filterType != "" {
				dt := issuestorage.DependencyType(filterType)
				if !issuestorage.ValidDependencyTypes[dt] {
					return fmt.Errorf("invalid dependency type %q", filterType)
				}
				typeFilter = &dt
			}

			// Route to correct storage
			store, err := app.StorageFor(ctx, issueID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", issueID, err)
			}

			// Resolve ID (support prefix matching)
			issue, err := resolveIssue(store, ctx, issueID)
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
func filterDeps(deps []issuestorage.Dependency, typeFilter *issuestorage.DependencyType) []issuestorage.Dependency {
	if typeFilter == nil {
		return deps
	}
	var filtered []issuestorage.Dependency
	for _, d := range deps {
		if d.Type == *typeFilter {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// outputDepListJSON outputs dependency list in JSON format.
// Returns an array of enriched dependency objects (matching show --json format).
func outputDepListJSON(app *App, ctx context.Context, issue *issuestorage.Issue, tree bool, direction string, typeFilter *issuestorage.DependencyType) error {
	showDown := direction == "" || direction == "down"
	showUp := direction == "" || direction == "up"

	// Default to "down" (dependencies) when no direction specified and both would show
	// For original beads compatibility, dep list returns dependencies by default
	if direction == "" {
		showDown = true
		showUp = false
	}

	var deps []issuestorage.Dependency
	if showDown {
		deps = filterDeps(issue.Dependencies, typeFilter)
	} else if showUp {
		deps = filterDeps(issue.Dependents, typeFilter)
	}

	// Return array of enriched dependencies (like show --json format)
	result := enrichDependencies(ctx, routing.NewGetter(app.Router, app.Storage), deps)

	return json.NewEncoder(app.Out).Encode(result)
}

// outputDepListText outputs dependency list in text format.
func outputDepListText(app *App, ctx context.Context, issue *issuestorage.Issue, tree bool, direction string, typeFilter *issuestorage.DependencyType) error {
	fmt.Fprintf(app.Out, "%s: %s\n", issue.ID, issue.Title)
	fmt.Fprintln(app.Out, strings.Repeat("-", len(issue.ID)+len(issue.Title)+2))

	if tree {
		return outputDepTree(app, ctx, issue, 0, make(map[string]bool))
	}

	getter := routing.NewGetter(app.Router, app.Storage)
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
				dep, err := getter.Get(ctx, d.ID)
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
				dep, err := getter.Get(ctx, d.ID)
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
func outputDepTree(app *App, ctx context.Context, issue *issuestorage.Issue, depth int, visited map[string]bool) error {
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

	getter := routing.NewGetter(app.Router, app.Storage)
	// Recursively show dependencies
	for _, dep := range issue.Dependencies {
		depIssue, err := getter.Get(ctx, dep.ID)
		if err != nil {
			fmt.Fprintf(app.Out, "%s  └─ %s (error: %v)\n", indent, dep.ID, err)
			continue
		}
		outputDepTree(app, ctx, depIssue, depth+1, visited)
	}

	return nil
}

// statusIndicator returns a symbol for the issue status.
func statusIndicator(status issuestorage.Status) string {
	switch status {
	case issuestorage.StatusClosed:
		return "✓"
	case issuestorage.StatusInProgress:
		return "●"
	case issuestorage.StatusBlocked:
		return "✗"
	case issuestorage.StatusTombstone:
		return "†"
	default:
		return "○"
	}
}

// resolveIssue finds an issue by ID or prefix.
func resolveIssue(store issuestorage.IssueStore, ctx context.Context, idOrPrefix string) (*issuestorage.Issue, error) {
	// Try exact match first
	issue, err := store.Get(ctx, idOrPrefix)
	if err == nil {
		return issue, nil
	}
	if err != issuestorage.ErrNotFound {
		return nil, err
	}

	// Try prefix matching
	return findByPrefix(store, ctx, idOrPrefix)
}

// addCrossStoreDep adds a dependency where the issue and dependency live in
// different rigs. It performs routing-aware cycle detection, then updates each
// side in its own store. The two Modify calls are not atomic — bd doctor
// handles any resulting asymmetries.
func addCrossStoreDep(ctx context.Context, getter issuestorage.IssueGetter, store, depStore issuestorage.IssueStore, issueID, depID string, dt issuestorage.DependencyType) error {
	hasCycle, err := hasCrossStoreCycle(ctx, getter, issueID, depID)
	if err != nil {
		return fmt.Errorf("checking for cycles: %w", err)
	}
	if hasCycle {
		return fmt.Errorf("cannot add dependency: would create a cycle")
	}

	// Add dependency entry to the issue
	if err := store.Modify(ctx, issueID, func(issue *issuestorage.Issue) error {
		if !issue.HasDependency(depID) {
			issue.Dependencies = append(issue.Dependencies, issuestorage.Dependency{ID: depID, Type: dt})
		}
		return nil
	}); err != nil {
		return fmt.Errorf("updating issue %s: %w", issueID, err)
	}

	// Add inverse dependent entry to the dependency
	if err := depStore.Modify(ctx, depID, func(dep *issuestorage.Issue) error {
		if !dep.HasDependent(issueID) {
			dep.Dependents = append(dep.Dependents, issuestorage.Dependency{ID: issueID, Type: dt})
		}
		return nil
	}); err != nil {
		return fmt.Errorf("updating dependency %s: %w", depID, err)
	}

	return nil
}

// removeCrossStoreDep removes a dependency where the issue and dependency live
// in different rigs. Updates each side in its own store.
func removeCrossStoreDep(ctx context.Context, store, depStore issuestorage.IssueStore, issueID, depID string) error {
	if err := store.Modify(ctx, issueID, func(issue *issuestorage.Issue) error {
		// Check if this was a parent-child dep — clear Parent field
		for _, dep := range issue.Dependencies {
			if dep.ID == depID && dep.Type == issuestorage.DepTypeParentChild {
				issue.Parent = ""
				break
			}
		}
		issue.Dependencies = filterOutDep(issue.Dependencies, depID)
		return nil
	}); err != nil {
		return fmt.Errorf("updating issue %s: %w", issueID, err)
	}

	if err := depStore.Modify(ctx, depID, func(dep *issuestorage.Issue) error {
		dep.Dependents = filterOutDep(dep.Dependents, issueID)
		return nil
	}); err != nil {
		return fmt.Errorf("updating dependency %s: %w", depID, err)
	}

	return nil
}

// hasCrossStoreCycle checks whether adding a dependency from issueID to depID
// would create a cycle. It does BFS from depID through dependencies using the
// routing-aware getter so it can traverse cross-store edges.
func hasCrossStoreCycle(ctx context.Context, getter issuestorage.IssueGetter, issueID, depID string) (bool, error) {
	if issueID == depID {
		return true, nil
	}

	visited := make(map[string]bool)
	queue := []string{depID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		issue, err := getter.Get(ctx, current)
		if err != nil {
			if err == issuestorage.ErrNotFound {
				continue
			}
			return false, err
		}

		for _, dep := range issue.Dependencies {
			if dep.ID == issueID {
				return true, nil
			}
			if !visited[dep.ID] {
				queue = append(queue, dep.ID)
			}
		}
	}

	return false, nil
}

// filterOutDep removes a dependency entry by ID from a slice.
func filterOutDep(deps []issuestorage.Dependency, id string) []issuestorage.Dependency {
	result := make([]issuestorage.Dependency, 0, len(deps))
	for _, d := range deps {
		if d.ID != id {
			result = append(result, d)
		}
	}
	return result
}
