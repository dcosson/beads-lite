// Package storage defines the interface for issue persistence in beads-lite.
// All storage engines (filesystem, SQLite, Dolt, etc.) implement this interface.
package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// DefaultMaxHierarchyDepth is the maximum number of dot-notation levels
// allowed in hierarchical child IDs (e.g., bd-a3f8.1.2.3 = depth 3).
const DefaultMaxHierarchyDepth = 3

// Directory names used by filesystem storage within the project data dir.
const (
	DirOpen    = "open"
	DirClosed  = "closed"
	DirDeleted = "deleted"
)

// ReservedDirs lists all directory names used by issue storage.
// Other storage systems (e.g., kvstorage) should not use these names.
var ReservedDirs = []string{DirOpen, DirClosed, DirDeleted}

// DependencyType represents the type of relationship between two issues.
type DependencyType string

const (
	DepTypeBlocks        DependencyType = "blocks"
	DepTypeTracks        DependencyType = "tracks"
	DepTypeRelated       DependencyType = "related"
	DepTypeParentChild   DependencyType = "parent-child"
	DepTypeDiscoveredFrom DependencyType = "discovered-from"
	DepTypeUntil         DependencyType = "until"
	DepTypeCausedBy      DependencyType = "caused-by"
	DepTypeValidates     DependencyType = "validates"
	DepTypeRelatesTo     DependencyType = "relates-to"
	DepTypeSupersedes    DependencyType = "supersedes"
)

// ValidDependencyTypes is the set of all valid dependency types.
var ValidDependencyTypes = map[DependencyType]bool{
	DepTypeBlocks:        true,
	DepTypeTracks:        true,
	DepTypeRelated:       true,
	DepTypeParentChild:   true,
	DepTypeDiscoveredFrom: true,
	DepTypeUntil:         true,
	DepTypeCausedBy:      true,
	DepTypeValidates:     true,
	DepTypeRelatesTo:     true,
	DepTypeSupersedes:    true,
}

// Dependency represents a typed dependency between two issues.
type Dependency struct {
	ID   string         `json:"id"`
	Type DependencyType `json:"type"`
}

