// Package storage defines the interface for issue persistence in beads.
// All storage engines (filesystem, SQLite, Dolt, etc.) implement this interface.
package storage

import (
	"context"
	"time"
)

// Issue represents a task/bug/feature in the system.
type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Type        IssueType `json:"type"`

	// Hierarchy (doubly-linked)
	Parent   string   `json:"parent,omitempty"`
	Children []string `json:"children,omitempty"`

	// Dependencies (doubly-linked)
	DependsOn  []string `json:"depends_on,omitempty"`  // issues this one waits for
	Dependents []string `json:"dependents,omitempty"` // issues waiting for this one

	// Blocking (doubly-linked)
	Blocks    []string `json:"blocks,omitempty"`     // issues this one blocks
	BlockedBy []string `json:"blocked_by,omitempty"` // issues blocking this one

	Labels    []string  `json:"labels,omitempty"`
	Assignee  string    `json:"assignee,omitempty"`
	Comments  []Comment `json:"comments,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
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
	StatusInProgress Status = "in-progress"
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
	// Create creates a new issue and returns its generated ID.
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

	// AddDependency creates a dependency relationship (A depends on B).
	// Locks both issues, then updates:
	//   - A.depends_on += B
	//   - B.dependents += A
	AddDependency(ctx context.Context, dependentID, dependencyID string) error

	// RemoveDependency removes a dependency relationship.
	// Locks both issues and updates both sides.
	RemoveDependency(ctx context.Context, dependentID, dependencyID string) error

	// AddBlock creates a blocking relationship (A blocks B).
	// Locks both issues, then updates:
	//   - A.blocks += B
	//   - B.blocked_by += A
	AddBlock(ctx context.Context, blockerID, blockedID string) error

	// RemoveBlock removes a blocking relationship.
	// Locks both issues and updates both sides.
	RemoveBlock(ctx context.Context, blockerID, blockedID string) error

	// SetParent sets the parent of an issue.
	// Locks both issues, then updates:
	//   - child.parent = parent
	//   - parent.children += child
	// If child had a previous parent, also locks and updates that issue.
	SetParent(ctx context.Context, childID, parentID string) error

	// RemoveParent removes the parent relationship.
	// Locks both child and parent, updates both sides.
	RemoveParent(ctx context.Context, childID string) error

	// AddComment adds a comment to an issue.
	AddComment(ctx context.Context, issueID string, comment *Comment) error

	// Init initializes the storage (creates directories, etc.).
	Init(ctx context.Context) error

	// Doctor checks for and optionally fixes inconsistencies.
	Doctor(ctx context.Context, fix bool) ([]string, error)
}

