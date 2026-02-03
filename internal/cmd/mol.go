package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/meow"

	"github.com/spf13/cobra"
)

// newMolCmd creates the mol command group with subcommands.
func newMolCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mol",
		Short: "Manage MEOW molecules (formula instances)",
		Long: `Manage MEOW molecules — instances of formula templates.

A molecule is a tree of issues created by pouring a formula.
Use subcommands to create, inspect, and clean up molecules.

Subcommands:
  pour      Pour a formula to create a molecule
  wisp      Pour an ephemeral (temporary) molecule
  seed      Preflight check: verify formula files are valid
  current   Show current steps for a molecule
  progress  Show completion statistics
  stale     Find blocking/stale steps
  burn      Cascade-delete a molecule
  squash    Create a digest from a molecule
  gc        Clean up old ephemeral molecules`,
	}

	cmd.AddCommand(newMolPourCmd(provider))
	cmd.AddCommand(newMolWispCmd(provider))
	cmd.AddCommand(newMolShowCmd(provider))
	cmd.AddCommand(newMolCurrentCmd(provider))
	cmd.AddCommand(newMolProgressCmd(provider))
	cmd.AddCommand(newMolStaleCmd(provider))
	cmd.AddCommand(newMolBurnCmd(provider))
	cmd.AddCommand(newMolSquashCmd(provider))
	cmd.AddCommand(newMolGCCmd(provider))
	cmd.AddCommand(newMolSeedCmd(provider))

	return cmd
}

