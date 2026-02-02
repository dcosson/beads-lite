package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// newSwarmCmd creates the swarm command group with subcommands.
func newSwarmCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swarm",
		Short: "Manage swarm molecules (coordinated multi-agent parallel work)",
		Long: `Manage swarm molecules — coordinated multi-agent parallel work on epics.

A swarm analyzes an epic's dependency graph to determine waves of parallelizable
work, then creates a molecule to track the coordinated execution.

Subcommands:
  validate   Analyze an epic's DAG for swarm feasibility
  create     Create a swarm molecule linked to an epic
  status     Show live status of a swarm (categorized children)
  list       List all swarm molecules with progress`,
	}

	cmd.AddCommand(newSwarmValidateCmd(provider))
	cmd.AddCommand(newSwarmCreateCmd(provider))
	cmd.AddCommand(newSwarmStatusCmd(provider))
	cmd.AddCommand(newSwarmListCmd(provider))

	return cmd
}

// SwarmValidateJSON is the JSON output format for "swarm validate".
type SwarmValidateJSON struct {
	EpicID         string              `json:"epic_id"`
	EpicTitle      string              `json:"epic_title"`
	Swarmable      bool                `json:"swarmable"`
	Waves          []SwarmWaveJSON     `json:"waves"`
	MaxParallelism int                 `json:"max_parallelism"`
	TotalChildren  int                 `json:"total_children"`
	Warnings       []string            `json:"warnings,omitempty"`
	Errors         []string            `json:"errors,omitempty"`
}

// SwarmWaveJSON represents a single wave in the validation output.
type SwarmWaveJSON struct {
	Wave   int              `json:"wave"`
	Issues []SwarmIssueJSON `json:"issues"`
}

// SwarmIssueJSON is a minimal issue representation for swarm output.
type SwarmIssueJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SwarmCreateJSON is the JSON output format for "swarm create".
type SwarmCreateJSON struct {
	MoleculeID string `json:"molecule_id"`
	EpicID     string `json:"epic_id"`
	EpicTitle  string `json:"epic_title"`
}

// SwarmStatusJSON is the JSON output format for "swarm status".
type SwarmStatusJSON struct {
	EpicID     string              `json:"epic_id"`
	EpicTitle  string              `json:"epic_title"`
	MoleculeID string              `json:"molecule_id,omitempty"`
	Total      int                 `json:"total"`
	Completed  int                 `json:"completed"`
	Active     int                 `json:"active"`
	Ready      int                 `json:"ready"`
	Blocked    int                 `json:"blocked"`
	Progress   float64             `json:"progress"`
	Children   []SwarmChildJSON    `json:"children"`
}

// SwarmChildJSON represents a child issue in status output.
type SwarmChildJSON struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Category string   `json:"category"`
	Blockers []string `json:"blockers,omitempty"`
}

// SwarmListJSON is the JSON output format for "swarm list".
type SwarmListJSON struct {
	Swarms []SwarmListEntryJSON `json:"swarms"`
}

// SwarmListEntryJSON is a single entry in swarm list output.
type SwarmListEntryJSON struct {
	MoleculeID string  `json:"molecule_id"`
	EpicID     string  `json:"epic_id"`
	EpicTitle  string  `json:"epic_title"`
	Total      int     `json:"total"`
	Completed  int     `json:"completed"`
	Active     int     `json:"active"`
	Ready      int     `json:"ready"`
	Blocked    int     `json:"blocked"`
	Progress   float64 `json:"progress"`
}

