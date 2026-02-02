// Package testutil provides test utilities for beads-lite storage testing.
package testutil

import (
	"context"
	"fmt"
	"math/rand"

	"beads-lite/internal/issuestorage"
)

// IssueGenerator creates test issues with various relationship patterns.
type IssueGenerator struct {
	storage issuestorage.IssueStore
	ids     []string
}

// NewIssueGenerator creates a new generator with the given storage.
func NewIssueGenerator(s issuestorage.IssueStore) *IssueGenerator {
	return &IssueGenerator{
		storage: s,
		ids:     make([]string, 0),
	}
}

// IDs returns all issue IDs created by this generator.
func (g *IssueGenerator) IDs() []string {
	return g.ids
}

// Cleanup deletes all issues created by this generator.
func (g *IssueGenerator) Cleanup(ctx context.Context) error {
	// Delete in reverse order to handle dependencies
	for i := len(g.ids) - 1; i >= 0; i-- {
		if err := g.storage.Delete(ctx, g.ids[i]); err != nil {
			return fmt.Errorf("cleanup issue %s: %w", g.ids[i], err)
		}
	}
	g.ids = g.ids[:0]
	return nil
}

// GenerateTree creates a hierarchy of issues with the specified depth and breadth.
// Each level has 'breadth' children, creating breadth^depth total issues.
func (g *IssueGenerator) GenerateTree(ctx context.Context, depth, breadth int) error {
	return g.generateTreeRecursive(ctx, "", depth, breadth)
}

func (g *IssueGenerator) generateTreeRecursive(ctx context.Context, parent string, depth, breadth int) error {
	if depth == 0 {
		return nil
	}
	for i := 0; i < breadth; i++ {
		issue := &issuestorage.Issue{
			Title:    fmt.Sprintf("Level %d Issue %d", depth, i),
			Status:   issuestorage.StatusOpen,
			Priority: issuestorage.PriorityMedium,
			Type:     issuestorage.TypeTask,
		}
		id, err := g.storage.Create(ctx, issue)
		if err != nil {
			return fmt.Errorf("create issue at depth %d: %w", depth, err)
		}
		g.ids = append(g.ids, id)

		if parent != "" {
			if err := g.storage.AddDependency(ctx, id, parent, issuestorage.DepTypeParentChild); err != nil {
				return fmt.Errorf("set parent for %s: %w", id, err)
			}
		}

		if err := g.generateTreeRecursive(ctx, id, depth-1, breadth); err != nil {
			return err
		}
	}
	return nil
}

// GenerateDependencyChain creates a linear dependency chain: A -> B -> C -> ...
// where each issue depends on the previous one.
// Returns the IDs in order from first (no dependencies) to last (depends on all).
func (g *IssueGenerator) GenerateDependencyChain(ctx context.Context, length int) ([]string, error) {
	if length <= 0 {
		return nil, nil
	}

	ids := make([]string, length)
	for i := 0; i < length; i++ {
		issue := &issuestorage.Issue{
			Title:    fmt.Sprintf("Chain %d", i),
			Status:   issuestorage.StatusOpen,
			Priority: issuestorage.PriorityMedium,
			Type:     issuestorage.TypeTask,
		}
		id, err := g.storage.Create(ctx, issue)
		if err != nil {
			return nil, fmt.Errorf("create chain issue %d: %w", i, err)
		}
		ids[i] = id
		g.ids = append(g.ids, id)

		if i > 0 {
			// Current issue depends on the previous one
			if err := g.storage.AddDependency(ctx, id, ids[i-1], issuestorage.DepTypeBlocks); err != nil {
				return nil, fmt.Errorf("add dependency from %s to %s: %w", id, ids[i-1], err)
			}
		}
	}
	return ids, nil
}

// GenerateDependencyDAG creates a directed acyclic graph of dependencies.
// Creates 'nodes' issues and adds up to 'edges' dependency relationships.
// Only creates forward edges (from higher-indexed to lower-indexed nodes)
// to guarantee no cycles.
func (g *IssueGenerator) GenerateDependencyDAG(ctx context.Context, nodes, edges int) error {
	if nodes <= 0 {
		return nil
	}

	ids := make([]string, nodes)
	for i := 0; i < nodes; i++ {
		issue := &issuestorage.Issue{
			Title:    fmt.Sprintf("Node %d", i),
			Status:   issuestorage.StatusOpen,
			Priority: issuestorage.PriorityMedium,
			Type:     issuestorage.TypeTask,
		}
		id, err := g.storage.Create(ctx, issue)
		if err != nil {
			return fmt.Errorf("create DAG node %d: %w", i, err)
		}
		ids[i] = id
		g.ids = append(g.ids, id)
	}

	// Create random edges (dependencies)
	// Only create forward edges to avoid cycles
	for i := 0; i < edges; i++ {
		a := rand.Intn(nodes)
		b := rand.Intn(nodes)
		if a != b && a < b {
			// b depends on a (higher index depends on lower)
			if err := g.storage.AddDependency(ctx, ids[b], ids[a], issuestorage.DepTypeBlocks); err != nil {
				// Ignore duplicate edge errors, just continue
				continue
			}
		}
	}
	return nil
}