// newMolPourCmd creates the "mol pour" subcommand.
func newMolPourCmd(provider *AppProvider) *cobra.Command {
	var vars []string

	cmd := &cobra.Command{
		Use:   "pour <formula-name>",
		Short: "Pour a formula to create a molecule",
		Long: `Instantiate a formula by name, creating a molecule (issue tree).

Variables can be supplied with --var key=value (repeatable).
Missing required variables cause an error.

Examples:
  bd mol pour feature-workflow
  bd mol pour bug-triage --var component=auth --var severity=high`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			actor, err := resolveActor(app)
			if err != nil {
				return err
			}

			opts := meow.PourOptions{
				FormulaName:    args[0],
				Vars:           parseVarFlags(vars),
				PrefixAddition: "mol",
				SearchPath:     meow.DefaultSearchPath(app.ConfigDir),
				Actor:          actor,
			}

			result, err := meow.Pour(cmd.Context(), app.Storage, opts)
			if err != nil {
				return fmt.Errorf("pour %s: %w", args[0], err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Poured molecule: %s\n", result.NewEpicID)
			fmt.Fprintf(app.Out, "  Children: %d\n", result.Created-1)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&vars, "var", nil, "Variable assignment (key=value, repeatable)")

	return cmd
}

// newMolWispCmd creates the "mol wisp" subcommand (ephemeral pour).
func newMolWispCmd(provider *AppProvider) *cobra.Command {
	var vars []string

	wispRunE := func(cmd *cobra.Command, args []string) error {
		app, err := provider.Get()
		if err != nil {
			return err
		}

		actor, err := resolveActor(app)
		if err != nil {
			return err
		}

		opts := meow.PourOptions{
			FormulaName:    args[0],
			Vars:           parseVarFlags(vars),
			Ephemeral:      true,
			PrefixAddition: "wisp",
			SearchPath:     meow.DefaultSearchPath(app.ConfigDir),
			Actor:          actor,
		}

		result, err := meow.Pour(cmd.Context(), app.Storage, opts)
		if err != nil {
			return fmt.Errorf("wisp %s: %w", args[0], err)
		}

		if app.JSON {
			return json.NewEncoder(app.Out).Encode(result)
		}

		fmt.Fprintf(app.Out, "Poured ephemeral molecule: %s\n", result.NewEpicID)
		fmt.Fprintf(app.Out, "  Children: %d\n", result.Created-1)
		return nil
	}

	cmd := &cobra.Command{
		Use:   "wisp <formula-name>",
		Short: "Pour an ephemeral molecule (auto-cleaned by gc)",
		Long: `Like pour, but marks the molecule as ephemeral.
Ephemeral molecules are automatically cleaned up by "bd mol gc".

Examples:
  bd mol wisp scratch-pad
  bd mol wisp create scratch-pad
  bd mol wisp spike --var topic=caching`,
		Args: cobra.ExactArgs(1),
		RunE: wispRunE,
	}

	createCmd := &cobra.Command{
		Use:   "create <formula-name>",
		Short: "Pour an ephemeral molecule (auto-cleaned by gc)",
		Args:  cobra.ExactArgs(1),
		RunE:  wispRunE,
	}
	createCmd.Flags().StringArrayVar(&vars, "var", nil, "Variable assignment (key=value, repeatable)")
	cmd.AddCommand(createCmd)

	cmd.Flags().StringArrayVar(&vars, "var", nil, "Variable assignment (key=value, repeatable)")

	return cmd
}

// newMolShowCmd creates the "mol show" subcommand.
func newMolShowCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <molecule-id>",
		Short: "Show full molecule details including issues and dependencies",
		Long: `Display all issues, dependencies, and metadata for a molecule.

Examples:
  bd mol show bd-a1b2
  bd mol show bd-a1b2 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			molID := args[0]

			root, err := app.Storage.Get(ctx, molID)
			if err != nil {
				return fmt.Errorf("get molecule root %s: %w", molID, err)
			}

			children, err := graph.CollectMoleculeChildren(ctx, app.Storage, molID)
			if err != nil {
				return fmt.Errorf("collect children of %s: %w", molID, err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(molShowToJSON(root, children))
			}

			// Text output.
			fmt.Fprintf(app.Out, "Molecule: %s (%s)\n", root.ID, root.Title)
			fmt.Fprintf(app.Out, "  Status: %s\n", root.Status)
			fmt.Fprintf(app.Out, "  Issues: %d\n", len(children)+1)
			for _, child := range children {
				fmt.Fprintf(app.Out, "  - %s: %s [%s]\n", child.ID, child.Title, child.Status)
			}
			return nil
		},
	}

	return cmd
}

// molShowToJSON builds the MolShowJSON output from root and children.
func molShowToJSON(root *issuestorage.Issue, children []*issuestorage.Issue) MolShowJSON {
	// Gather all dependencies across all issues in the molecule.
	allIssues := make([]*issuestorage.Issue, 0, len(children)+1)
	allIssues = append(allIssues, root)
	allIssues = append(allIssues, children...)

	var deps []ListDepJSON
	for _, issue := range allIssues {
		for _, dep := range issue.Dependencies {
			deps = append(deps, ListDepJSON{
				CreatedAt:   formatTime(issue.CreatedAt),
				CreatedBy:   issue.CreatedBy,
				DependsOnID: dep.ID,
				IssueID:     issue.ID,
				Type:        string(dep.Type),
			})
		}
	}

	// Convert all issues to MolIssueJSON (children + root, matching reference format).
	issues := make([]MolIssueJSON, 0, len(children)+1)
	for _, child := range children {
		issues = append(issues, ToMolIssueJSON(child))
	}
	issues = append(issues, ToMolIssueJSON(root))

	return MolShowJSON{
		BondedFrom:   nil,
		Dependencies: deps,
		IsCompound:   false,
		Issues:       issues,
		Root:         ToMolIssueJSON(root),
		Variables:    nil,
	}
}

// newMolCurrentCmd creates the "mol current" subcommand.
func newMolCurrentCmd(provider *AppProvider) *cobra.Command {
	var actor string
	var limit int
	var rangeFlag string

	cmd := &cobra.Command{
		Use:   "current [molecule-id]",
		Short: "Show current steps for a molecule",
		Long: `List the steps of a molecule, classified by status.

Use --for to filter to steps assigned to a specific actor.
Use --limit to cap the number of steps shown.
Use --range to show a slice of step IDs (start-end).

Examples:
  bd mol current bd-a1b2
  bd mol current bd-a1b2 --for alice --limit 10
  bd mol current bd-a1b2 --range step1-step5`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			opts := meow.CurrentOptions{
				Actor: actor,
				Limit: limit,
			}

			if len(args) == 1 {
				opts.MoleculeID = args[0]
			} else {
				// No arg: resolve actor for inference.
				resolvedActor, err := resolveActor(app)
				if err != nil {
					return err
				}
				if opts.Actor == "" {
					opts.Actor = resolvedActor
				}
			}

			if rangeFlag != "" {
				parts := strings.SplitN(rangeFlag, "-", 2)
				if len(parts) == 2 {
					opts.RangeStart = parts[0]
					opts.RangeEnd = parts[1]
				}
			}

			result, err := meow.Current(cmd.Context(), app.Storage, opts)
			if err != nil {
				return fmt.Errorf("current: %w", err)
			}

			if result == nil {
				fmt.Fprintln(app.Out, "No molecules in progress.")
				fmt.Fprintln(app.Out, "Hint: use `bd mol pour` to create one, or `bd update --status in_progress` to start a step.")
				return nil
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(molCurrentToJSON(result))
			}

			if len(result.Steps) == 0 {
				fmt.Fprintln(app.Out, "No steps found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Molecule: %s (%s)\n", result.RootID, result.Title)
			for _, s := range result.Steps {
				assignee := ""
				if s.Assignee != "" {
					assignee = fmt.Sprintf(" (@%s)", s.Assignee)
				}
				fmt.Fprintf(app.Out, "  [%s] %s: %s%s\n", s.Status, s.ID, s.Title, assignee)
			}
			fmt.Fprintf(app.Out, "Progress: %d/%d (%.0f%%)\n",
				result.Progress.Completed, result.Progress.Total, result.Progress.Percent)
			return nil
		},
	}

	cmd.Flags().StringVar(&actor, "for", "", "Filter steps assigned to this actor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of steps to show")
	cmd.Flags().StringVar(&rangeFlag, "range", "", "Step ID range (start-end)")

	return cmd
}

// newMolProgressCmd creates the "mol progress" subcommand.
func newMolProgressCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "progress [molecule-id]",
		Short: "Show completion statistics for a molecule",
		Long: `Display completion progress for a molecule.

Shows total steps, completed steps, percentage, rate, and ETA.

Examples:
  bd mol progress bd-a1b2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			molID := args[0]

			result, err := meow.Progress(ctx, app.Storage, molID)
			if err != nil {
				return fmt.Errorf("progress: %w", err)
			}

			if app.JSON {
				// Load root issue for title.
				root, rootErr := app.Storage.Get(ctx, molID)
				title := ""
				if rootErr == nil {
					title = root.Title
				}

				// Find current step ID.
				currentStepID := ""
				view, viewErr := meow.Current(ctx, app.Storage, meow.CurrentOptions{MoleculeID: molID})
				if viewErr == nil {
					for _, s := range view.Steps {
						if s.Status == "current" {
							currentStepID = s.ID
							break
						}
					}
				}

				// Compute rate/ETA based on elapsed time.
				var ratePerHour, etaHours float64
				if rootErr == nil && result.Completed > 0 {
					elapsed := time.Since(root.CreatedAt).Hours()
					if elapsed > 0 {
						ratePerHour = float64(result.Completed) / elapsed
						remaining := result.Total - result.Completed
						if remaining > 0 && ratePerHour > 0 {
							etaHours = float64(remaining) / ratePerHour
						}
					}
				}

				return json.NewEncoder(app.Out).Encode(MolProgressJSON{
					Completed:     result.Completed,
					CurrentStepID: currentStepID,
					ETAHours:      etaHours,
					InProgress:    result.InProgress,
					MoleculeID:    molID,
					MoleculeTitle: title,
					Percent:       result.Percent,
					RatePerHour:   ratePerHour,
					Total:         result.Total,
				})
			}

			fmt.Fprintf(app.Out, "Progress: %d/%d (%.0f%%)\n", result.Completed, result.Total, result.Percent)
			fmt.Fprintf(app.Out, "  In Progress: %d\n", result.InProgress)
			fmt.Fprintf(app.Out, "  Ready:       %d\n", result.Ready)
			fmt.Fprintf(app.Out, "  Blocked:     %d\n", result.Blocked)
			fmt.Fprintf(app.Out, "  Pending:     %d\n", result.Pending)
			return nil
		},
	}

	return cmd
}

