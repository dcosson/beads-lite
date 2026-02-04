package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newActivityCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Real-time mutation feed (no-op in beads-lite)",
		Long: `In the reference implementation, activity streams real-time issue
mutations. beads-lite does not implement a daemon or event system,
so this command is accepted for compatibility but produces no output.

With --follow, the process stays open until interrupted (Ctrl-C).
Without --follow, it exits immediately.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			follow, _ := cmd.Flags().GetBool("follow")
			if !follow {
				return nil
			}

			// Block until interrupted
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			return nil
		},
	}

	cmd.Flags().Bool("follow", false, "Follow mode: keep process open for new events (no-op)")
	cmd.Flags().Bool("town", false, "Filter to town events (no-op)")
	cmd.Flags().Bool("json", false, "Output in JSON format (no-op)")

	return cmd
}
