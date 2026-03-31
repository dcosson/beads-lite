package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"beads-lite/internal/graph"
	"beads-lite/internal/issuestorage"

	"github.com/spf13/cobra"
)

// GraphTaskJSON represents a task node in graph JSON output.
type GraphTaskJSON struct {
	Blocks             []string `json:"blocks"`
	DirectBlockers     []string `json:"direct_blockers"`
	EffectivelyBlocked bool     `json:"effectively_blocked"`
	ID                 string   `json:"id"`
	InheritedBlockers  []string `json:"inherited_blockers"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
}

// GraphGroupJSON represents one parent group in graph JSON output.
type GraphGroupJSON struct {
	BlockedBy    []string        `json:"blocked_by"`
	ParentID     string          `json:"parent_id"`
	ParentStatus string          `json:"parent_status"`
	ParentTitle  string          `json:"parent_title"`
	ParentType   string          `json:"parent_type"`
	Tasks        []GraphTaskJSON `json:"tasks"`
}

// GraphWaveJSON represents a wave in graph JSON output.
type GraphWaveJSON struct {
	Issues []string `json:"issues"`
	Wave   int      `json:"wave"`
}

// GraphOutputJSON is the full graph JSON payload.
type GraphOutputJSON struct {
	CascadeParentBlocking bool             `json:"cascade_parent_blocking"`
	Groups                []GraphGroupJSON `json:"groups"`
	Standalone            []GraphTaskJSON  `json:"standalone"`
	Waves                 []GraphWaveJSON  `json:"waves,omitempty"`
}

type graphGroup struct {
	ParentID string
	Parent   *issuestorage.Issue
	TaskIDs  []string
}

func newGraphCmd(provider *AppProvider) *cobra.Command {
	var waves bool

	cmd := &cobra.Command{
		Use:   "graph [parent-id]",
		Short: "Render dependency graph as grouped trees",
		Long: `Render dependency graph as grouped trees with back-references.

Without an argument, renders all open tasks grouped by immediate parent.
With a parent ID, renders that parent's descendant scope grouped by immediate parent.

Use --waves to include cross-parent wave grouping.
Use --json to emit structured output.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := provider.Get()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			cascade := cascadeEnabled(app)

			rootID := ""
			if len(args) == 1 {
				resolved, err := resolveIssue(app.Storage, ctx, args[0])
				if err != nil {
					if err == issuestorage.ErrNotFound {
						return fmt.Errorf("no issue found matching %q", args[0])
					}
					return err
				}
				rootID = resolved.ID
			}

			allIssues, err := collectGraphIssues(ctx, app.Storage, rootID)
			if err != nil {
				return err
			}
			leafTasks := selectLeafTasks(allIssues)
			if len(leafTasks) == 0 {
				if app.JSON {
					return json.NewEncoder(app.Out).Encode(GraphOutputJSON{
						CascadeParentBlocking: cascade,
						Groups:                []GraphGroupJSON{},
						Standalone:            []GraphTaskJSON{},
					})
				}
				fmt.Fprintln(app.Out, "No issues to graph.")
				return nil
			}

			taskByID := make(map[string]*issuestorage.Issue, len(leafTasks))
			for _, task := range leafTasks {
				taskByID[task.ID] = task
			}

			closedSet, err := graph.BuildClosedSet(ctx, app.Storage)
			if err != nil {
				return err
			}
			blockersByID, err := graph.EffectiveBlockersBatch(ctx, app.Storage, leafTasks, closedSet, cascade)
			if err != nil {
				return err
			}

			groups, standalone := buildGraphGroups(ctx, app.Storage, leafTasks)
			parentOrder := topologicalParentOrder(groups, closedSet)

			var waveData []GraphWaveJSON
			if waves || app.JSON {
				ws, _, err := graph.TopologicalWavesAcrossParents(ctx, app.Storage, rootID, cascade)
				if err != nil {
					return err
				}
				waveData = toGraphWaveJSON(ws)
			}

			if app.JSON {
				return outputGraphJSON(app, taskByID, groups, parentOrder, standalone, blockersByID, closedSet, cascade, waveData)
			}

			outputGraphText(app, taskByID, groups, parentOrder, standalone, blockersByID, closedSet)
			if waves {
				printWavesText(app, waveData)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&waves, "waves", false, "Show cross-parent wave grouping")
	return cmd
}