// newSwarmValidateCmd creates the "swarm validate" subcommand.
func newSwarmValidateCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <epic-id>",
		Short: "Analyze an epic's DAG for swarm feasibility",
		Long: `Validate whether an epic's dependency graph is suitable for swarming.

Computes topological waves, detects cycles, and reports structural warnings
such as disconnected subgraphs or suspicious dependency patterns.

Examples:
  bd swarm validate bl-abc123
  bd swarm validate bl-abc123 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			epicID := args[0]

			store, err := app.StorageFor(ctx, epicID)
			if err != nil {
				return err
			}

			epic, err := store.Get(ctx, epicID)
			if err != nil {
				return fmt.Errorf("get epic %s: %w", epicID, err)
			}

			children, err := graph.CollectMoleculeChildren(ctx, store, epicID)
			if err != nil {
				return fmt.Errorf("collect children: %w", err)
			}

			if len(children) == 0 {
				return fmt.Errorf("epic %s has no children", epicID)
			}

			result := SwarmValidateJSON{
				EpicID:        epic.ID,
				EpicTitle:     epic.Title,
				TotalChildren: len(children),
			}

			// Compute waves
			waves, err := graph.TopologicalWaves(children)
			if err != nil {
				result.Swarmable = false
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.Swarmable = true

				// Build ID→title map
				byID := make(map[string]string, len(children))
				for _, c := range children {
					byID[c.ID] = c.Title
				}

				maxPar := 0
				for i, wave := range waves {
					waveJSON := SwarmWaveJSON{Wave: i}
					for _, id := range wave {
						waveJSON.Issues = append(waveJSON.Issues, SwarmIssueJSON{
							ID:    id,
							Title: byID[id],
						})
					}
					result.Waves = append(result.Waves, waveJSON)
					if len(wave) > maxPar {
						maxPar = len(wave)
					}
				}
				result.MaxParallelism = maxPar
			}

			// Check for structural warnings
			result.Warnings = swarmWarnings(children)

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output
			if result.Swarmable {
				fmt.Fprintf(app.Out, "%s Swarmable: %s (%d children, %d waves, max parallelism %d)\n",
					app.SuccessColor("✓"), epic.Title, result.TotalChildren, len(result.Waves), result.MaxParallelism)
			} else {
				fmt.Fprintf(app.Out, "✗ Not swarmable: %s\n", epic.Title)
				for _, e := range result.Errors {
					fmt.Fprintf(app.Out, "  Error: %s\n", e)
				}
			}

			if len(result.Warnings) > 0 {
				fmt.Fprintf(app.Out, "\nWarnings:\n")
				for _, w := range result.Warnings {
					fmt.Fprintf(app.Out, "  ⚠ %s\n", w)
				}
			}

			if result.Swarmable {
				fmt.Fprintf(app.Out, "\nWaves:\n")
				for _, w := range result.Waves {
					fmt.Fprintf(app.Out, "  Wave %d:\n", w.Wave)
					for _, issue := range w.Issues {
						fmt.Fprintf(app.Out, "    %s: %s\n", issue.ID, issue.Title)
					}
				}
			}

			return nil
		},
	}

	return cmd
}

// newSwarmCreateCmd creates the "swarm create" subcommand.
func newSwarmCreateCmd(provider *AppProvider) *cobra.Command {
	var coordinator string

	cmd := &cobra.Command{
		Use:   "create <epic-id>",
		Short: "Create a swarm molecule linked to an epic",
		Long: `Create a swarm molecule for coordinated multi-agent work on an epic.

Runs validate first — errors if the epic is not swarmable. If the target is a
single task (not an epic), auto-wraps by creating an epic with the task as its
only child.

Examples:
  bd swarm create bl-abc123
  bd swarm create bl-abc123 --coordinator witness
  bd swarm create bl-task1  # auto-wraps single task in epic`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			targetID := args[0]

			store, err := app.StorageFor(ctx, targetID)
			if err != nil {
				return err
			}

			target, err := store.Get(ctx, targetID)
			if err != nil {
				return fmt.Errorf("get target %s: %w", targetID, err)
			}

			epicID := targetID
			epic := target

			// Auto-wrap single task: create epic with task as child
			if target.Type != issuestorage.TypeEpic {
				wrapEpic := &issuestorage.Issue{
					Title:    fmt.Sprintf("Swarm: %s", target.Title),
					Type:     issuestorage.TypeEpic,
					Status:   issuestorage.StatusOpen,
					Priority: target.Priority,
					Owner:    target.Owner,
				}
				wrapID, err := store.Create(ctx, wrapEpic)
				if err != nil {
					return fmt.Errorf("create wrapper epic: %w", err)
				}
				if err := store.AddDependency(ctx, targetID, wrapID, issuestorage.DepTypeParentChild); err != nil {
					return fmt.Errorf("add parent-child: %w", err)
				}
				epicID = wrapID
				epic, err = store.Get(ctx, epicID)
				if err != nil {
					return fmt.Errorf("re-read epic: %w", err)
				}
			}

			// Validate
			children, err := graph.CollectMoleculeChildren(ctx, store, epicID)
			if err != nil {
				return fmt.Errorf("collect children: %w", err)
			}
			if len(children) == 0 {
				return fmt.Errorf("epic %s has no children", epicID)
			}
			_, err = graph.TopologicalWaves(children)
			if err != nil {
				return fmt.Errorf("not swarmable: %w", err)
			}

			// Create molecule issue
			mol := &issuestorage.Issue{
				Title:    fmt.Sprintf("swarm:%s", epic.Title),
				Type:     issuestorage.TypeMolecule,
				MolType:  issuestorage.MolTypeSwarm,
				Status:   issuestorage.StatusOpen,
				Priority: epic.Priority,
				Assignee: coordinator,
			}
			molID, err := store.Create(ctx, mol)
			if err != nil {
				return fmt.Errorf("create molecule: %w", err)
			}

			// Link molecule to epic via relates-to
			if err := store.AddDependency(ctx, molID, epicID, issuestorage.DepTypeRelatesTo); err != nil {
				return fmt.Errorf("link molecule to epic: %w", err)
			}

			result := SwarmCreateJSON{
				MoleculeID: molID,
				EpicID:     epicID,
				EpicTitle:  epic.Title,
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			fmt.Fprintf(app.Out, "%s Created swarm molecule: %s\n", app.SuccessColor("✓"), molID)
			fmt.Fprintf(app.Out, "  Epic: %s (%s)\n", epicID, epic.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&coordinator, "coordinator", "", "Assignee for the swarm molecule")

	return cmd
}