// Issue represents a task/bug/feature in the system.
type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Type        IssueType `json:"type"`

	// Hierarchy convenience field (set automatically with parent-child deps)
	Parent string `json:"parent,omitempty"`

	// Typed dependencies
	Dependencies []Dependency `json:"dependencies,omitempty"` // issues this one depends on
	Dependents   []Dependency `json:"dependents,omitempty"`   // issues that depend on this one

	Labels    []string  `json:"labels,omitempty"`
	Assignee  string    `json:"assignee,omitempty"`
	Ephemeral bool      `json:"ephemeral,omitempty"` // If true, not exported to JSONL
	Comments  []Comment `json:"comments,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CloseReason string     `json:"close_reason,omitempty"`
}

// Children returns the IDs of child issues (dependents with type parent-child).
func (issue *Issue) Children() []string {
	var children []string
	for _, dep := range issue.Dependents {
		if dep.Type == DepTypeParentChild {
			children = append(children, dep.ID)
		}
	}
	return children
}

// HasDependency checks if the issue has a dependency on the given ID.
func (issue *Issue) HasDependency(id string) bool {
	for _, dep := range issue.Dependencies {
		if dep.ID == id {
			return true
		}
	}
	return false
}

// HasDependent checks if the issue has a dependent with the given ID.
func (issue *Issue) HasDependent(id string) bool {
	for _, dep := range issue.Dependents {
		if dep.ID == id {
			return true
		}
	}
	return false
}

// DependencyIDs returns the IDs from the Dependencies list, optionally filtered by type.
func (issue *Issue) DependencyIDs(filterType *DependencyType) []string {
	var ids []string
	for _, dep := range issue.Dependencies {
		if filterType == nil || dep.Type == *filterType {
			ids = append(ids, dep.ID)
		}
	}
	return ids
}

// DependentIDs returns the IDs from the Dependents list, optionally filtered by type.
func (issue *Issue) DependentIDs(filterType *DependencyType) []string {
	var ids []string
	for _, dep := range issue.Dependents {
		if filterType == nil || dep.Type == *filterType {
			ids = append(ids, dep.ID)
		}
	}
	return ids
}

// Comment represents a comment on an issue.
type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Status represents the current state of an issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDeferred   Status = "deferred"
	StatusClosed     Status = "closed"
)

// Priority represents the urgency of an issue.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
	PriorityBacklog  Priority = "backlog"
)

// Display returns the priority in P0-P4 format for human-readable output.
func (p Priority) Display() string {
	switch p {
	case PriorityCritical:
		return "P0"
	case PriorityHigh:
		return "P1"
	case PriorityMedium:
		return "P2"
	case PriorityLow:
		return "P3"
	case PriorityBacklog:
		return "P4"
	default:
		return string(p)
	}
}

// IssueType represents the category of an issue.
type IssueType string

const (
	TypeTask    IssueType = "task"
	TypeBug     IssueType = "bug"
	TypeFeature IssueType = "feature"
	TypeEpic    IssueType = "epic"
	TypeChore   IssueType = "chore"
)

// ListFilter specifies criteria for listing issues.
type ListFilter struct {
	Status          *Status    // nil means any
	Priority        *Priority  // nil means any
	Type            *IssueType // nil means any
	Parent          *string    // nil means any, empty string means root only
	Labels          []string   // issues must have all these labels
	Assignee        *string    // nil means any
	IncludeChildren bool       // if true, include descendants of matching issues
}

// Storage defines the interface for issue persistence.
// All storage engines must implement this interface.
type Storage interface {
	// Create creates a new issue and returns its ID.
	// If issue.ID is already set, that ID is used directly (for hierarchical child IDs).
	// Otherwise, a random ID is generated.
	Create(ctx context.Context, issue *Issue) (string, error)

	// Get retrieves an issue by ID.
	// Returns ErrNotFound if the issue doesn't exist.
	Get(ctx context.Context, id string) (*Issue, error)

	// Update replaces an issue's data.
	// Returns ErrNotFound if the issue doesn't exist.
	Update(ctx context.Context, issue *Issue) error

	// Delete permanently removes an issue.
	// Returns ErrNotFound if the issue doesn't exist.
	Delete(ctx context.Context, id string) error

	// List returns all issues matching the filter.
	// If filter is nil, returns all open issues.
	List(ctx context.Context, filter *ListFilter) ([]*Issue, error)

	// Close moves an issue to closed status.
	// This is separate from Update because implementations may
	// handle closed issues differently (e.g., move to different directory).
	Close(ctx context.Context, id string) error

	// Reopen moves a closed issue back to open status.
	Reopen(ctx context.Context, id string) error

	// AddDependency creates a typed dependency relationship (issueID depends on dependsOnID).
	// Locks both issues, then updates:
	//   - issueID.dependencies += {dependsOnID, depType}
	//   - dependsOnID.dependents += {issueID, depType}
	// When depType is parent-child, also sets issueID.Parent = dependsOnID
	// and handles reparenting (removes old parent-child dep if child had a previous parent).
	AddDependency(ctx context.Context, issueID, dependsOnID string, depType DependencyType) error

	// RemoveDependency removes a dependency relationship by ID.
	// Locks both issues and removes the dep entry from both sides.
	// If the removed dep was parent-child, also clears issueID.Parent.
	RemoveDependency(ctx context.Context, issueID, dependsOnID string) error

	// AddComment adds a comment to an issue.
	AddComment(ctx context.Context, issueID string, comment *Comment) error

	// GetNextChildID validates the parent exists, checks hierarchy depth limits,
	// atomically increments the child counter, and returns the full child ID
	// (e.g., "bd-a3f8" → "bd-a3f8.1", "bd-a3f8.1" → "bd-a3f8.1.1").
	// Returns ErrNotFound if parent doesn't exist, ErrMaxDepthExceeded if
	// the parent is already at the maximum hierarchy depth.
	GetNextChildID(ctx context.Context, parentID string) (string, error)

	// Init initializes the storage (creates directories, etc.).
	Init(ctx context.Context) error

	// Doctor checks for and optionally fixes inconsistencies.
	Doctor(ctx context.Context, fix bool) ([]string, error)
}

// IsHierarchicalID reports whether id is a hierarchical child ID.
// An ID is hierarchical if it contains a dot and the suffix after the last
// dot is purely numeric (e.g. "bd-a3f8.1" is hierarchical, but
// "my.project-abc" is not).
func IsHierarchicalID(id string) bool {
	dot := strings.LastIndex(id, ".")
	if dot < 0 || dot == len(id)-1 {
		return false
	}
	suffix := id[dot+1:]
	for _, r := range suffix {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// HierarchyDepth returns the nesting depth of an ID by counting dots.
// A root ID like "bd-a3f8" has depth 0; "bd-a3f8.1" has depth 1, etc.
func HierarchyDepth(id string) int {
	return strings.Count(id, ".")
}

// CheckHierarchyDepth verifies that parentID is not already at the maximum
// hierarchy depth. If adding a child to parentID would exceed maxDepth,
// it returns ErrMaxDepthExceeded with a descriptive message.
// For example, with maxDepth=3, a parent "bd-x.1.2.3" (depth 3) is rejected
// because a child would be at depth 4.
func CheckHierarchyDepth(parentID string, maxDepth int) error {
	depth := HierarchyDepth(parentID)
	if depth >= maxDepth {
		return fmt.Errorf("cannot add child to %s (depth %d): maximum hierarchy depth is %d: %w",
			parentID, depth, maxDepth, ErrMaxDepthExceeded)
	}
	return nil
}

// ChildID returns the composite child ID given a parent ID and child number.
func ChildID(parentID string, childNum int) string {
	return fmt.Sprintf("%s.%d", parentID, childNum)
}

// ParseHierarchicalID splits a hierarchical ID into its immediate parent and
// child number. For example, "bd-a3f8.2" returns ("bd-a3f8", 2, true).
// Returns ("", 0, false) if the ID is not hierarchical.
func ParseHierarchicalID(id string) (parentID string, childNum int, ok bool) {
	if !IsHierarchicalID(id) {
		return "", 0, false
	}
	dot := strings.LastIndex(id, ".")
	parentID = id[:dot]
	childNum, _ = strconv.Atoi(id[dot+1:])
	return parentID, childNum, true
}

// RootParentID returns the root parent portion of a (possibly hierarchical) ID.
// For hierarchical IDs this is everything before the first dot
// (e.g. "bd-a3f8.1.2" → "bd-a3f8"). For non-hierarchical IDs the full ID
// is returned unchanged.
func RootParentID(id string) string {
	dot := strings.Index(id, ".")
	if dot < 0 {
		return id
	}
	return id[:dot]
}
