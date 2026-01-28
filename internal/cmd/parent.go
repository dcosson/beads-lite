package cmd

import (
	"encoding/json"
	"fmt"

	"beads2/internal/storage"

	"github.com/spf13/cobra"
)

// newParentCmd creates the parent command group.
func newParentCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parent",
		Short: "Manage issue hierarchy",
		Long:  `Commands for managing parent-child relationships between issues.`,
	}

	cmd.AddCommand(newParentSetCmd(provider))
	cmd.AddCommand(newParentRemoveCmd(provider))

	return cmd
}

// newParentSetCmd creates the parent set command.
func newParentSetCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <child-id> <parent-id>",
		Short: "Set the parent of an issue",
		Long: `Set the parent of an issue, creating a hierarchy relationship.

The child issue will have its parent field set to the parent issue,
and the parent issue will have the child added to its children list.

If the child already has a parent, it will be re-parented to the new parent.

Examples:
  bd parent set bd-a1b2 bd-c3d4   # Make bd-c3d4 the parent of bd-a1b2`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			childID := args[0]
			parentID := args[1]

			// Verify child exists
			child, err := app.Storage.Get(ctx, childID)
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("child issue not found: %s", childID)
				}
				return fmt.Errorf("getting child issue: %w", err)
			}

			// Verify parent exists
			_, err = app.Storage.Get(ctx, parentID)
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("parent issue not found: %s", parentID)
				}
				return fmt.Errorf("getting parent issue: %w", err)
			}

			// Set the parent
			if err := app.Storage.SetParent(ctx, childID, parentID); err != nil {
				if err == storage.ErrCycle {
					return fmt.Errorf("cannot set parent: would create a cycle")
				}
				return fmt.Errorf("setting parent: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{
					"child":  child.ID,
					"parent": parentID,
					"status": "updated",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Set parent of %s to %s\n", childID, parentID)
			return nil
		},
	}

	return cmd
}

// newParentRemoveCmd creates the parent remove command.
func newParentRemoveCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <child-id>",
		Short: "Remove the parent of an issue",
		Long: `Remove the parent of an issue, making it a root issue.

The child issue will have its parent field cleared,
and the current parent will have the child removed from its children list.

Examples:
  bd parent remove bd-a1b2   # Make bd-a1b2 a root issue`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			childID := args[0]

			// Verify child exists and has a parent
			child, err := app.Storage.Get(ctx, childID)
			if err != nil {
				if err == storage.ErrNotFound {
					return fmt.Errorf("issue not found: %s", childID)
				}
				return fmt.Errorf("getting issue: %w", err)
			}

			if child.Parent == "" {
				return fmt.Errorf("issue %s has no parent", childID)
			}

			oldParent := child.Parent

			// Remove the parent
			if err := app.Storage.RemoveParent(ctx, childID); err != nil {
				return fmt.Errorf("removing parent: %w", err)
			}

			// Output the result
			if app.JSON {
				result := map[string]string{
					"child":      child.ID,
					"old_parent": oldParent,
					"status":     "removed",
				}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Removed parent of %s (was %s)\n", childID, oldParent)
			return nil
		},
	}

	return cmd
}