func collectGraphIssues(ctx context.Context, store issuestorage.IssueStore, rootID string) ([]*issuestorage.Issue, error) {
	if rootID != "" {
		descendants, err := graph.CollectMoleculeChildren(ctx, store, rootID)
		if err != nil {
			return nil, fmt.Errorf("collect descendants of %s: %w", rootID, err)
		}
		var out []*issuestorage.Issue
		for _, issue := range descendants {
			if issue.Ephemeral {
				continue
			}
			out = append(out, issue)
		}
		return out, nil
	}

	issues, err := store.List(ctx, &issuestorage.ListFilter{Statuses: []issuestorage.Status{issuestorage.StatusOpen}})
	if err != nil {
		return nil, fmt.Errorf("list open issues: %w", err)
	}
	var out []*issuestorage.Issue
	for _, issue := range issues {
		if issue.Ephemeral {
			continue
		}
		out = append(out, issue)
	}
	return out, nil
}

func selectLeafTasks(allIssues []*issuestorage.Issue) []*issuestorage.Issue {
	if len(allIssues) == 0 {
		return nil
	}
	byID := make(map[string]*issuestorage.Issue, len(allIssues))
	for _, issue := range allIssues {
		byID[issue.ID] = issue
	}

	var leaves []*issuestorage.Issue
	for _, issue := range allIssues {
		hasChildInScope := false
		for _, childID := range issue.Children() {
			if _, ok := byID[childID]; ok {
				hasChildInScope = true
				break
			}
		}
		if !hasChildInScope {
			leaves = append(leaves, issue)
		}
	}
	return leaves
}

func buildGraphGroups(ctx context.Context, store issuestorage.IssueStore, leafTasks []*issuestorage.Issue) (map[string]*graphGroup, []string) {
	groups := make(map[string]*graphGroup)
	var standalone []string

	for _, task := range leafTasks {
		if task.Parent == "" {
			standalone = append(standalone, task.ID)
			continue
		}
		g, ok := groups[task.Parent]
		if !ok {
			var parent *issuestorage.Issue
			if p, err := store.Get(ctx, task.Parent); err == nil {
				parent = p
			}
			g = &graphGroup{ParentID: task.Parent, Parent: parent}
			groups[task.Parent] = g
		}
		g.TaskIDs = append(g.TaskIDs, task.ID)
	}

	for _, g := range groups {
		sort.Strings(g.TaskIDs)
	}
	sort.Strings(standalone)
	return groups, standalone
}

