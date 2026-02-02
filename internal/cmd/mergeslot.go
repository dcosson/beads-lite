package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/mergeslot"

	"github.com/spf13/cobra"
)

// MergeSlotJSON is the JSON output format for merge-slot commands.
type MergeSlotJSON struct {
	Status      string   `json:"status"`
	Holder      string   `json:"holder,omitempty"`
	Waiters     []string `json:"waiters,omitempty"`
	FirstWaiter string   `json:"first_waiter,omitempty"`
}

func mergeSlotJSON(ms mergeslot.MergeSlot) MergeSlotJSON {
	return MergeSlotJSON{
		Status:  ms.Status,
		Holder:  ms.Holder,
		Waiters: ms.Waiters,
	}
}

// newMergeSlotCmd creates the merge-slot command group.
func newMergeSlotCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge-slot",
		Short: "Manage merge slot mutex",
	}

	cmd.AddCommand(newMergeSlotCreateCmd(provider))
	cmd.AddCommand(newMergeSlotCheckCmd(provider))
	cmd.AddCommand(newMergeSlotAcquireCmd(provider))
	cmd.AddCommand(newMergeSlotReleaseCmd(provider))

	return cmd
}

// newMergeSlotCreateCmd creates the "merge-slot create" subcommand.
func newMergeSlotCreateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the merge slot (idempotent)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if err := mergeslot.Create(ctx, app.MergeSlotStore); err != nil {
				return err
			}

			if app.JSON {
				ms, err := mergeslot.Check(ctx, app.MergeSlotStore)
				if err != nil {
					return err
				}
				return json.NewEncoder(app.Out).Encode(mergeSlotJSON(ms))
			}

			fmt.Fprintf(app.Out, "%s Merge slot created\n", app.SuccessColor("\u2713"))
			return nil
		},
	}

	return cmd
}

// newMergeSlotCheckCmd creates the "merge-slot check" subcommand.
func newMergeSlotCheckCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check merge slot status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			ms, err := mergeslot.Check(ctx, app.MergeSlotStore)
			if err != nil {
				return err
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(mergeSlotJSON(ms))
			}

			fmt.Fprintf(app.Out, "Status:  %s\n", ms.Status)
			if ms.Holder != "" {
				fmt.Fprintf(app.Out, "Holder:  %s\n", ms.Holder)
			}
			if len(ms.Waiters) > 0 {
				fmt.Fprintf(app.Out, "Waiters: %s\n", strings.Join(ms.Waiters, ", "))
			}

			return nil
		},
	}

	return cmd
}

// newMergeSlotAcquireCmd creates the "merge-slot acquire" subcommand.
func newMergeSlotAcquireCmd(provider *AppProvider) *cobra.Command {
	var wait bool

	cmd := &cobra.Command{
		Use:   "acquire <requester>",
		Short: "Acquire the merge slot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			requester := args[0]

			ms, err := mergeslot.Acquire(ctx, app.MergeSlotStore, requester, wait)
			if err != nil {
				if app.JSON {
					out := mergeSlotJSON(ms)
					return json.NewEncoder(app.Out).Encode(struct {
						MergeSlotJSON
						Error string `json:"error"`
					}{out, err.Error()})
				}
				return err
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(mergeSlotJSON(ms))
			}

			fmt.Fprintf(app.Out, "%s Merge slot acquired by %s\n", app.SuccessColor("\u2713"), requester)
			return nil
		},
	}

	cmd.Flags().BoolVar(&wait, "wait", false, "Add to waiters list if slot is held")

	return cmd
}

// newMergeSlotReleaseCmd creates the "merge-slot release" subcommand.
func newMergeSlotReleaseCmd(provider *AppProvider) *cobra.Command {
	var holder string

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release the merge slot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			ms, firstWaiter, err := mergeslot.Release(ctx, app.MergeSlotStore, holder)
			if err != nil {
				return err
			}

			if app.JSON {
				out := mergeSlotJSON(ms)
				out.FirstWaiter = firstWaiter
				return json.NewEncoder(app.Out).Encode(out)
			}

			fmt.Fprintf(app.Out, "%s Merge slot released\n", app.SuccessColor("\u2713"))
			if firstWaiter != "" {
				fmt.Fprintf(app.Out, "Next waiter: %s\n", firstWaiter)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&holder, "holder", "", "Verify current holder before releasing")

	return cmd
}