// newSwarmStatusCmd creates the "swarm status" subcommand.
func newSwarmStatusCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <epic-or-swarm-id>",
		Short: "Show live status of a swarm",
		Long: `Show the current status of a swarm by categorizing all children.

Accepts either an epic ID or a swarm molecule ID. If given a molecule with
MolType=swarm, follows the relates-to link to find the epic.

Examples:
  bd swarm status bl-abc123
  bd swarm status bl-mol456 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			inputID := args[0]

			store, err := app.StorageFor(ctx, inputID)
			if err != nil {
				return err
			}

			issue, err := store.Get(ctx, inputID)
			if err != nil {
				return fmt.Errorf("get %s: %w", inputID, err)
			}

			// Resolve: if molecule with MolType=swarm, follow relates-to to epic
			var epicID, moleculeID string
			if issue.Type == issuestorage.TypeMolecule && issue.MolType == issuestorage.MolTypeSwarm {
				moleculeID = issue.ID
				relatesTo := issuestorage.DepTypeRelatesTo
				deps := issue.DependencyIDs(&relatesTo)
				if len(deps) == 0 {
					return fmt.Errorf("swarm molecule %s has no relates-to link to an epic", inputID)
				}
				epicID = deps[0]
			} else {
				epicID = issue.ID
			}

			epic, err := store.Get(ctx, epicID)
			if err != nil {
				return fmt.Errorf("get epic %s: %w", epicID, err)
			}

			children, err := graph.CollectMoleculeChildren(ctx, store, epicID)
			if err != nil {
				return fmt.Errorf("collect children: %w", err)
			}

			closedSet, err := graph.BuildClosedSet(ctx, store)
			if err != nil {
				return fmt.Errorf("build closed set: %w", err)
			}

			classes := graph.ClassifySteps(children, closedSet)

			// Build child set for blocker resolution
			childSet := make(map[string]bool, len(children))
			for _, c := range children {
				childSet[c.ID] = true
			}

			var completed, active, ready, blocked int
			var childResults []SwarmChildJSON
			depType := issuestorage.DepTypeBlocks

			for _, c := range children {
				cat := string(classes[c.ID])
				switch classes[c.ID] {
				case graph.StepDone:
					completed++
				case graph.StepCurrent:
					active++
				case graph.StepReady:
					ready++
				case graph.StepBlocked:
					blocked++
				}

				child := SwarmChildJSON{
					ID:       c.ID,
					Title:    c.Title,
					Status:   string(c.Status),
					Category: cat,
				}

				// For blocked issues, report open blockers
				if classes[c.ID] == graph.StepBlocked {
					for _, depID := range c.DependencyIDs(&depType) {
						if childSet[depID] && !closedSet[depID] {
							child.Blockers = append(child.Blockers, depID)
						}
					}
				}

				childResults = append(childResults, child)
			}

			total := len(children)
			var progress float64
			if total > 0 {
				progress = float64(completed) / float64(total) * 100
			}

			result := SwarmStatusJSON{
				EpicID:     epicID,
				EpicTitle:  epic.Title,
				MoleculeID: moleculeID,
				Total:      total,
				Completed:  completed,
				Active:     active,
				Ready:      ready,
				Blocked:    blocked,
				Progress:   progress,
				Children:   childResults,
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(result)
			}

			// Text output
			fmt.Fprintf(app.Out, "Swarm: %s (%s)\n", epicID, epic.Title)
			if moleculeID != "" {
				fmt.Fprintf(app.Out, "Molecule: %s\n", moleculeID)
			}
			fmt.Fprintf(app.Out, "Progress: %.0f%% (%d/%d)\n", progress, completed, total)
			fmt.Fprintf(app.Out, "  Completed: %d | Active: %d | Ready: %d | Blocked: %d\n\n",
				completed, active, ready, blocked)

			for _, c := range childResults {
				marker := " "
				switch c.Category {
				case string(graph.StepDone):
					marker = "✓"
				case string(graph.StepCurrent):
					marker = "▶"
				case string(graph.StepReady):
					marker = "○"
				case string(graph.StepBlocked):
					marker = "✗"
				}
				fmt.Fprintf(app.Out, "  %s %s: %s [%s]\n", marker, c.ID, c.Title, c.Category)
				if len(c.Blockers) > 0 {
					fmt.Fprintf(app.Out, "    blocked by: %s\n", strings.Join(c.Blockers, ", "))
				}
			}

			return nil
		},
	}

	return cmd
}

// newSwarmListCmd creates the "swarm list" subcommand.
func newSwarmListCmd(provider *AppProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all swarm molecules with progress",
		Long: `List all swarm molecules and their progress.