func topologicalParentOrder(groups map[string]*graphGroup, closedSet map[string]bool) []string {
	if len(groups) == 0 {
		return nil
	}

	inDegree := make(map[string]int, len(groups))
	outEdges := make(map[string][]string, len(groups))
	for id := range groups {
		inDegree[id] = 0
	}

	depType := issuestorage.DepTypeBlocks
	for id, g := range groups {
		if g.Parent == nil {
			continue
		}
		for _, depID := range g.Parent.DependencyIDs(&depType) {
			if closedSet[depID] {
				continue
			}
			if _, ok := groups[depID]; !ok {
				continue
			}
			outEdges[depID] = append(outEdges[depID], id)
			inDegree[id]++
		}
	}

	for from := range outEdges {
		sort.Strings(outEdges[from])
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var ordered []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, id)

		for _, next := range outEdges[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
		sort.Strings(queue)
	}

	if len(ordered) == len(groups) {
		return ordered
	}

	seen := make(map[string]bool, len(ordered))
	for _, id := range ordered {
		seen[id] = true
	}
	var remaining []string
	for id := range groups {
		if !seen[id] {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func outputGraphText(
	app *App,
	taskByID map[string]*issuestorage.Issue,
	groups map[string]*graphGroup,
	parentOrder []string,
	standalone []string,
	blockersByID map[string]*graph.EffectiveBlockersResult,
	closedSet map[string]bool,
) {
	printedAny := false
	for _, parentID := range parentOrder {
		g := groups[parentID]
		if g == nil || len(g.TaskIDs) == 0 {
			continue
		}
		if printedAny {
			fmt.Fprintln(app.Out)
		}
		printedAny = true
		fmt.Fprintln(app.Out, formatParentHeader(g, closedSet))
		renderTaskTree(app, g.TaskIDs, taskByID, blockersByID)
	}

	if len(standalone) > 0 {
		if printedAny {
			fmt.Fprintln(app.Out)
		}
		fmt.Fprintln(app.Out, "Standalone (no parent)")
		renderTaskTree(app, standalone, taskByID, blockersByID)
	}
}

func outputGraphJSON(
	app *App,
	taskByID map[string]*issuestorage.Issue,
	groups map[string]*graphGroup,
	parentOrder []string,
	standalone []string,
	blockersByID map[string]*graph.EffectiveBlockersResult,
	closedSet map[string]bool,
	cascade bool,
	waves []GraphWaveJSON,
) error {
	out := GraphOutputJSON{
		CascadeParentBlocking: cascade,
		Groups:                []GraphGroupJSON{},
		Standalone:            []GraphTaskJSON{},
		Waves:                 waves,
	}

	for _, parentID := range parentOrder {
		g := groups[parentID]
		if g == nil {
			continue
		}
		groupJSON := GraphGroupJSON{
			BlockedBy: parentDirectBlockers(g.Parent, closedSet),
			ParentID:  g.ParentID,
			Tasks:     make([]GraphTaskJSON, 0, len(g.TaskIDs)),
		}
		if g.Parent != nil {
			groupJSON.ParentTitle = g.Parent.Title
			groupJSON.ParentType = string(g.Parent.Type)
			groupJSON.ParentStatus = string(g.Parent.Status)
		}
		for _, taskID := range g.TaskIDs {
			task := taskByID[taskID]
			if task == nil {
				continue
			}
			groupJSON.Tasks = append(groupJSON.Tasks, toGraphTaskJSON(task, g.TaskIDs, blockersByID[taskID]))
		}
		out.Groups = append(out.Groups, groupJSON)
	}

	for _, taskID := range standalone {
		task := taskByID[taskID]
		if task == nil {
			continue
		}
		out.Standalone = append(out.Standalone, toGraphTaskJSON(task, standalone, blockersByID[taskID]))
	}

	return json.NewEncoder(app.Out).Encode(out)
}

func renderTaskTree(
	app *App,
	groupTaskIDs []string,
	taskByID map[string]*issuestorage.Issue,
	blockersByID map[string]*graph.EffectiveBlockersResult,
) {
	taskSet := make(map[string]bool, len(groupTaskIDs))
	for _, id := range groupTaskIDs {
		taskSet[id] = true
	}

	childrenByID := make(map[string][]string)
	inDegree := make(map[string]int, len(groupTaskIDs))
	depType := issuestorage.DepTypeBlocks
	for _, id := range groupTaskIDs {
		inDegree[id] = 0
	}
	for _, id := range groupTaskIDs {
		issue := taskByID[id]
		if issue == nil {
			continue
		}
		for _, dep := range issue.Dependents {
			if dep.Type != depType || !taskSet[dep.ID] {
				continue
			}
			childrenByID[id] = append(childrenByID[id], dep.ID)
			inDegree[dep.ID]++
		}
	}
	for id := range childrenByID {
		sort.Strings(childrenByID[id])
	}

	var roots []string
	for _, id := range groupTaskIDs {
		if inDegree[id] == 0 {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)
	if len(roots) == 0 {
		roots = append([]string{}, groupTaskIDs...)
		sort.Strings(roots)
	}

	seen := make(map[string]bool)
	var renderNode func(id string, prefix string, isLast bool)
	renderNode = func(id string, prefix string, isLast bool) {
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		if seen[id] {
			fmt.Fprintf(app.Out, "%s%s↗ %s  (see above)\n", prefix, connector, id)
			return
		}
		seen[id] = true

		issue := taskByID[id]
		if issue == nil {
			fmt.Fprintf(app.Out, "%s%s%s\n", prefix, connector, id)
			return
		}

		annotation := taskAnnotation(blockersByID[id])
		fmt.Fprintf(app.Out, "%s%s%s %s  %s%s\n",
			prefix,
			connector,
			taskStatusIcon(issue, blockersByID[id]),
			issue.ID,
			issue.Title,
			annotation,
		)

		children := childrenByID[id]
		for i, childID := range children {
			renderNode(childID, childPrefix, i == len(children)-1)
		}
	}

	for i, rootID := range roots {
		renderNode(rootID, "", i == len(roots)-1)
	}

	var leftovers []string
	for _, id := range groupTaskIDs {
		if !seen[id] {
			leftovers = append(leftovers, id)
		}
	}
	sort.Strings(leftovers)
	for _, id := range leftovers {
		renderNode(id, "", true)
	}
}

func formatParentHeader(g *graphGroup, closedSet map[string]bool) string {
	title := g.ParentID
	status := "open"
	icon := "○"
	blockedBy := []string{}

	if g.Parent != nil {
		title = g.Parent.Title
		status = string(g.Parent.Status)
		blockedBy = parentDirectBlockers(g.Parent, closedSet)
		icon = parentStatusIcon(g.Parent, len(blockedBy) > 0)
	}

	if len(blockedBy) > 0 {
		return fmt.Sprintf("%s [%s parent blocked by: %s] - %s", g.ParentID, icon, strings.Join(blockedBy, ", "), title)
	}
	return fmt.Sprintf("%s [%s %s] - %s", g.ParentID, icon, status, title)
}

func parentDirectBlockers(parent *issuestorage.Issue, closedSet map[string]bool) []string {
	if parent == nil {
		return nil
	}
	depType := issuestorage.DepTypeBlocks
	var out []string
	for _, depID := range parent.DependencyIDs(&depType) {
		if !closedSet[depID] {
			out = append(out, depID)
		}
	}
	sort.Strings(out)
	return out
}

func parentStatusIcon(issue *issuestorage.Issue, parentBlocked bool) string {
	if issue == nil {
		return "○"
	}
	if issue.Status == issuestorage.StatusClosed {
		return "✓"
	}
	if issue.Status == issuestorage.StatusInProgress {
		return "◐"
	}
	if issue.Status == issuestorage.StatusBlocked || parentBlocked {
		return "●"
	}
	return "○"
}

func taskStatusIcon(issue *issuestorage.Issue, blockers *graph.EffectiveBlockersResult) string {
	if issue.Status == issuestorage.StatusClosed {
		return "✓"
	}
	if issue.Status == issuestorage.StatusInProgress {
		return "◐"
	}
	if issue.Status == issuestorage.StatusBlocked || (blockers != nil && blockers.HasBlockers()) {
		return "●"
	}
	return "○"
}

func taskAnnotation(blockers *graph.EffectiveBlockersResult) string {
	if blockers == nil || !blockers.HasBlockers() {
		return ""
	}
	parts := []string{}
	if len(blockers.Inherited) > 0 {
		parts = append(parts, "[parent blocked]")
	}
	if len(blockers.Direct) > 0 {
		direct := append([]string{}, blockers.Direct...)
		sort.Strings(direct)
		parts = append(parts, fmt.Sprintf("[blocked by: %s]", strings.Join(direct, ", ")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + strings.Join(parts, " ")
}

func toGraphTaskJSON(issue *issuestorage.Issue, groupTaskIDs []string, blockers *graph.EffectiveBlockersResult) GraphTaskJSON {
	taskSet := make(map[string]bool, len(groupTaskIDs))
	for _, id := range groupTaskIDs {
		taskSet[id] = true
	}

	depType := issuestorage.DepTypeBlocks
	blocks := []string{}
	for _, dep := range issue.Dependents {
		if dep.Type == depType && taskSet[dep.ID] {
			blocks = append(blocks, dep.ID)
		}
	}
	sort.Strings(blocks)

	direct := []string{}
	inherited := []string{}
	effective := issue.Status == issuestorage.StatusBlocked
	if blockers != nil {
		direct = append(direct, blockers.Direct...)
		sort.Strings(direct)

		seen := make(map[string]bool)
		for _, ib := range blockers.Inherited {
			if !seen[ib.BlockerID] {
				seen[ib.BlockerID] = true
				inherited = append(inherited, ib.BlockerID)
			}
		}
		sort.Strings(inherited)
		effective = effective || blockers.HasBlockers()
	}

	return GraphTaskJSON{
		Blocks:             blocks,
		DirectBlockers:     direct,
		EffectivelyBlocked: effective,
		ID:                 issue.ID,
		InheritedBlockers:  inherited,
		Status:             string(issue.Status),
		Title:              issue.Title,
	}
}

func toGraphWaveJSON(waves [][]string) []GraphWaveJSON {
	if len(waves) == 0 {
		return nil
	}
	out := make([]GraphWaveJSON, len(waves))
	for i, wave := range waves {
		copyWave := append([]string{}, wave...)
		sort.Strings(copyWave)
		out[i] = GraphWaveJSON{Wave: i, Issues: copyWave}
	}
	return out
}

func printWavesText(app *App, waves []GraphWaveJSON) {
	if len(waves) == 0 {
		return
	}
	fmt.Fprintln(app.Out)
	fmt.Fprintln(app.Out, "Waves")
	for _, wave := range waves {
		fmt.Fprintf(app.Out, "  Wave %d: %s\n", wave.Wave, strings.Join(wave.Issues, ", "))
	}
}
