package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/slot"

	"github.com/spf13/cobra"
)

// SlotJSON is the JSON output format for slot commands.
type SlotJSON struct {
	Agent string `json:"agent"`
	Hook  string `json:"hook,omitempty"`
	Role  string `json:"role,omitempty"`
}

func slotJSON(agentID string, rec slot.SlotRecord) SlotJSON {
	return SlotJSON{
		Agent: agentID,
		Hook:  rec.Hook,
		Role:  rec.Role,
	}
}

// newSlotCmd creates the slot command group.
func newSlotCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slot",
		Short: "Manage agent bead slots",
	}

	cmd.AddCommand(newSlotShowCmd(provider))
	cmd.AddCommand(newSlotSetCmd(provider))
	cmd.AddCommand(newSlotClearCmd(provider))

	return cmd
}

// newSlotShowCmd creates the "slot show" subcommand.
func newSlotShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show slots for an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			agentID := args[0]

			rec, err := slot.GetSlots(ctx, app.SlotStore, agentID)
			if err != nil {
				return err
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(slotJSON(agentID, rec))
			}

			fmt.Fprintf(app.Out, "Agent: %s\n", agentID)

			hookDisplay := "(empty)"
			if rec.Hook != "" {
				hookDisplay = rec.Hook
				if title := lookupTitle(app, ctx, rec.Hook); title != "" {
					hookDisplay = fmt.Sprintf("%s (%s)", rec.Hook, title)
				}
			}
			fmt.Fprintf(app.Out, "Hook:  %s\n", hookDisplay)

			roleDisplay := "(empty)"
			if rec.Role != "" {
				roleDisplay = rec.Role
				if title := lookupTitle(app, ctx, rec.Role); title != "" {
					roleDisplay = fmt.Sprintf("%s (%s)", rec.Role, title)
				}
			}
			fmt.Fprintf(app.Out, "Role:  %s\n", roleDisplay)

			return nil
		},
	}

	return cmd
}

// newSlotSetCmd creates the "slot set" subcommand.
func newSlotSetCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <agent-id> <slot-name> <bead-id>",
		Short: "Set a slot on an agent",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			agentID := args[0]
			slotName := args[1]
			beadID := args[2]

			if slotName != "hook" && slotName != "role" {
				return fmt.Errorf("invalid slot %q: must be \"hook\" or \"role\"", slotName)
			}

			// Validate the target bead exists
			store, err := app.StorageFor(ctx, beadID)
			if err != nil {
				return fmt.Errorf("routing issue %s: %w", beadID, err)
			}
			if _, err := store.Get(ctx, beadID); err != nil {
				if err == issuestorage.ErrNotFound {
					return fmt.Errorf("bead %s not found", beadID)
				}
				return fmt.Errorf("getting bead %s: %w", beadID, err)
			}

			if err := slot.SetSlot(ctx, app.SlotStore, agentID, slotName, beadID); err != nil {
				return err
			}

			// GUPP: setting the hook slot marks the target bead as hooked
			if slotName == "hook" {
				if err := store.Modify(ctx, beadID, func(i *issuestorage.Issue) error {
					i.Status = issuestorage.StatusHooked
					return nil
				}); err != nil {
					return fmt.Errorf("setting hooked status on %s: %w", beadID, err)
				}
			}

			if app.JSON {
				rec, err := slot.GetSlots(ctx, app.SlotStore, agentID)
				if err != nil {
					return err
				}
				return json.NewEncoder(app.Out).Encode(slotJSON(agentID, rec))
			}

			fmt.Fprintf(app.Out, "%s Set %s on %s to %s\n", app.SuccessColor("✓"), slotName, agentID, beadID)
			return nil
		},
	}

	return cmd
}

// newSlotClearCmd creates the "slot clear" subcommand.
func newSlotClearCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear <agent-id> <slot-name>",
		Short: "Clear a slot on an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			agentID := args[0]
			slotName := args[1]

			if slotName != "hook" && slotName != "role" {
				return fmt.Errorf("invalid slot %q: must be \"hook\" or \"role\"", slotName)
			}

			prev, err := slot.ClearSlot(ctx, app.SlotStore, agentID, slotName)
			if err != nil {
				return err
			}

			if app.JSON {
				rec, err := slot.GetSlots(ctx, app.SlotStore, agentID)
				if err != nil {
					return err
				}
				return json.NewEncoder(app.Out).Encode(slotJSON(agentID, rec))
			}

			if prev == "" {
				fmt.Fprintf(app.Out, "%s on %s is already empty\n", slotName, agentID)
			} else {
				fmt.Fprintf(app.Out, "%s Cleared %s on %s\n", app.SuccessColor("✓"), slotName, agentID)
			}
			return nil
		},
	}

	return cmd
}

// lookupTitle tries to fetch a bead's title from the issue store.
// Returns "" if the lookup fails for any reason.
func lookupTitle(app *App, ctx context.Context, beadID string) string {
	store, err := app.StorageFor(ctx, beadID)
	if err != nil {
		return ""
	}
	issue, err := store.Get(ctx, beadID)
	if err != nil {
		return ""
	}
	return issue.Title
}
