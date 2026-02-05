// Package issuestorage defines the interface for issue persistence in beads-lite.
// All storage engines (filesystem, SQLite, Dolt, etc.) implement this interface.
package issuestorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors returned by IssueStore implementations.
var (
	ErrNotFound          = errors.New("issue not found")
	ErrAlreadyExists     = errors.New("issue already exists")
	ErrLockTimeout       = errors.New("could not acquire lock")
	ErrInvalidID         = errors.New("invalid issue ID")
	ErrCycle             = errors.New("operation would create a cycle")
	ErrAlreadyTombstoned = errors.New("issue is already tombstoned")
)

// DependencyType represents the type of relationship between two issues.
type DependencyType string

const (
	DepTypeBlocks         DependencyType = "blocks"
	DepTypeTracks         DependencyType = "tracks"
	DepTypeRelated        DependencyType = "related"
	DepTypeParentChild    DependencyType = "parent-child"
	DepTypeDiscoveredFrom DependencyType = "discovered-from"
	DepTypeUntil          DependencyType = "until"
	DepTypeCausedBy       DependencyType = "caused-by"
	DepTypeValidates      DependencyType = "validates"
	DepTypeRelatesTo      DependencyType = "relates-to"
	DepTypeSupersedes     DependencyType = "supersedes"
)

// ValidDependencyTypes is the set of all valid dependency types.
var ValidDependencyTypes = map[DependencyType]bool{
	DepTypeBlocks:         true,
	DepTypeTracks:         true,
	DepTypeRelated:        true,
	DepTypeParentChild:    true,
	DepTypeDiscoveredFrom: true,
	DepTypeUntil:          true,
	DepTypeCausedBy:       true,
	DepTypeValidates:      true,
	DepTypeRelatesTo:      true,
	DepTypeSupersedes:     true,
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
	MolType     MolType   `json:"mol_type,omitempty"`

	// Hierarchy convenience field (set automatically with parent-child deps)
	Parent string `json:"parent,omitempty"`

	// Typed dependencies
	Dependencies []Dependency `json:"dependencies,omitempty"` // issues this one depends on
	Dependents   []Dependency `json:"dependents,omitempty"`   // issues that depend on this one

	CreatedBy string `json:"created_by,omitempty"`
	Owner     string `json:"owner,omitempty"`

	Labels      []string   `json:"labels,omitempty"`
	Assignee    string     `json:"assignee,omitempty"`
	Ephemeral   bool       `json:"ephemeral,omitempty"` // If true, not exported to JSONL
	Comments    []Comment  `json:"comments,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CloseReason string     `json:"close_reason,omitempty"`

	// Gate fields (async coordination primitives)
	AwaitType string   `json:"await_type,omitempty"` // "gh:run", "gh:pr", "timer", "human", "bead"
	AwaitID   string   `json:"await_id,omitempty"`   // external identifier being waited on
	TimeoutNS int64    `json:"timeout_ns,omitempty"` // nanoseconds (matches reference impl column name)
	Waiters   []string `json:"waiters,omitempty"`    // addresses to notify when gate clears

	// Tombstone fields (set when issue is soft-deleted)
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	DeletedBy    string     `json:"deleted_by,omitempty"`
	DeleteReason string     `json:"delete_reason,omitempty"`
	OriginalType IssueType  `json:"original_type,omitempty"`
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
	ID        int       `json:"id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// Status represents the current state of an issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDeferred   Status = "deferred"
	StatusHooked     Status = "hooked"
	StatusPinned     Status = "pinned"
	StatusClosed     Status = "closed"
	StatusTombstone  Status = "tombstone"
)

// BuiltinStatuses lists the statuses users can set directly (excludes tombstone).
var BuiltinStatuses = []Status{
	StatusOpen, StatusInProgress, StatusBlocked, StatusDeferred,
	StatusHooked, StatusPinned, StatusClosed,
}

// Priority represents the urgency of an issue (0=critical .. 4=backlog).
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityMedium   Priority = 2
	PriorityLow      Priority = 3
	PriorityBacklog  Priority = 4
)

// Display returns the priority in P0-P4 format for human-readable output.
func (p Priority) Display() string {
	return fmt.Sprintf("P%d", p)
}

// MarshalJSON writes priority as an integer.
func (p Priority) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(p))
}

// UnmarshalJSON reads priority from an integer or a legacy word-form string
// ("critical", "high", "medium", "low", "backlog") for backward compatibility
// with on-disk JSON written before the int conversion.
func (p *Priority) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*p = Priority(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("priority must be int or string, got %s", string(data))
	}
	switch strings.ToLower(s) {
	case "critical":
		*p = PriorityCritical
	case "high":
		*p = PriorityHigh
	case "medium":
		*p = PriorityMedium
	case "low":
		*p = PriorityLow
	case "backlog":
		*p = PriorityBacklog
	default:
		return fmt.Errorf("unknown priority %q", s)
	}
	return nil
}

