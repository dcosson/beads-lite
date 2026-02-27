package graph

import (
	"context"
	"fmt"
	"log"

	"beads-lite/internal/issuestorage"
)

// InheritedBlocker represents a blocking constraint inherited from an ancestor.
type InheritedBlocker struct {
	AncestorID string // the parent/grandparent that has the blocking dep
	BlockerID  string // the issue blocking that ancestor
}

// EffectiveBlockersResult holds both direct and inherited blocking info.
type EffectiveBlockersResult struct {
	Direct    []string           // direct DepTypeBlocks dependency IDs that are unclosed
	Inherited []InheritedBlocker // blocking constraints from ancestors
}

// HasBlockers returns true if there are any direct or inherited blockers.
func (r *EffectiveBlockersResult) HasBlockers() bool {
	return len(r.Direct) > 0 || len(r.Inherited) > 0
}

// AllBlockerIDs returns a deduplicated set of all blocker IDs (direct and inherited).
func (r *EffectiveBlockersResult) AllBlockerIDs() []string {
	seen := make(map[string]bool, len(r.Direct)+len(r.Inherited))
	var ids []string
	for _, id := range r.Direct {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, ib := range r.Inherited {
		if !seen[ib.BlockerID] {
			seen[ib.BlockerID] = true
			ids = append(ids, ib.BlockerID)
		}
	}
	return ids
}

// EffectiveBlockers returns all blocking constraints on an issue:
//   - Direct: the issue's own unclosed DepTypeBlocks dependencies
//   - Inherited: unclosed DepTypeBlocks dependencies from any ancestor in the parent chain
//
// When cascade is false, Inherited is always empty.
//
// Error handling is resilient: if an ancestor cannot be loaded, a warning is
// logged and traversal stops rather than failing the entire call.
func EffectiveBlockers(
	ctx context.Context,
	store issuestorage.IssueGetter,
	issue *issuestorage.Issue,
	closedSet map[string]bool,
	cascade bool,
) (*EffectiveBlockersResult, error) {
	result := &EffectiveBlockersResult{}

	// Collect direct blockers
	depType := issuestorage.DepTypeBlocks
	for _, depID := range issue.DependencyIDs(&depType) {
		if !closedSet[depID] {
			result.Direct = append(result.Direct, depID)
		}
	}

	if !cascade {
		return result, nil
	}

	// Walk parent chain for inherited blockers
	currentID := issue.Parent
	visited := map[string]bool{issue.ID: true}

	for currentID != "" {
		if visited[currentID] {
			// Cycle guard — stop traversal
			break
		}
		visited[currentID] = true

		ancestor, err := store.Get(ctx, currentID)
		if err != nil {
			// Resilient: log warning and stop traversal
			log.Printf("WARNING: graph.EffectiveBlockers: cannot load ancestor %s: %v", currentID, err)
			break
		}

		for _, depID := range ancestor.DependencyIDs(&depType) {
			if !closedSet[depID] {
				result.Inherited = append(result.Inherited, InheritedBlocker{
					AncestorID: currentID,
					BlockerID:  depID,
				})
			}
		}

		currentID = ancestor.Parent
	}

	return result, nil
}

// IsEffectivelyBlocked returns true if the issue has any unclosed direct
// or inherited blocking constraints.
func IsEffectivelyBlocked(
	ctx context.Context,
	store issuestorage.IssueGetter,
	issue *issuestorage.Issue,
	closedSet map[string]bool,
	cascade bool,
) (bool, error) {
	result, err := EffectiveBlockers(ctx, store, issue, closedSet, cascade)
	if err != nil {
		return false, err
	}
	return result.HasBlockers(), nil
}

// EffectiveBlockersBatch computes EffectiveBlockers for multiple issues,
// sharing the same closedSet and store. Returns a map from issue ID to result.
// This is more efficient than calling EffectiveBlockers individually when
// processing many issues, because ancestor lookups are cached across calls.
func EffectiveBlockersBatch(
	ctx context.Context,
	store issuestorage.IssueGetter,
	issues []*issuestorage.Issue,
	closedSet map[string]bool,
	cascade bool,
) (map[string]*EffectiveBlockersResult, error) {
	results := make(map[string]*EffectiveBlockersResult, len(issues))

	if !cascade {
		// No parent walking needed — just compute direct blockers for each
		depType := issuestorage.DepTypeBlocks
		for _, issue := range issues {
			r := &EffectiveBlockersResult{}
			for _, depID := range issue.DependencyIDs(&depType) {
				if !closedSet[depID] {
					r.Direct = append(r.Direct, depID)
				}
			}
			results[issue.ID] = r
		}
		return results, nil
	}

	// With cascade, cache ancestor lookups to avoid redundant Get calls
	ancestorCache := make(map[string]*issuestorage.Issue)

	for _, issue := range issues {
		result := &EffectiveBlockersResult{}

		// Direct blockers
		depType := issuestorage.DepTypeBlocks
		for _, depID := range issue.DependencyIDs(&depType) {
			if !closedSet[depID] {
				result.Direct = append(result.Direct, depID)
			}
		}

		// Walk parent chain for inherited blockers
		currentID := issue.Parent
		visited := map[string]bool{issue.ID: true}

		for currentID != "" {
			if visited[currentID] {
				break
			}
			visited[currentID] = true

			ancestor, ok := ancestorCache[currentID]
			if !ok {
				var err error
				ancestor, err = store.Get(ctx, currentID)
				if err != nil {
					log.Printf("WARNING: graph.EffectiveBlockersBatch: cannot load ancestor %s: %v", currentID, err)
					break
				}
				ancestorCache[currentID] = ancestor
			}

			for _, depID := range ancestor.DependencyIDs(&depType) {
				if !closedSet[depID] {
					result.Inherited = append(result.Inherited, InheritedBlocker{
						AncestorID: currentID,
						BlockerID:  depID,
					})
				}
			}

			currentID = ancestor.Parent
		}

		results[issue.ID] = result
	}

	return results, nil
}

// FindReadyStepsWithCascade is like FindReadySteps but considers inherited
// blockers from parent-chain cascade when cascade is true.
func FindReadyStepsWithCascade(
	ctx context.Context,
	store issuestorage.IssueGetter,
	children []*issuestorage.Issue,
	closedSet map[string]bool,
	cascade bool,
) ([]*issuestorage.Issue, error) {
	if !cascade {
		// Fall back to the existing non-cascade logic for efficiency
		return FindReadySteps(children, closedSet), nil
	}

	childSet := make(map[string]bool, len(children))
	for _, c := range children {
		childSet[c.ID] = true
	}

	var ready []*issuestorage.Issue
	for _, c := range children {
		if c.Status == issuestorage.StatusClosed {
			continue
		}

		blocked, err := IsEffectivelyBlocked(ctx, store, c, closedSet, cascade)
		if err != nil {
			return nil, fmt.Errorf("check blocked status for %s: %w", c.ID, err)
		}
		if blocked {
			continue
		}

		// Also check intra-set direct blockers (same logic as FindReadySteps)
		intraBlocked := false
		depType := issuestorage.DepTypeBlocks
		for _, depID := range c.DependencyIDs(&depType) {
			if childSet[depID] && !closedSet[depID] {
				intraBlocked = true
				break
			}
		}
		if !intraBlocked {
			ready = append(ready, c)
		}
	}
	return ready, nil
}
