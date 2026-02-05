package issueservice

import (
	"context"
	"fmt"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/routing"
)

// IssueStore routes operations to the correct underlying store based on
// issue ID prefix. It consolidates all dependency domain logic (cycle
// detection, parent-child handling, reparenting) so that underlying
// storage backends remain pure CRUD.
//
// IssueStore should always be used in place of a storage backend directly,
// even in code paths where there is no routing configured. It enforces
// dependency validations (cycle detection, parent-child same-rig
// constraints, reparenting) that raw storage backends do not.
//
// When router is nil, all operations delegate straight to the local store.
type IssueStore struct {
	router *routing.Router
	local  issuestorage.IssueStore
	stores map[string]issuestorage.IssueStore // cache opened stores by prefix
}

// NewIssueStore creates a routing-aware IssueStore. When router is nil,
// all operations pass through to local with no overhead.
// NewIssueStore creates an IssueStore with optional routing. When router is nil,
// all operations pass through to local with dependency validation only.
func New(router *routing.Router, local issuestorage.IssueStore) *IssueStore {
	return &IssueStore{
		router: router,
		local:  local,
		stores: make(map[string]issuestorage.IssueStore),
	}
}

// storeFor returns the underlying IssueStore for the given issue ID.
// Caches opened remote stores by prefix for the lifetime of this IssueStore.
func (s *IssueStore) storeFor(id string) issuestorage.IssueStore {
	if s.router == nil {
		return s.local
	}

	paths, prefix, isRemote, err := s.router.Resolve(id)
	if err != nil || !isRemote {
		return s.local
	}

	if store, ok := s.stores[prefix]; ok {
		return store
	}

	store := filesystem.New(paths.DataDir, prefix)
	s.stores[prefix] = store
	return store
}

// sameStore reports whether two issue IDs resolve to the same storage.
func (s *IssueStore) sameStore(id1, id2 string) bool {
	if s.router == nil {
		return true
	}
	return s.router.SameStore(id1, id2)
}

// Router returns the underlying router (may be nil).
func (s *IssueStore) Router() *routing.Router {
	return s.router
}

// --- issuestorage.IssueStore: single-ID routing ---

func (s *IssueStore) Get(ctx context.Context, id string) (*issuestorage.Issue, error) {
	return s.storeFor(id).Get(ctx, id)
}

func (s *IssueStore) Modify(ctx context.Context, id string, fn func(*issuestorage.Issue) error) error {
	return s.storeFor(id).Modify(ctx, id, fn)
}

func (s *IssueStore) Delete(ctx context.Context, id string) error {
	return s.storeFor(id).Delete(ctx, id)
}

func (s *IssueStore) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	return s.storeFor(parentID).GetNextChildID(ctx, parentID)
}

// --- issuestorage.IssueStore: always local ---

func (s *IssueStore) Create(ctx context.Context, issue *issuestorage.Issue, opts ...issuestorage.CreateOpts) (string, error) {
	return s.local.Create(ctx, issue, opts...)
}

func (s *IssueStore) List(ctx context.Context, filter *issuestorage.ListFilter) ([]*issuestorage.Issue, error) {
	return s.local.List(ctx, filter)
}

func (s *IssueStore) Init(ctx context.Context) error {
	return s.local.Init(ctx)
}

func (s *IssueStore) Doctor(ctx context.Context, fix bool) ([]string, error) {
	return s.local.Doctor(ctx, fix)
}

// --- Dependency operations ---

// AddDependency creates a typed dependency relationship (issueID depends on dependsOnID).
// Handles cycle detection, parent-child constraints, and reparenting.
func (s *IssueStore) AddDependency(ctx context.Context, issueID, dependsOnID string, depType issuestorage.DependencyType) error {
	if depType == issuestorage.DepTypeParentChild {
		return s.addParentChildDep(ctx, issueID, dependsOnID)
	}

	hasCycle, err := s.hasCycle(ctx, issueID, dependsOnID)
	if err != nil {
		return err
	}
	if hasCycle {
		return issuestorage.ErrCycle
	}

	// Add dependency to the source issue
	if err := s.storeFor(issueID).Modify(ctx, issueID, func(issue *issuestorage.Issue) error {
		if !issue.HasDependency(dependsOnID) {
			issue.Dependencies = append(issue.Dependencies, issuestorage.Dependency{ID: dependsOnID, Type: depType})
		}
		return nil
	}); err != nil {
		return err
	}

	// Add inverse dependent to the target issue
	return s.storeFor(dependsOnID).Modify(ctx, dependsOnID, func(dep *issuestorage.Issue) error {
		if !dep.HasDependent(issueID) {
			dep.Dependents = append(dep.Dependents, issuestorage.Dependency{ID: issueID, Type: depType})
		}
		return nil
	})
}

