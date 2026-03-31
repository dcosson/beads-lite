package graph

import (
	"context"
	"fmt"
	"log"

	"beads-lite/internal/issuestorage"
)

// TopologicalWavesAcrossParents computes parallelizable waves across multiple
// parent issues, respecting parent-level blocking relationships.
//
// If rootID is non-empty, collects all descendants of that root.
// If rootID is empty, operates on all open non-ephemeral tasks.
//
// When cascade is true, parent-level blocks are translated into synthetic
// task-level edges: if Parent B blocks Parent A, every root task in A gets a
// synthetic dependency on every sink task in B (the blocking frontier).
//
// Returns waves where wave[0] can start immediately, wave[1] after wave[0]
// completes, etc. Also returns a lookup map of ID → Issue for all collected tasks.
func TopologicalWavesAcrossParents(
	ctx context.Context,
	store issuestorage.IssueStore,
	rootID string,
	cascade bool,
) ([][]string, map[string]*issuestorage.Issue, error) {
	// 1. Collect all issues in scope
	var allIssues []*issuestorage.Issue
	if rootID != "" {
		descendants, err := CollectMoleculeChildren(ctx, store, rootID)
		if err != nil {
			return nil, nil, fmt.Errorf("collecting descendants of %s: %w", rootID, err)
		}
		allIssues = descendants
	} else {
		all, err := store.List(ctx, &issuestorage.ListFilter{Statuses: []issuestorage.Status{issuestorage.StatusOpen}})
		if err != nil {
			return nil, nil, fmt.Errorf("listing open issues: %w", err)
		}
		// Filter out ephemeral issues
		for _, issue := range all {
			if !issue.Ephemeral {
				allIssues = append(allIssues, issue)
			}
		}
	}

	if len(allIssues) == 0 {
		return nil, nil, nil
	}

	// Build lookup
	byID := make(map[string]*issuestorage.Issue, len(allIssues))
	for _, issue := range allIssues {
		byID[issue.ID] = issue
	}

	// Separate leaf tasks (no children in set) from parents (have children in set)
	var leafTasks []*issuestorage.Issue
	var parents []*issuestorage.Issue

	for _, issue := range allIssues {
		hasChildInSet := false
		for _, childID := range issue.Children() {
			if _, ok := byID[childID]; ok {
				hasChildInSet = true
				break
			}
		}
		if hasChildInSet {
			parents = append(parents, issue)
		} else {
			leafTasks = append(leafTasks, issue)
		}
	}

	if len(leafTasks) == 0 {
		return nil, byID, nil
	}

	// Build leaf task set
	leafSet := make(map[string]bool, len(leafTasks))
	for _, t := range leafTasks {
		leafSet[t.ID] = true
	}

	// 3. If cascade, inject synthetic blocking edges
	if cascade {
		injectSyntheticEdges(parents, leafTasks, leafSet, byID)
	}

	// 4. Run TopologicalWaves on the leaf tasks (with any synthetic edges)
	waves, err := TopologicalWaves(leafTasks)
	if err != nil {
		return nil, nil, fmt.Errorf("topological waves: %w", err)
	}

	return waves, byID, nil
}

// injectSyntheticEdges adds synthetic DepTypeBlocks dependencies to model
// parent-level blocking relationships at the leaf task level.
func injectSyntheticEdges(
	parents []*issuestorage.Issue,
	leafTasks []*issuestorage.Issue,
	leafSet map[string]bool,
	byID map[string]*issuestorage.Issue,
) {
	// Build parent→leaf-children mapping for quick lookup
	parentLeafChildren := make(map[string][]string)
	for _, t := range leafTasks {
		if t.Parent != "" {
			parentLeafChildren[t.Parent] = append(parentLeafChildren[t.Parent], t.ID)
		}
	}

	// Also collect all leaf descendants of each parent (not just direct children)
	// for nested parent hierarchies
	parentAllLeaves := make(map[string][]string)
	for _, parent := range parents {
		parentAllLeaves[parent.ID] = collectLeafDescendants(parent.ID, byID, leafSet)
	}

	depType := issuestorage.DepTypeBlocks
	for _, parent := range parents {
		for _, dep := range parent.Dependencies {
			if dep.Type != depType {
				continue
			}
			blockerID := dep.ID

			// Compute blocking frontier
			frontier := blockingFrontier(blockerID, leafSet, byID, parentAllLeaves)
			if len(frontier) == 0 {
				continue
			}

			// Find root tasks under the blocked parent:
			// leaf descendants of this parent that have no intra-set blockers
			blockedLeaves := parentAllLeaves[parent.ID]
			if len(blockedLeaves) == 0 {
				continue
			}
			blockedRoots := findRootTasks(blockedLeaves, leafSet, byID)

			// Add synthetic edges
			for _, rootTask := range blockedRoots {
				rootIssue := byID[rootTask]
				for _, frontierTask := range frontier {
					if rootTask == frontierTask {
						continue // don't self-reference
					}
					// Check if already has this dependency
					if !rootIssue.HasDependency(frontierTask) {
						rootIssue.Dependencies = append(rootIssue.Dependencies, issuestorage.Dependency{
							ID:   frontierTask,
							Type: depType,
						})
					}
				}
			}
		}
	}
}

