package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newEditCmd creates the edit command.
func newEditCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <issue-id>",
		Short: "Edit issue description in $EDITOR",
		Long: `Open an issue's description in your default text editor for interactive editing.

The editor is resolved from $EDITOR, then $VISUAL, then falls back to vi.

If the description is unchanged after editing, no update is made.

Examples:
  bd edit bd-a1b2
  bd edit a1b2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			query := args[0]

			store := app.Storage

			// Try exact match first, then prefix
			issue, err := store.Get(ctx, query)
			if err == issuestorage.ErrNotFound {
				issue, err = findByPrefix(store, ctx, query)
			}
			if err != nil {
				if err == issuestorage.ErrNotFound {
					return fmt.Errorf("no issue found matching %q", query)
				}
				return err
			}

			// Resolve editor
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				editor = "vi"
			}

			// Create temp file with descriptive name for syntax highlighting
			tmpDir := os.TempDir()
			tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("bd-edit-%s-*.md", issue.ID))
			if err != nil {
				return fmt.Errorf("creating temp file: %w", err)
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)

			// Write current description
			if _, err := tmpFile.WriteString(issue.Description); err != nil {
				tmpFile.Close()
				return fmt.Errorf("writing to temp file: %w", err)
			}
			if err := tmpFile.Close(); err != nil {
				return fmt.Errorf("closing temp file: %w", err)
			}

			// Open editor
			editorPath, err := exec.LookPath(filepath.Base(editor))
			if err != nil {
				// Try the full value in case it's an absolute path
				editorPath = editor
			}
			editorCmd := exec.CommandContext(ctx, editorPath, tmpPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}

			// Read back edited content
			data, err := os.ReadFile(tmpPath)
			if err != nil {
				return fmt.Errorf("reading edited file: %w", err)
			}
			newDescription := string(data)

			// Check if changed
			if newDescription == issue.Description {
				if app.JSON {
					result := []IssueJSON{ToIssueJSON(ctx, store, issue, false, false)}
					return json.NewEncoder(app.Out).Encode(result)
				}
				fmt.Fprintf(app.Out, "No changes for %s\n", issue.ID)
				return nil
			}

			// Update description
			if err := store.Modify(ctx, issue.ID, func(i *issuestorage.Issue) error {
				i.Description = newDescription
				return nil
			}); err != nil {
				return fmt.Errorf("updating issue: %w", err)
			}

			if app.JSON {
				updatedIssue, err := store.Get(ctx, issue.ID)
				if err != nil {
					return fmt.Errorf("fetching updated issue: %w", err)
				}
				result := []IssueJSON{ToIssueJSON(ctx, store, updatedIssue, false, false)}
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Updated description for %s\n", issue.ID)
			return nil
		},
	}

	return cmd
}