// newMolStaleCmd creates the "mol stale" subcommand.
func newMolStaleCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stale <molecule-id>",
		Short: "Find stale/blocking steps in a molecule",
		Long: `Identify steps that are blocking progress.

A step is stale if its dependencies are met but it hasn't progressed.

Examples:
  bd mol stale bd-a1b2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			steps, err := meow.FindStaleSteps(cmd.Context(), app.Storage, args[0])
			if err != nil {
				return fmt.Errorf("stale: %w", err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(steps)
			}

			if len(steps) == 0 {
				fmt.Fprintln(app.Out, "No stale steps found.")
				return nil
			}

			fmt.Fprintf(app.Out, "Stale steps (%d):\n", len(steps))
			for _, s := range steps {
				fmt.Fprintf(app.Out, "  %s: %s (%s)\n", s.ID, s.Title, s.Reason)
			}
			return nil
		},
	}

	return cmd
}

// newMolBurnCmd creates the "mol burn" subcommand.
func newMolBurnCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn <molecule-id>",
		Short: "Cascade-delete a molecule and all its steps",
		Long: `Delete a molecule root and all of its child step issues.

This is irreversible. All issues in the molecule tree are removed.

Examples:
  bd mol burn bd-a1b2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			result, err := meow.Burn(cmd.Context(), app.Storage, args[0])
			if err != nil {
				return fmt.Errorf("burn: %w", err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Burned molecule: %s (%d issues deleted)\n", args[0], result.DeletedCount)
			return nil
		},
	}

	return cmd
}

