package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newImportCmd creates the import command.
// In beads-lite the storage layer is filesystem-based with no separate import
// step, so this command is a no-op accepted for compatibility with the
// reference implementation which imports from JSONL exports.
func newImportCmd(provider *AppProvider) *cobra.Command {
	var (
		inputFile           string
		renameOnImport      bool
		noGitHistory        bool
		protectLeftSnapshot bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import beads data (no-op in beads-lite)",
		Long: `In the reference implementation, import reads issues from a JSONL file
into the database. beads-lite uses direct filesystem storage and does
not require a separate import step. This command is accepted for
compatibility but performs no action.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			if app.JSON {
				fmt.Fprintln(app.Out, `{"status":"noop","message":"import is not needed in beads-lite"}`)
				return nil
			}

			fmt.Fprintln(app.Out, "import: no-op (beads-lite uses direct filesystem storage)")
			return nil
		},
	}

	// Compatibility flags — accepted but not used by beads-lite.
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSONL file (accepted for compatibility, no-op)")
	cmd.Flags().BoolVar(&renameOnImport, "rename-on-import", false, "Accepted for compatibility (no-op)")
	cmd.Flags().BoolVar(&noGitHistory, "no-git-history", false, "Accepted for compatibility (no-op)")
	cmd.Flags().BoolVar(&protectLeftSnapshot, "protect-left-snapshot", false, "Accepted for compatibility (no-op)")

	return cmd
}
