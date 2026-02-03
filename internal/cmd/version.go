package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current version of beads-lite. It can be overridden at build
// time via -ldflags "-X beads-lite/internal/cmd.Version=1.2.3".
var Version = "0.50.0"

func newVersionCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider.JSONOutput {
				return json.NewEncoder(provider.Out).Encode(map[string]string{
					"version": Version,
				})
			}
			fmt.Fprintf(provider.Out, "bd version %s\n", Version)
			return nil
		},
	}
	return cmd
}
