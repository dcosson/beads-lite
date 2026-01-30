// Package graph provides stateless graph traversal functions for molecule workflows.
// All functions take (ctx, storage.Storage, ...) with no struct state.
// Import chain: cmd → meow → graph → storage.
package graph

import (
	"context"
	"fmt"

	"beads-lite/internal/storage"
)

// StepStatus classifies a molecule step's current state.
type StepStatus string

const (
	StepDone    StepStatus = "done"
	StepCurrent StepStatus = "current"
	StepReady   StepStatus = "ready"
	StepBlocked StepStatus = "blocked"
	StepPending StepStatus = "pending"
)

// FindMoleculeRoot walks the parent chain (via DepParentChild dependencies) to
// find the root epic of a molecule. The root is an issue with no parent.
func FindMoleculeRoot(ctx context.Context, store storage.Storage, issueID string) (*storage.Issue, error) {
	visited := make(map[string]bool)
	currentID := issueID

	for {
		if visited[currentID] {
			return nil, fmt.Errorf("cycle detected in parent chain at %s", currentID)
		}
		visited[currentID] = true

		issue, err := store.Get(ctx, currentID)
		if err != nil {
			return nil, fmt.Errorf("get issue %s: %w", currentID, err)
		}

		if issue.Parent == "" {
			return issue, nil
		}
		currentID = issue.Parent
	}
}

// CollectMoleculeChildren recursively collects all descendants of rootID via
// DepParentChild dependencies. Returns all descendants (not including the root).
func CollectMoleculeChildren(ctx context.Context, store storage.Storage, rootID string) ([]*storage.Issue, error) {
	root, err := store.Get(ctx, rootID)
	if err != nil {
		return nil, fmt.Errorf("get root %s: %w", rootID, err)
	}

	var result []*storage.Issue
	queue := root.Children()

	visited := map[string]bool{rootID: true}
	for len(queue) > 0 {
		childID := queue[0]
		queue = queue[1:]

		if visited[childID] {
			continue
		}
		visited[childID] = true

		child, err := store.Get(ctx, childID)
		if err != nil {
			return nil, fmt.Errorf("get child %s: %w", childID, err)
		}
		result = append(result, child)
		queue = append(queue, child.Children()...)
	}
	return result, nil
}

// TopologicalOrder sorts issues using Kahn's algorithm on DepBlocks edges.
// Returns issues in dependency-respecting order. Returns an error if cycles exist.
func TopologicalOrder(children []*storage.Issue) ([]*storage.Issue, error) {
	if len(children) == 0 {
		return nil, nil
	}

	// Build ID → issue lookup and adjacency
	byID := make(map[string]*storage.Issue, len(children))
	childSet := make(map[string]bool, len(children))
	for _, c := range children {
		byID[c.ID] = c
		childSet[c.ID] = true
	}

	// inDegree: how many DepBlocks edges point into this node (within the child set)
	inDegree := make(map[string]int, len(children))
	// outEdges: node → list of nodes it blocks (i.e. that depend on it)
	outEdges := make(map[string][]string, len(children))

	for _, c := range children {
		if _, ok := inDegree[c.ID]; !ok {
			inDegree[c.ID] = 0
		}
		// c.Dependencies with type "blocks" means c depends on (is blocked by) dep.ID
		depType := storage.DepTypeBlocks
		for _, depID := range c.DependencyIDs(&depType) {
			if !childSet[depID] {
				continue // skip deps outside the molecule
			}
			outEdges[depID] = append(outEdges[depID], c.ID)
			inDegree[c.ID]++
		}
	}

	// Kahn's: start with nodes that have no incoming edges
	var queue []string
	for _, c := range children {
		if inDegree[c.ID] == 0 {
			queue = append(queue, c.ID)
		}
	}

	var result []*storage.Issue
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		result = append(result, byID[id])

		for _, next := range outEdges[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(result) != len(children) {
		return nil, fmt.Errorf("cycle detected in dependency graph: sorted %d of %d issues", len(result), len(children))
	}
	return result, nil
}

// FindReadySteps returns issues that are open and whose blocking dependencies
// are all in the closedSet.
func FindReadySteps(children []*storage.Issue, closedSet map[string]bool) []*storage.Issue {
	childSet := make(map[string]bool, len(children))
	for _, c := range children {
		childSet[c.ID] = true
	}

	var ready []*storage.Issue
	depType := storage.DepTypeBlocks
	for _, c := range children {
		if c.Status == storage.StatusClosed {
			continue
		}
		blocked := false
		for _, depID := range c.DependencyIDs(&depType) {
			if childSet[depID] && !closedSet[depID] {
				blocked = true
				break
			}
		}
		if !blocked {
			ready = append(ready, c)
		}
	}
	return ready
}

// FindNextStep returns the first ready step after currentID in topological order.
// An issue is ready if it is not closed and all its blocking deps are in closedSet.
func FindNextStep(ordered []*storage.Issue, currentID string, closedSet map[string]bool) *storage.Issue {
	childSet := make(map[string]bool, len(ordered))
	for _, c := range ordered {
		childSet[c.ID] = true
	}

	pastCurrent := false
	depType := storage.DepTypeBlocks
	for _, c := range ordered {
		if c.ID == currentID {
			pastCurrent = true
			continue
		}
		if !pastCurrent {
			continue
		}
		if c.Status == storage.StatusClosed {
			continue
		}
		blocked := false
		for _, depID := range c.DependencyIDs(&depType) {
			if childSet[depID] && !closedSet[depID] {
				blocked = true
				break
			}
		}
		if !blocked {
			return c
		}
	}
	return nil
}

// ClassifySteps classifies each child issue as done, current, ready, blocked, or pending.
func ClassifySteps(children []*storage.Issue, closedSet map[string]bool) map[string]StepStatus {
	childSet := make(map[string]bool, len(children))
	for _, c := range children {
		childSet[c.ID] = true
	}

	result := make(map[string]StepStatus, len(children))
	depType := storage.DepTypeBlocks

	for _, c := range children {
		if c.Status == storage.StatusClosed {
			result[c.ID] = StepDone
			continue
		}
		if c.Status == storage.StatusInProgress {
			result[c.ID] = StepCurrent
			continue
		}

		// Check if all blocking deps are resolved
		blocked := false
		for _, depID := range c.DependencyIDs(&depType) {
			if childSet[depID] && !closedSet[depID] {
				blocked = true
				break
			}
		}
		if blocked {
			result[c.ID] = StepBlocked
		} else {
			result[c.ID] = StepReady
		}
	}
	return result
}

// BuildClosedSet queries all closed issues and returns their IDs as a set.
func BuildClosedSet(ctx context.Context, store storage.Storage) (map[string]bool, error) {
	status := storage.StatusClosed
	closed, err := store.List(ctx, &storage.ListFilter{Status: &status})
	if err != nil {
		return nil, fmt.Errorf("list closed issues: %w", err)
	}
	set := make(map[string]bool, len(closed))
	for _, issue := range closed {
		set[issue.ID] = true
	}
	return set, nil
}