// collectLeafDescendants returns all leaf task IDs that are descendants of the
// given parentID (at any depth). A leaf task is one in leafSet.
func collectLeafDescendants(parentID string, byID map[string]*issuestorage.Issue, leafSet map[string]bool) []string {
	var result []string
	visited := map[string]bool{parentID: true}
	queue := []string{parentID}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		issue, ok := byID[id]
		if !ok {
			continue
		}

		for _, childID := range issue.Children() {
			if visited[childID] {
				continue
			}
			visited[childID] = true

			if leafSet[childID] {
				result = append(result, childID)
			}
			// Also recurse into children that are parents
			if _, ok := byID[childID]; ok {
				queue = append(queue, childID)
			}
		}
	}

	return result
}

// blockingFrontier computes the set of leaf tasks that must complete last within
// the blocker's subtree. These are the dependency-graph sinks: leaf tasks that
// no other leaf task in the set depends on.
func blockingFrontier(
	blockerID string,
	leafSet map[string]bool,
	byID map[string]*issuestorage.Issue,
	parentAllLeaves map[string][]string,
) []string {
	// If blocker is itself a leaf task, return it directly
	if leafSet[blockerID] {
		return []string{blockerID}
	}

	// If blocker is a parent, get its leaf descendants
	descendants, ok := parentAllLeaves[blockerID]
	if !ok || len(descendants) == 0 {
		// Blocker is outside the task set entirely
		if _, inSet := byID[blockerID]; !inSet {
			log.Printf("WARNING: graph.blockingFrontier: blocker %s is outside the task set, skipping", blockerID)
		}
		return nil
	}

	// Find sinks: descendants with no intra-set DepTypeBlocks dependents
	// A sink is a task where no other task in the descendant set lists it
	// as a DepTypeBlocks dependency (nothing is waiting for it to unblock).
	descendantSet := make(map[string]bool, len(descendants))
	for _, id := range descendants {
		descendantSet[id] = true
	}

	// Build set of tasks that are depended on (i.e., block something else)
	depType := issuestorage.DepTypeBlocks
	isBlocker := make(map[string]bool)
	for _, id := range descendants {
		issue := byID[id]
		if issue == nil {
			continue
		}
		for _, depID := range issue.DependencyIDs(&depType) {
			if descendantSet[depID] {
				isBlocker[depID] = true
			}
		}
	}

	// Sinks are descendants that are NOT depended on by any other descendant
	var sinks []string
	for _, id := range descendants {
		if !isBlocker[id] {
			sinks = append(sinks, id)
		}
	}

	// If no sinks found (shouldn't happen if no cycles), fall back to all descendants
	if len(sinks) == 0 {
		return descendants
	}

	return sinks
}

// findRootTasks returns the leaf tasks from the given set that have no
// intra-leaf-set DepTypeBlocks blockers. These are the "entry point" tasks.
func findRootTasks(taskIDs []string, leafSet map[string]bool, byID map[string]*issuestorage.Issue) []string {
	taskIDSet := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		taskIDSet[id] = true
	}

	depType := issuestorage.DepTypeBlocks
	var roots []string
	for _, id := range taskIDs {
		issue := byID[id]
		if issue == nil {
			continue
		}
		hasBlocker := false
		for _, depID := range issue.DependencyIDs(&depType) {
			if taskIDSet[depID] {
				hasBlocker = true
				break
			}
		}
		if !hasBlocker {
			roots = append(roots, id)
		}
	}
	return roots
}