Queries for issues with Type=molecule and MolType=swarm, then computes
status for each by following the relates-to link to its epic.

Examples:
  bd swarm list
  bd swarm list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			molType := issuestorage.TypeMolecule
			swarmMolType := issuestorage.MolTypeSwarm
			filter := &issuestorage.ListFilter{
				Type:    &molType,
				MolType: &swarmMolType,
			}

			molecules, err := app.Storage.List(ctx, filter)
			if err != nil {
				return fmt.Errorf("list swarm molecules: %w", err)
			}

			closedSet, err := graph.BuildClosedSet(ctx, app.Storage)
			if err != nil {
				return fmt.Errorf("build closed set: %w", err)
			}

			var entries []SwarmListEntryJSON
			relatesTo := issuestorage.DepTypeRelatesTo

			for _, mol := range molecules {
				deps := mol.DependencyIDs(&relatesTo)
				if len(deps) == 0 {
					continue
				}
				epicID := deps[0]

				epic, err := app.Storage.Get(ctx, epicID)
				if err != nil {
					continue // skip if epic not found
				}

				children, err := graph.CollectMoleculeChildren(ctx, app.Storage, epicID)
				if err != nil {
					continue
				}

				classes := graph.ClassifySteps(children, closedSet)

				var completed, activeCnt, readyCnt, blockedCnt int
				for _, status := range classes {
					switch status {
					case graph.StepDone:
						completed++
					case graph.StepCurrent:
						activeCnt++
					case graph.StepReady:
						readyCnt++
					case graph.StepBlocked:
						blockedCnt++
					}
				}

				total := len(children)
				var progress float64
				if total > 0 {
					progress = float64(completed) / float64(total) * 100
				}

				entries = append(entries, SwarmListEntryJSON{
					MoleculeID: mol.ID,
					EpicID:     epicID,
					EpicTitle:  epic.Title,
					Total:      total,
					Completed:  completed,
					Active:     activeCnt,
					Ready:      readyCnt,
					Blocked:    blockedCnt,
					Progress:   progress,
				})
			}

			if app.JSON {
				return json.NewEncoder(app.Out).Encode(SwarmListJSON{Swarms: entries})
			}

			if len(entries) == 0 {
				fmt.Fprintf(app.Out, "No swarm molecules found.\n")
				return nil
			}

			fmt.Fprintf(app.Out, "Swarms (%d):\n\n", len(entries))
			for _, e := range entries {
				fmt.Fprintf(app.Out, "  %s → %s: %s\n", e.MoleculeID, e.EpicID, e.EpicTitle)
				fmt.Fprintf(app.Out, "    Progress: %.0f%% (%d/%d) | Active: %d | Ready: %d | Blocked: %d\n\n",
					e.Progress, e.Completed, e.Total, e.Active, e.Ready, e.Blocked)
			}

			return nil
		},
	}

	return cmd
}

// swarmWarnings checks for structural issues in the epic's children.
func swarmWarnings(children []*issuestorage.Issue) []string {
	var warnings []string

	// Check for disconnected subgraphs: find issues that neither block nor are blocked
	// by any other issue in the set
	childSet := make(map[string]bool, len(children))
	for _, c := range children {
		childSet[c.ID] = true
	}

	depType := issuestorage.DepTypeBlocks
	hasBlockEdge := make(map[string]bool)
	for _, c := range children {
		for _, depID := range c.DependencyIDs(&depType) {
			if childSet[depID] {
				hasBlockEdge[c.ID] = true
				hasBlockEdge[depID] = true
			}
		}
	}

	var disconnected []string
	for _, c := range children {
		if !hasBlockEdge[c.ID] && len(children) > 1 {
			disconnected = append(disconnected, c.ID)
		}
	}
	if len(disconnected) > 0 && len(disconnected) < len(children) {
		warnings = append(warnings, fmt.Sprintf("%d issue(s) have no dependency edges: %s",
			len(disconnected), strings.Join(disconnected, ", ")))
	}

	return warnings
}