// ParsePriority converts a string to a Priority value.
// Accepts numeric ("0"-"4"), P-format ("P0"-"P4"), or legacy word forms
// ("critical", "high", "medium", "low", "backlog").
// Returns PriorityMedium and an error for unrecognized input.
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "p0", "critical":
		return PriorityCritical, nil
	case "1", "p1", "high":
		return PriorityHigh, nil
	case "2", "p2", "medium", "":
		return PriorityMedium, nil
	case "3", "p3", "low":
		return PriorityLow, nil
	case "4", "p4", "backlog":
		return PriorityBacklog, nil
	default:
		return PriorityMedium, fmt.Errorf("unknown priority %q", s)
	}
}

// IssueType represents the category of an issue.
type IssueType string

const (
	TypeTask     IssueType = "task"
	TypeBug      IssueType = "bug"
	TypeFeature  IssueType = "feature"
	TypeEpic     IssueType = "epic"
	TypeChore    IssueType = "chore"
	TypeGate     IssueType = "gate"
	TypeMolecule IssueType = "molecule"
)

// MolType represents the molecule type of an issue.
type MolType string

const (
	MolTypeSwarm  MolType = "swarm"
	MolTypePatrol MolType = "patrol"
	MolTypeWork   MolType = "work"
)

// ValidateMolType returns true if the given value is a valid MolType.
// Empty string is treated as equivalent to MolTypeWork.
func ValidateMolType(s string) bool {
	switch s {
	case "", string(MolTypeSwarm), string(MolTypePatrol), string(MolTypeWork):
		return true
	}
	return false
}

// ListFilter specifies criteria for listing issues.
type ListFilter struct {
	Status          *Status    // nil means any
	Priority        *Priority  // nil means any
	Type            *IssueType // nil means any
	MolType         *MolType   // nil means any
	Parent          *string    // nil means any, empty string means root only
	Labels          []string   // issues must have all these labels
	Assignee        *string    // nil means any
	IncludeChildren bool       // if true, include descendants of matching issues
}

// CreateOpts provides optional parameters for Create.
type CreateOpts struct {
	// PrefixAddition is inserted between the store's base prefix and the
	// random suffix when generating a new ID (e.g. "mol" → "bd-mol-xxxx").
	// Ignored when issue.ID is already set.
	PrefixAddition string
}

// IssueGetter provides read-only access to issues by ID.
// IssueStore embeds this implicitly (it has a Get method), so any IssueStore
// value satisfies IssueGetter. A separate implementation can provide
// routing-aware lookups that dispatch to different stores based on ID prefix.
type IssueGetter interface {
	Get(ctx context.Context, id string) (*Issue, error)
}

// IssueStore defines the interface for issue persistence.
// All storage engines must implement this interface.
type IssueStore interface {
	// Create creates a new issue and returns its ID.
	// If issue.ID is already set, that ID is used directly (for hierarchical child IDs).
	// Otherwise, a deterministic content-based ID is generated.
	// An optional CreateOpts may be supplied to customise ID generation.
	Create(ctx context.Context, issue *Issue, opts ...CreateOpts) (string, error)

	// Get retrieves an issue by ID.
	// Returns ErrNotFound if the issue doesn't exist.
	Get(ctx context.Context, id string) (*Issue, error)

	// Modify atomically reads an issue, applies fn to it, and writes it back.
	// fn receives the current issue from disk (under lock) and should mutate it.
	// Status transitions (e.g., setting StatusClosed or reopening) are handled
	// automatically: ApplyStatusDefaults sets ClosedAt/CloseReason, and
	// filesystem implementations move files between directories as needed.
	// Returns ErrNotFound if the issue doesn't exist.
	Modify(ctx context.Context, id string, fn func(*Issue) error) error

	// Delete permanently removes an issue.
	// Returns ErrNotFound if the issue doesn't exist.
	Delete(ctx context.Context, id string) error

	// List returns all issues matching the filter.
	// If filter is nil, returns all open issues.
	List(ctx context.Context, filter *ListFilter) ([]*Issue, error)

	// GetNextChildID validates the parent exists, checks hierarchy depth limits,
	// scans for existing children, and returns the next child ID
	// (e.g., "bd-a3f8" → "bd-a3f8.1", "bd-a3f8.1" → "bd-a3f8.1.1").
	// The returned ID is not reserved; the caller should use O_EXCL on
	// creation and retry GetNextChildID on collision.
	// Returns ErrNotFound if parent doesn't exist, ErrMaxDepthExceeded if
	// the parent is already at the maximum hierarchy depth.
	GetNextChildID(ctx context.Context, parentID string) (string, error)

	// Init initializes the storage (creates directories, etc.).
	Init(ctx context.Context) error

	// Doctor checks for and optionally fixes inconsistencies.
	Doctor(ctx context.Context, fix bool) ([]string, error)
}
