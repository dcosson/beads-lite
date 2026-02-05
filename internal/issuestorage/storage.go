// Package issuestorage defines the interface for issue persistence in beads-lite.
// All storage engines (filesystem, SQLite, Dolt, etc.) implement this interface.
package issuestorage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ApplyStatusDefaults sets side-effect fields for status transitions.
// Called by Modify implementations after fn() returns, before writing.
func ApplyStatusDefaults(old, updated *Issue) {
	if updated.Status == StatusClosed && old.Status != StatusClosed {
		now := time.Now()
		updated.ClosedAt = &now
		if updated.CloseReason == "" {
			updated.CloseReason = "Closed"
		}
	}
	if old.Status == StatusClosed && updated.Status != StatusClosed {
		updated.ClosedAt = nil
		updated.CloseReason = ""
	}
}

// DirForStatus returns the directory name for the given issue status.
func DirForStatus(status Status) string {
	switch status {
	case StatusClosed:
		return DirClosed
	case StatusTombstone:
		return DirDeleted
	default:
		return DirOpen
	}
}

// DefaultMaxHierarchyDepth is the maximum number of dot-notation levels
// allowed in hierarchical child IDs (e.g., bd-a3f8.1.2.3 = depth 3).
const DefaultMaxHierarchyDepth = 3

// Directory names used by filesystem storage within the project data dir.
const (
	DirIssues    = "issues"
	DirOpen      = "open"
	DirClosed    = "closed"
	DirDeleted   = "deleted"
	DirEphemeral = "ephemeral"
)

// ReservedDirs lists all directory names used by issue storage.
// Other storage systems (e.g., kvstorage) should not use these names.
var ReservedDirs = []string{DirOpen, DirClosed, DirDeleted, DirEphemeral}

// DirForIssue returns the appropriate directory for an issue based on its
// status and ephemeral flag. Tombstoned issues always go to DirDeleted.
// Non-tombstone ephemeral issues go to DirEphemeral; others go to the
// directory matching their status.
func DirForIssue(issue *Issue) string {
	if issue.Status == StatusTombstone {
		return DirDeleted
	}
	if issue.Ephemeral {
		return DirEphemeral
	}
	return DirForStatus(issue.Status)
}

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

// BuildPrefix composes a full ID prefix from a base prefix and an optional
// addition. It normalises dashes so the result always ends with exactly one
// dash and never contains double-dashes.
//
//	BuildPrefix("bd-", "")    → "bd-"
//	BuildPrefix("bd-", "mol") → "bd-mol-"
//	BuildPrefix("bd",  "mol") → "bd-mol-"
//	BuildPrefix("bd-", "-mol-") → "bd-mol-"
func BuildPrefix(base, addition string) string {
	base = strings.TrimRight(base, "-")
	addition = strings.Trim(addition, "-")
	if addition == "" {
		return base + "-"
	}
	return base + "-" + addition + "-"
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
