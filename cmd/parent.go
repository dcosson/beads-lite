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

var parentCmd = &cobra.Command{
	Use:   "parent",
	Short: "Manage issue parent relationships",
	Long:  `Set or remove the parent of an issue to create hierarchical relationships.`,
}

var parentSetCmd = &cobra.Command{
	Use:   "set <child-id> <parent-id>",
	Short: "Set the parent of an issue",
	Long: `Set the parent of an issue, making it a child of the specified parent.
If the child already has a parent, it will be reparented to the new parent.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := GetApp()
		childID := args[0]
		parentID := args[1]

		ctx := context.Background()
		err := app.Storage.SetParent(ctx, childID, parentID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("issue not found")
			}
			if errors.Is(err, storage.ErrCycle) {
				return fmt.Errorf("cannot set parent: would create a cycle")
			}
			return err
		}

		if app.JSON {
			result := map[string]string{
				"child_id":  childID,
				"parent_id": parentID,
				"status":    "ok",
			}
			enc := json.NewEncoder(app.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		fmt.Fprintf(app.Out, "Set parent of %s to %s\n", childID, parentID)
		return nil
	},
}

var parentRemoveCmd = &cobra.Command{
	Use:   "remove <child-id>",
	Short: "Remove the parent of an issue",
	Long:  `Remove the parent relationship, making the issue a root issue.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := GetApp()
		childID := args[0]

		ctx := context.Background()
		err := app.Storage.RemoveParent(ctx, childID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("issue not found")
			}
			return err
		}

		if app.JSON {
			result := map[string]string{
				"child_id": childID,
				"status":   "ok",
			}
			enc := json.NewEncoder(app.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		fmt.Fprintf(app.Out, "Removed parent from %s\n", childID)
		return nil
	},
}

func init() {
	parentCmd.AddCommand(parentSetCmd)
	parentCmd.AddCommand(parentRemoveCmd)
	rootCmd.AddCommand(parentCmd)
}