// addParentChildDep handles AddDependency with parent-child type.
// Parent-child deps must be same-rig. Handles reparenting.
func (s *IssueStore) addParentChildDep(ctx context.Context, childID, parentID string) error {
	if !s.sameStore(childID, parentID) {
		return fmt.Errorf("cannot add parent-child dependency across different rigs")
	}

	hasCycle, err := s.hasHierarchyCycle(ctx, childID, parentID)
	if err != nil {
		return err
	}
	if hasCycle {
		return issuestorage.ErrCycle
	}

	store := s.storeFor(childID)

	// Modify the child: set parent, remove old parent dep, add new parent dep
	var oldParentID string
	if err := store.Modify(ctx, childID, func(child *issuestorage.Issue) error {
		if child.Parent != "" && child.Parent != parentID {
			oldParentID = child.Parent
			child.Dependencies = removeDep(child.Dependencies, child.Parent)
		}
		child.Parent = parentID
		if !child.HasDependency(parentID) {
			child.Dependencies = append(child.Dependencies, issuestorage.Dependency{ID: parentID, Type: issuestorage.DepTypeParentChild})
		}
		return nil
	}); err != nil {
		return err
	}

	// Remove child from old parent's dependents
	if oldParentID != "" {
		_ = store.Modify(ctx, oldParentID, func(oldParent *issuestorage.Issue) error {
			oldParent.Dependents = removeDep(oldParent.Dependents, childID)
			return nil
		})
	}

	// Add child to new parent's dependents
	return store.Modify(ctx, parentID, func(parent *issuestorage.Issue) error {
		if !parent.HasDependent(childID) {
			parent.Dependents = append(parent.Dependents, issuestorage.Dependency{ID: childID, Type: issuestorage.DepTypeParentChild})
		}
		return nil
	})
}

// RemoveDependency removes a dependency relationship by ID from both sides.
// If the removed dep was parent-child, also clears issueID.Parent.
func (s *IssueStore) RemoveDependency(ctx context.Context, issueID, dependsOnID string) error {
	if err := s.storeFor(issueID).Modify(ctx, issueID, func(issue *issuestorage.Issue) error {
		for _, dep := range issue.Dependencies {
			if dep.ID == dependsOnID && dep.Type == issuestorage.DepTypeParentChild {
				issue.Parent = ""
				break
			}
		}
		issue.Dependencies = removeDep(issue.Dependencies, dependsOnID)
		return nil
	}); err != nil {
		return err
	}

	return s.storeFor(dependsOnID).Modify(ctx, dependsOnID, func(dep *issuestorage.Issue) error {
		dep.Dependents = removeDep(dep.Dependents, issueID)
		return nil
	})
}

// --- Cycle detection ---

// hasCycle checks if adding issueID→dependsOnID would create a cycle.
// BFS from dependsOnID following Dependencies; if issueID is reachable, it's a cycle.
// Uses s.Get() so traversal is routing-aware (works across rigs).
func (s *IssueStore) hasCycle(ctx context.Context, issueID, dependsOnID string) (bool, error) {
	if issueID == dependsOnID {
		return true, nil
	}

	visited := make(map[string]bool)
	queue := []string{dependsOnID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		issue, err := s.Get(ctx, current)
		if err != nil {
			if err == issuestorage.ErrNotFound {
				continue
			}
			return false, err
		}

		for _, dep := range issue.Dependencies {
			if dep.ID == issueID {
				return true, nil
			}
			if !visited[dep.ID] {
				queue = append(queue, dep.ID)
			}
		}
	}

	return false, nil
}

// hasHierarchyCycle checks if setting child's parent to parent would create
// a cycle in the hierarchy. Walks the ancestor chain from parent via .Parent.
func (s *IssueStore) hasHierarchyCycle(ctx context.Context, child, parent string) (bool, error) {
	if child == parent {
		return true, nil
	}

	current := parent
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			break
		}
		visited[current] = true

		issue, err := s.Get(ctx, current)
		if err != nil {
			if err == issuestorage.ErrNotFound {
				break
			}
			return false, err
		}

		if issue.Parent == child {
			return true, nil
		}
		current = issue.Parent
	}

	return false, nil
}

// removeDep removes a dependency entry by ID from a Dependency slice.
func removeDep(deps []issuestorage.Dependency, id string) []issuestorage.Dependency {
	result := make([]issuestorage.Dependency, 0, len(deps))
	for _, d := range deps {
		if d.ID != id {
			result = append(result, d)
		}
	}
	return result
}