// newMolSquashCmd creates the "mol squash" subcommand.
func newMolSquashCmd(provider *AppProvider) *cobra.Command {
	var summary string
	var keepChildren bool

	cmd := &cobra.Command{
		Use:   "squash <molecule-id>",
		Short: "Create a digest from a molecule",
		Long: `Squash a molecule into a single digest issue.

The digest summarises all steps. By default, child step issues are
removed after squashing. Use --keep-children to preserve them.

Examples:
  bd mol squash bd-a1b2 --summary "Completed auth feature"
  bd mol squash bd-a1b2 --keep-children`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			opts := meow.SquashOptions{
				MoleculeID:   args[0],
				Summary:      summary,
				KeepChildren: keepChildren,
			}

			result, err := meow.Squash(cmd.Context(), app.Storage, opts)
			if err != nil {
				return fmt.Errorf("squash: %w", err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "Squashed molecule into digest: %s\n", result.DigestID)
			fmt.Fprintf(app.Out, "  Squashed steps: %d\n", len(result.SquashedIDs))
			return nil
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "", "Summary text for the digest issue")
	cmd.Flags().BoolVar(&keepChildren, "keep-children", false, "Keep child step issues after squashing")

	return cmd
}

// newMolGCCmd creates the "mol gc" subcommand.
func newMolGCCmd(provider *AppProvider) *cobra.Command {
	var olderThan string

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Clean up old ephemeral molecules",
		Long: `Remove ephemeral molecules older than the specified duration.

Ephemeral molecules are created with "bd mol wisp" and are intended
for temporary/scratch work.

Examples:
  bd mol gc                      # default: 7 days
  bd mol gc --older-than 24h
  bd mol gc --older-than 168h    # 1 week`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			dur := 7 * 24 * time.Hour // default: 7 days
			if olderThan != "" {
				parsed, parseErr := time.ParseDuration(olderThan)
				if parseErr != nil {
					return fmt.Errorf("invalid duration %q: %w", olderThan, parseErr)
				}
				dur = parsed
			}

			opts := meow.GCOptions{
				OlderThan: dur,
			}

			result, err := meow.GC(cmd.Context(), app.Storage, opts)
			if err != nil {
				return fmt.Errorf("gc: %w", err)
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "GC: removed %d ephemeral molecules\n", result.Count)
			return nil
		},
	}

	cmd.Flags().StringVar(&olderThan, "older-than", "", "Remove molecules older than this duration (e.g. 24h, 168h)")

	return cmd
}

// molCurrentToJSON converts a MoleculeView to the reference JSON format.
func molCurrentToJSON(view *meow.MoleculeView) []MolCurrentJSON {
	steps := make([]MolCurrentStepJSON, 0, len(view.Steps))
	var nextStep *MolIssueJSON

	for _, s := range view.Steps {
		// Map graph status to reference output status.
		var status string
		var isCurrent bool
		switch s.Status {
		case "done":
			status = "completed"
		case "current":
			status = "in_progress"
			isCurrent = true
		case "ready":
			status = "ready"
			if nextStep == nil && s.Issue != nil {
				ij := ToMolIssueJSON(s.Issue)
				nextStep = &ij
			}
		case "blocked", "pending":
			status = "pending"
		default:
			status = string(s.Status)
		}

		step := MolCurrentStepJSON{
			IsCurrent: isCurrent,
			Status:    status,
		}
		if s.Issue != nil {
			step.Issue = ToMolIssueJSON(s.Issue)
		}
		steps = append(steps, step)
	}

	return []MolCurrentJSON{{
		Completed:     view.Progress.Completed,
		MoleculeID:    view.RootID,
		MoleculeTitle: view.Title,
		NextStep:      nextStep,
		Steps:         steps,
		Total:         view.Progress.Total,
	}}
}

// parseVarFlags converts ["key=value", ...] into a map.
func parseVarFlags(vars []string) map[string]string {
	m := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
