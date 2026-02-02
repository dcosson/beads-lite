package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/agent"
	"beads-lite/internal/slot"

	"github.com/spf13/cobra"
)

// AgentJSON is the JSON output format for agent commands.
type AgentJSON struct {
	Agent        string `json:"agent"`
	State        string `json:"state,omitempty"`
	LastActivity string `json:"last_activity,omitempty"`
	RoleType     string `json:"role_type,omitempty"`
	Rig          string `json:"rig,omitempty"`
	Hook         string `json:"hook,omitempty"`
	Role         string `json:"role,omitempty"`
}

// newAgentCmd creates the agent command group.
func newAgentCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agent state",
	}

	cmd.AddCommand(newAgentStateCmd(provider))
	cmd.AddCommand(newAgentHeartbeatCmd(provider))
	cmd.AddCommand(newAgentShowCmd(provider))

	return cmd
}

// newAgentStateCmd creates the "agent state" subcommand.
func newAgentStateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state <agent-id> <state>",
		Short: "Set agent state (creates agent if it doesn't exist)",
		Long: fmt.Sprintf("Set agent state. Valid states: %s",
			strings.Join(agent.ValidStates, ", ")),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			agentID := args[0]
			state := args[1]

			if err := agent.SetState(ctx, app.AgentStore, agentID, state); err != nil {
				return err
			}

			if app.JSON {
				a, err := agent.GetAgent(ctx, app.AgentStore, agentID)
				if err != nil {
					return err
				}
				return json.NewEncoder(app.Out).Encode(AgentJSON{
					Agent:        agentID,
					State:        a.State,
					LastActivity: a.LastActivity.Format("2006-01-02T15:04:05Z"),
					RoleType:     a.RoleType,
					Rig:          a.Rig,
				})
			}

			fmt.Fprintf(app.Out, "%s Set agent %s state to %s\n", app.SuccessColor("✓"), agentID, state)
			return nil
		},
	}

	return cmd
}

// newAgentHeartbeatCmd creates the "agent heartbeat" subcommand.
func newAgentHeartbeatCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heartbeat <agent-id>",
		Short: "Update agent last_activity timestamp",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			agentID := args[0]

			if err := agent.Heartbeat(ctx, app.AgentStore, agentID); err != nil {
				return err
			}

			if app.JSON {
				a, err := agent.GetAgent(ctx, app.AgentStore, agentID)
				if err != nil {
					return err
				}
				return json.NewEncoder(app.Out).Encode(AgentJSON{
					Agent:        agentID,
					State:        a.State,
					LastActivity: a.LastActivity.Format("2006-01-02T15:04:05Z"),
					RoleType:     a.RoleType,
					Rig:          a.Rig,
				})
			}

			fmt.Fprintf(app.Out, "%s Heartbeat recorded for agent %s\n", app.SuccessColor("✓"), agentID)
			return nil
		},
	}

	return cmd
}

// newAgentShowCmd creates the "agent show" subcommand.
func newAgentShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show agent state and slots",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			agentID := args[0]

			a, err := agent.GetAgent(ctx, app.AgentStore, agentID)
			if err != nil {
				return err
			}

			slots, err := slot.GetSlots(ctx, app.SlotStore, agentID)
			if err != nil {
				return err
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(AgentJSON{
					Agent:        agentID,
					State:        a.State,
					LastActivity: a.LastActivity.Format("2006-01-02T15:04:05Z"),
					RoleType:     a.RoleType,
					Rig:          a.Rig,
					Hook:         slots.Hook,
					Role:         slots.Role,
				})
			}

			fmt.Fprintf(app.Out, "Agent: %s\n", agentID)
			fmt.Fprintf(app.Out, "State: %s\n", a.State)
			fmt.Fprintf(app.Out, "Last:  %s\n", a.LastActivity.Format("2006-01-02T15:04:05Z"))

			if a.RoleType != "" {
				fmt.Fprintf(app.Out, "Type:  %s\n", a.RoleType)
			}
			if a.Rig != "" {
				fmt.Fprintf(app.Out, "Rig:   %s\n", a.Rig)
			}

			hookDisplay := "(empty)"
			if slots.Hook != "" {
				hookDisplay = slots.Hook
				if title := lookupTitle(app, ctx, slots.Hook); title != "" {
					hookDisplay = fmt.Sprintf("%s (%s)", slots.Hook, title)
				}
			}
			fmt.Fprintf(app.Out, "Hook:  %s\n", hookDisplay)

			roleDisplay := "(empty)"
			if slots.Role != "" {
				roleDisplay = slots.Role
				if title := lookupTitle(app, ctx, slots.Role); title != "" {
					roleDisplay = fmt.Sprintf("%s (%s)", slots.Role, title)
				}
			}
			fmt.Fprintf(app.Out, "Role:  %s\n", roleDisplay)

			return nil
		},
	}

	return cmd
}
