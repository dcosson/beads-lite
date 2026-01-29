# Beads Lite Design Specification

## Overview

Beads Lite is a complete redesign of the storage layer, replacing the dual SQLite + JSONL sync architecture with a **pluggable storage engine** model. The system defines a clean `Storage` interface that commands program against, with swappable backend implementations.

**Core principles:**
- Storage engine is swappable via interface (filesystem, SQLite, Dolt, etc.)
- No sync layer, no daemon, no background processes
- Commands complete in milliseconds, not seconds
- Each storage engine optimizes for different use cases

**Storage engine options:**

| Engine | Use Case | Trade-offs |
|--------|----------|------------|
| **Filesystem** (default) | Git-native workflows, multi-agent | Human-readable, git-diff-friendly, but no transactions |
| **SQLite** (planned) | Single-user, offline-first | ACID transactions, but binary file in git |
| **Dolt** (planned) | SQL + git branching | Best of both worlds, but external dependency |

This document focuses on the **Filesystem storage engine**, which is the default and reference implementation. Other engines implement the same `Storage` interface with engine-specific optimizations.

---

# Filesystem Storage Engine

The following sections describe the **Filesystem storage engine** specifically. Other storage engines (SQLite, Dolt) will have their own implementation details while conforming to the same `Storage` interface.

## Directory Structure

```
.beads/
├── open/
│   ├── <id>.json       # Issue data (source of truth)
│   └── <id>.lock       # flock held during mutations
└── closed/
    └── <id>.json       # Closed issues (rarely need locks)
```

**No index file.** The filesystem structure *is* the state:
- `open/` contains all open issues
- `closed/` contains all closed issues
- Issue status is determined by which directory contains the file

**ID format:** `bd-<4 hex chars>` (e.g., `bd-a1b2`), generated from random bytes. Short IDs prioritize human ergonomics; collisions are handled by retry (see ID Generation).

## Issue Schema

Each `<id>.json` file contains:

```json
{
  "id": "bd-a1b2",
  "title": "Implement user authentication",
  "description": "Add login/logout functionality...",
  "status": "open",
  "priority": "high",
  "type": "feature",
  "parent": "bd-e5f6",
  "children": ["bd-i9j0"],
  "depends_on": ["bd-m3n4"],
  "dependents": ["bd-q7r8"],
  "blocks": ["bd-s9t0"],
  "blocked_by": ["bd-u1v2"],
  "labels": ["auth", "backend"],
  "assignee": "alice",
  "comments": [
    {
      "id": "c-a1b2",
      "author": "bob",
      "body": "Should we use JWT or sessions?",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "created_at": "2024-01-15T09:00:00Z",
  "updated_at": "2024-01-15T14:22:00Z",
  "closed_at": null
}
```

**Relationship pairs (doubly-linked):**

| Field | Inverse | Meaning |
|-------|---------|---------|
| `parent` | `children` | Hierarchy: this issue is a child of parent |
| `children` | `parent` | Hierarchy: these issues are children of this one |
| `depends_on` | `dependents` | This issue depends on those issues completing first |
| `dependents` | `depends_on` | Those issues depend on this one |
| `blocks` | `blocked_by` | This issue blocks those issues from starting |
| `blocked_by` | `blocks` | This issue is blocked by those issues |

All relationships are maintained symmetrically: when A.depends_on includes B, then B.dependents includes A. Updates lock both issues and modify both files atomically.

**Status values:** `open`, `in-progress`, `blocked`, `deferred`, `closed`

**Type values:** `task`, `bug`, `feature`, `epic`, `chore`

**Priority values:** `critical`, `high`, `medium`, `low`

Note: The `status` field in JSON and the directory location must agree. The directory is authoritative; if they disagree, the file should be moved to match its status or vice versa. The `bd doctor` command can detect and fix such inconsistencies.

## Locking Strategy

### Issue Locks

Each issue has a corresponding `.lock` file used with `flock(2)`:

```
open/bd-a1b2.json   # issue data
open/bd-a1b2.lock   # lock file
```

**Lock acquisition:**
1. Open or create `<id>.lock`
2. Call `flock(fd, LOCK_EX)` (blocking) or `flock(fd, LOCK_EX|LOCK_NB)` (non-blocking)
3. Perform mutations on `<id>.json`
4. Close the lock file (releases lock)

**Lock files in closed/:** Generally not needed since closed issues are rarely edited. If editing a closed issue, create a temporary lock file.

### Multi-Issue Operations

For operations touching multiple issues (e.g., adding a dependency):

1. Collect all issue IDs involved
2. Sort IDs lexicographically
3. Acquire locks in sorted order (prevents deadlock)
4. Perform all mutations
5. Release locks in reverse order

Example for `bd dep add A B` (A depends on B):
```
sorted = sort(["bd-a1b2", "bd-c3d4"])  # consistent order prevents deadlock
lock(sorted[0])
lock(sorted[1])
# update A.depends_on += B
# update B.dependents += A
unlock(sorted[1])
unlock(sorted[0])
```

### Atomicity and Crash Safety

File writes must be atomic to prevent corruption if the process crashes mid-write:

**Write pattern:**
```go
func atomicWriteJSON(path string, data interface{}) error {
    // 1. Write to temporary file in same directory
    tmp := path + ".tmp." + randomSuffix()
    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
    if err != nil {
        return err
    }

    // 2. Write and sync data
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    if err := enc.Encode(data); err != nil {
        f.Close()
        os.Remove(tmp)
        return err
    }
    if err := f.Sync(); err != nil {  // fsync before rename
        f.Close()
        os.Remove(tmp)
        return err
    }
    f.Close()

    // 3. Atomic rename (POSIX guarantees this is atomic)
    return os.Rename(tmp, path)
}
```

**Close operation (moving between directories):**
```go
func (fs *FilesystemStorage) Close(ctx context.Context, id string) error {
    lock := fs.acquireLock(id)
    defer lock.Close()

    issue, err := fs.Get(ctx, id)
    if err != nil {
        return err
    }

    issue.Status = StatusClosed
    issue.ClosedAt = timePtr(time.Now())

    // 1. Write to closed/ first (creates new file)
    closedPath := fs.issuePath(id, true)
    if err := atomicWriteJSON(closedPath, issue); err != nil {
        return err
    }

    // 2. Remove from open/ (file now exists in closed/)
    openPath := fs.issuePath(id, false)
    if err := os.Remove(openPath); err != nil {
        // Rollback: remove from closed/
        os.Remove(closedPath)
        return err
    }

    return nil
}
```

### Stale Lock Considerations

`flock(2)` locks are automatically released when:
- The file descriptor is closed
- The process exits (including crashes)
- The process is killed (SIGKILL)

**NFS Warning:** `flock` does not work reliably over NFS. If `.beads/` is on an NFS mount, use local storage or switch to the SQLite engine which uses database-level locking.

**Orphaned lock files:** Lock files may remain after the lock is released. This is harmless - they're just empty files. `bd doctor` cleans them up periodically.

### ID Generation and Collision Handling

```go
func generateID() (string, error) {
    bytes := make([]byte, 2)  // 16 bits = 4 hex chars
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return fmt.Sprintf("bd-%x", bytes), nil
}

func (fs *FilesystemStorage) Create(ctx context.Context, issue *Issue) (string, error) {
    // Retry loop for collision handling (extremely rare)
    for attempts := 0; attempts < 3; attempts++ {
        id, err := generateID()
        if err != nil {
            return "", err
        }

        path := fs.issuePath(id, false)

        // O_EXCL fails if file exists - collision detection
        f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
        if os.IsExist(err) {
            continue  // Collision, retry with new ID
        }
        if err != nil {
            return "", err
        }

        issue.ID = id
        // ... write issue data ...
        return id, nil
    }
    return "", errors.New("failed to generate unique ID after 3 attempts")
}
```

---

# Storage Interface (All Engines)

The following sections apply to **all storage engine implementations**.

## Storage Interface

The storage layer is abstracted behind an interface, allowing alternative implementations (e.g., SQLite-only for users who don't need git versioning).

```go
package storage

import (
    "context"
)

// Issue represents a task/bug/feature in the system
type Issue struct {
    ID          string            `json:"id"`
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Status      Status            `json:"status"`
    Priority    Priority          `json:"priority"`
    Type        IssueType         `json:"type"`

    // Hierarchy (doubly-linked)
    Parent      string            `json:"parent,omitempty"`
    Children    []string          `json:"children,omitempty"`

    // Dependencies (doubly-linked)
    DependsOn   []string          `json:"depends_on,omitempty"`   // issues this one waits for
    Dependents  []string          `json:"dependents,omitempty"`   // issues waiting for this one

    // Blocking (doubly-linked)
    Blocks      []string          `json:"blocks,omitempty"`       // issues this one blocks
    BlockedBy   []string          `json:"blocked_by,omitempty"`   // issues blocking this one

    Labels      []string          `json:"labels,omitempty"`
    Assignee    string            `json:"assignee,omitempty"`
    Comments    []Comment         `json:"comments,omitempty"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    ClosedAt    *time.Time        `json:"closed_at,omitempty"`
}

type Comment struct {
    ID        string    `json:"id"`
    Author    string    `json:"author"`
    Body      string    `json:"body"`
    CreatedAt time.Time `json:"created_at"`
}

type Status string

const (
    StatusOpen       Status = "open"
    StatusInProgress Status = "in-progress"
    StatusBlocked    Status = "blocked"
    StatusDeferred   Status = "deferred"
    StatusClosed     Status = "closed"
)

type Priority string

const (
    PriorityCritical Priority = "critical"
    PriorityHigh     Priority = "high"
    PriorityMedium   Priority = "medium"
    PriorityLow      Priority = "low"
)

type IssueType string

const (
    TypeTask    IssueType = "task"
    TypeBug     IssueType = "bug"
    TypeFeature IssueType = "feature"
    TypeEpic    IssueType = "epic"
    TypeChore   IssueType = "chore"
)

// ListFilter specifies criteria for listing issues
type ListFilter struct {
    Status    *Status    // nil means any
    Priority  *Priority  // nil means any
    Type      *IssueType // nil means any
    Parent    *string    // nil means any, empty string means root only
    Labels    []string   // issues must have all these labels
    Assignee  *string    // nil means any
    IncludeChildren bool // if true, include descendants of matching issues
}

// Storage defines the interface for issue persistence
type Storage interface {
    // Create creates a new issue and returns its generated ID
    Create(ctx context.Context, issue *Issue) (string, error)

    // Get retrieves an issue by ID
    // Returns ErrNotFound if the issue doesn't exist
    Get(ctx context.Context, id string) (*Issue, error)

    // Update replaces an issue's data
    // Returns ErrNotFound if the issue doesn't exist
    Update(ctx context.Context, issue *Issue) error

    // Delete permanently removes an issue
    // Returns ErrNotFound if the issue doesn't exist
    Delete(ctx context.Context, id string) error

    // List returns all issues matching the filter
    // If filter is nil, returns all open issues
    List(ctx context.Context, filter *ListFilter) ([]*Issue, error)

    // Close moves an issue to closed status
    // This is separate from Update because implementations may
    // handle closed issues differently (e.g., move to different directory)
    Close(ctx context.Context, id string) error

    // Reopen moves a closed issue back to open status
    Reopen(ctx context.Context, id string) error

    // AddDependency creates a dependency relationship (A depends on B)
    // Locks both issues, then updates:
    //   - A.depends_on += B
    //   - B.dependents += A
    AddDependency(ctx context.Context, dependentID, dependencyID string) error

    // RemoveDependency removes a dependency relationship
    // Locks both issues and updates both sides
    RemoveDependency(ctx context.Context, dependentID, dependencyID string) error

    // AddBlock creates a blocking relationship (A blocks B)
    // Locks both issues, then updates:
    //   - A.blocks += B
    //   - B.blocked_by += A
    AddBlock(ctx context.Context, blockerID, blockedID string) error

    // RemoveBlock removes a blocking relationship
    // Locks both issues and updates both sides
    RemoveBlock(ctx context.Context, blockerID, blockedID string) error

    // SetParent sets the parent of an issue
    // Locks both issues, then updates:
    //   - child.parent = parent
    //   - parent.children += child
    // If child had a previous parent, also locks and updates that issue
    SetParent(ctx context.Context, childID, parentID string) error

    // RemoveParent removes the parent relationship
    // Locks both child and parent, updates both sides
    RemoveParent(ctx context.Context, childID string) error

    // AddComment adds a comment to an issue
    AddComment(ctx context.Context, issueID string, comment *Comment) error

    // Init initializes the storage (creates directories, etc.)
    Init(ctx context.Context) error

    // Doctor checks for and optionally fixes inconsistencies
    Doctor(ctx context.Context, fix bool) ([]string, error)
}

// Errors
var (
    ErrNotFound      = errors.New("issue not found")
    ErrAlreadyExists = errors.New("issue already exists")
    ErrLockTimeout   = errors.New("could not acquire lock")
    ErrInvalidID     = errors.New("invalid issue ID")
    ErrCycle         = errors.New("operation would create a cycle")
)
```

## Filesystem Storage Implementation

```go
package filesystem

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "syscall"

    "beads-lite/storage"
)

type FilesystemStorage struct {
    root string // path to .beads directory
}

func New(root string) *FilesystemStorage {
    return &FilesystemStorage{root: root}
}

func (fs *FilesystemStorage) openDir(status storage.Status) string {
    if status == storage.StatusClosed {
        return filepath.Join(fs.root, "closed")
    }
    return filepath.Join(fs.root, "open")
}

func (fs *FilesystemStorage) issuePath(id string, closed bool) string {
    dir := "open"
    if closed {
        dir = "closed"
    }
    return filepath.Join(fs.root, dir, id+".json")
}

func (fs *FilesystemStorage) lockPath(id string) string {
    return filepath.Join(fs.root, "open", id+".lock")
}

// acquireLock gets an exclusive flock on the issue
func (fs *FilesystemStorage) acquireLock(id string) (*os.File, error) {
    lockPath := fs.lockPath(id)
    f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
    if err != nil {
        return nil, err
    }

    if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
        f.Close()
        return nil, err
    }

    return f, nil
}

// acquireLocksOrdered acquires locks on multiple issues in sorted order
func (fs *FilesystemStorage) acquireLocksOrdered(ids []string) ([]*os.File, error) {
    sorted := make([]string, len(ids))
    copy(sorted, ids)
    sort.Strings(sorted)

    locks := make([]*os.File, 0, len(sorted))
    for _, id := range sorted {
        lock, err := fs.acquireLock(id)
        if err != nil {
            // Release already-acquired locks
            for _, l := range locks {
                l.Close()
            }
            return nil, err
        }
        locks = append(locks, lock)
    }

    return locks, nil
}

func (fs *FilesystemStorage) Get(ctx context.Context, id string) (*storage.Issue, error) {
    // Check open first (more common case)
    path := fs.issuePath(id, false)
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        // Check closed
        path = fs.issuePath(id, true)
        data, err = os.ReadFile(path)
    }
    if os.IsNotExist(err) {
        return nil, storage.ErrNotFound
    }
    if err != nil {
        return nil, err
    }

    var issue storage.Issue
    if err := json.Unmarshal(data, &issue); err != nil {
        return nil, err
    }

    return &issue, nil
}

// Additional methods implement the Storage interface...
```

## CLI Commands

All commands support `--json` flag for machine-readable output.

### Core Commands

#### `bd init`

Initialize beads in the current directory.

```bash
bd init
```

Creates `.beads/<project>/open/` and `.beads/<project>/closed/` directories.

#### `bd create`

Create a new issue.

```bash
bd create "Fix login bug"
bd create "Add OAuth support" --type feature --priority high
bd create "Implement caching" --parent bd-a1b2
bd create "Write tests" --depends-on bd-e5f6
```

**Flags:**
- `--type, -t` - Issue type (task, bug, feature, epic, chore). Default: task
- `--priority, -p` - Priority (critical, high, medium, low). Default: medium
- `--parent` - Parent issue ID
- `--depends-on, -d` - Issue ID this depends on (can repeat)
- `--label, -l` - Add label (can repeat)
- `--assignee, -a` - Assign to user
- `--description` - Full description (or read from stdin with `-`)

**Output:** Prints the new issue ID.

#### `bd show <id>`

Display an issue's details.

```bash
bd show bd-a1b2
bd show bd-a1       # prefix matching OK
```

Shows title, description, status, dependencies, comments, etc.

#### `bd list`

List issues.

```bash
bd list                    # all open issues
bd list --all              # open and closed
bd list --closed           # only closed
bd list --type bug         # only bugs
bd list --priority high    # only high priority
bd list --label backend    # with label
bd list --parent bd-a1b2   # children of issue
bd list --roots            # only root issues (no parent)
```

**Flags:**
- `--all, -a` - Include closed issues
- `--closed` - Only closed issues
- `--type, -t` - Filter by type
- `--priority, -p` - Filter by priority
- `--label, -l` - Filter by label (can repeat, must have all)
- `--assignee` - Filter by assignee
- `--parent` - Filter by parent
- `--roots` - Only issues without a parent
- `--format, -f` - Output format: `short` (default), `long`, `ids`

#### `bd update <id>`

Update an issue's fields.

```bash
bd update bd-a1b2 --title "New title"
bd update bd-a1b2 --priority critical
bd update bd-a1b2 --status in-progress
bd update bd-a1b2 --add-label urgent --remove-label backlog
bd update bd-a1b2 --assignee alice
bd update bd-a1b2 --description -   # read from stdin
```

**Flags:**
- `--title` - New title
- `--description` - New description
- `--priority, -p` - New priority
- `--type, -t` - New type
- `--status, -s` - New status
- `--assignee, -a` - New assignee (empty string to unassign)
- `--add-label` - Add label (can repeat)
- `--remove-label` - Remove label (can repeat)

#### `bd close <id>`

Close an issue.

```bash
bd close bd-a1b2
bd close bd-a1b2 bd-c3d4 bd-e5f6   # close multiple
```

Moves the issue from `open/` to `closed/`, sets status to "closed", sets closed_at timestamp.

#### `bd reopen <id>`

Reopen a closed issue.

```bash
bd reopen bd-a1b2
```

Moves the issue from `closed/` to `open/`, sets status to "open", clears closed_at.

#### `bd delete <id>`

Permanently delete an issue.

```bash
bd delete bd-a1b2
bd delete bd-a1 --force   # skip confirmation, prefix match
```

**Flags:**
- `--force, -f` - Skip confirmation prompt

### Dependency Commands

#### `bd dep add <from> <to>`

Add a dependency (from depends on to).

```bash
bd dep add bd-a1b2 bd-c3d4   # a1b2 depends on c3d4
```

Locks both issues and updates symmetrically:
- Adds `bd-c3d4` to `bd-a1b2.depends_on`
- Adds `bd-a1b2` to `bd-c3d4.dependents`

#### `bd dep remove <from> <to>`

Remove a dependency.

```bash
bd dep remove bd-a1b2 bd-c3d4
```

#### `bd dep list <id>`

List an issue's dependencies.

```bash
bd dep list bd-a1b2           # show what a1b2 depends on and what it blocks
bd dep list bd-a1b2 --tree    # show full dependency tree
```

### Hierarchy Commands

#### `bd parent set <child> <parent>`

Set an issue's parent.

```bash
bd parent set bd-a1b2 bd-c3d4   # a1b2 is now a child of c3d4
```

Locks all involved issues and updates symmetrically:
- Sets `bd-a1b2.parent` to `bd-c3d4`
- Adds `bd-a1b2` to `bd-c3d4.children`
- If a1b2 had a previous parent, removes a1b2 from that parent's children

#### `bd parent remove <child>`

Remove an issue's parent (make it a root issue).

```bash
bd parent remove bd-a1b2
```

#### `bd children <id>`

List an issue's children.

```bash
bd children bd-a1b2
bd children bd-a1b2 --tree    # show full subtree
```

### Comment Commands

#### `bd comment add <id>`

Add a comment to an issue.

```bash
bd comment add bd-a1b2 "This looks good to merge"
bd comment add bd-a1b2 -   # read from stdin
```

#### `bd comment list <id>`

List comments on an issue.

```bash
bd comment list bd-a1b2
```

### Utility Commands

#### `bd ready`

List issues that are ready to work on (open, not blocked).

```bash
bd ready
bd ready --priority high   # only high priority
```

An issue is "ready" if:
- Status is "open" (not in-progress, blocked, deferred, or closed)
- All issues in `depends_on` are closed

#### `bd blocked`

List issues that are blocked.

```bash
bd blocked
```

Shows each blocked issue and what it's waiting on.

#### `bd doctor`

Check for and fix inconsistencies.

```bash
bd doctor          # check only
bd doctor --fix    # check and fix
```

Checks for:
- Status field doesn't match directory (open/ vs closed/)
- Broken dependency references (depends_on/blocks point to non-existent issues)
- Broken parent/child references
- Orphaned lock files
- Malformed JSON files

#### `bd stats`

Show statistics.

```bash
bd stats
```

Output:
```
Open issues:     42
  In progress:   3
  Blocked:       5
  Deferred:      2
Closed issues:   128
Total:           170
```

#### `bd search <query>`

Search issue titles and descriptions.

```bash
bd search "authentication"
bd search "login" --all        # include closed
bd search "bug" --title-only   # only search titles
```

### Git Integration Commands

#### `bd compact`

Remove old closed issues to reduce repository size.

```bash
bd compact --before 2024-01-01          # delete closed before date
bd compact --older-than 90d             # delete closed older than 90 days
bd compact --dry-run                    # show what would be deleted
```

**Flags:**
- `--before` - Delete issues closed before this date
- `--older-than` - Delete issues closed more than this duration ago
- `--dry-run` - Show what would be deleted without deleting
- `--force` - Skip confirmation

## Command Implementation Structure

Commands are implemented separately from storage, using the Storage interface:

```go
package cmd

import (
    "context"
    "fmt"

    "beads-lite/storage"
    "github.com/spf13/cobra"
)

// App holds application state shared across commands
type App struct {
    Storage storage.Storage
    Out     io.Writer
    Err     io.Writer
    JSON    bool   // output in JSON format
}

func NewListCmd(app *App) *cobra.Command {
    var (
        all      bool
        closed   bool
        typeFlag string
        priority string
        labels   []string
        parent   string
        roots    bool
        format   string
    )

    cmd := &cobra.Command{
        Use:   "list",
        Short: "List issues",
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()

            filter := &storage.ListFilter{}

            if closed {
                s := storage.StatusClosed
                filter.Status = &s
            } else if !all {
                // Default: open issues only
                // This is handled by the storage layer
            }

            if typeFlag != "" {
                t := storage.IssueType(typeFlag)
                filter.Type = &t
            }

            // ... apply other filters ...

            issues, err := app.Storage.List(ctx, filter)
            if err != nil {
                return err
            }

            if app.JSON {
                return json.NewEncoder(app.Out).Encode(issues)
            }

            for _, issue := range issues {
                fmt.Fprintf(app.Out, "%s  %s\n", issue.ID, issue.Title)
            }

            return nil
        },
    }

    cmd.Flags().BoolVarP(&all, "all", "a", false, "Include closed issues")
    cmd.Flags().BoolVar(&closed, "closed", false, "Only closed issues")
    cmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Filter by type")
    cmd.Flags().StringVarP(&priority, "priority", "p", "", "Filter by priority")
    cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Filter by label")
    cmd.Flags().StringVar(&parent, "parent", "", "Filter by parent")
    cmd.Flags().BoolVar(&roots, "roots", false, "Only root issues")
    cmd.Flags().StringVarP(&format, "format", "f", "short", "Output format")

    return cmd
}
```

## Configuration

Configuration is stored in `.beads/config.yaml`:

```yaml
# Default values for new issues
defaults:
  priority: medium
  type: task

# ID generation
id:
  prefix: "bd-"
  length: 4

# Actor (used for comments, audit)
actor: "${USER}"

# Project data location
project:
  name: "issues"
```

Configuration is loaded once at startup and passed to commands. The config provider:
- Resolves the config path by searching upward for `.beads/config.yaml` (or defaults to `./.beads/config.yaml` when none is found).
- Resolves the data path by using the configured project name (`.beads/<project.name>`).
- Fails fast with a helpful `bd init` message when config or data paths are missing.

## Git Merge Conflict Handling

When multiple users or agents edit the same issue on different branches, git merge will produce invalid JSON. This is an inherent trade-off of the filesystem approach.

**Prevention strategies:**

1. **Short-lived branches:** Merge frequently to minimize divergence
2. **Issue ownership:** Assign issues to avoid concurrent edits
3. **Append-only patterns:** Comments are append-only, reducing conflicts

**Detection:**

```bash
# After a merge, run doctor to find corrupted files
bd doctor
```

**Resolution:**

```bash
# bd doctor --fix attempts auto-repair
bd doctor --fix

# For manual resolution, issues are human-readable JSON
# Edit the file directly, then verify:
cat .beads/<project>/open/bd-a1b2.json | jq .  # validates JSON
```

**Recommended .gitattributes:**

```gitattributes
# Treat issue files as union merge (append both versions)
# This creates invalid JSON but preserves all data for manual resolution
.beads/**/*.json merge=union
```

Alternative: A custom merge driver could be implemented to merge JSON intelligently:

```gitattributes
.beads/**/*.json merge=beads-json
```

```bash
# In .git/config or global config:
[merge "beads-json"]
    name = Beads JSON merge driver
    driver = bd merge-driver %O %A %B %P
```

The merge driver would:
1. Parse both versions as JSON
2. Merge comments arrays (union, dedupe by comment ID)
3. Take later `updated_at` for scalar conflicts
4. Mark file for manual review if structural conflicts exist

## Future Work: Molecules and Formulas

This section describes features to be implemented after the core system is stable.

### Molecules

Molecules are reusable issue templates with pre-defined structure:

```yaml
# .beads/molecules/feature-development.yaml
name: feature-development
description: Standard feature development workflow
issues:
  - id: design
    title: "Design: {{.Title}}"
    type: task
  - id: implement
    title: "Implement: {{.Title}}"
    type: feature
    depends_on: [design]
  - id: test
    title: "Test: {{.Title}}"
    type: task
    depends_on: [implement]
  - id: review
    title: "Review: {{.Title}}"
    type: task
    depends_on: [test]
```

**Usage:**
```bash
bd mol create feature-development --set Title="User Authentication"
```

Creates all four issues with proper dependencies.

### Formulas

Formulas are scripted workflows that can create, update, and manage issues programmatically:

```yaml
# .beads/formulas/triage-bugs.yaml
name: triage-bugs
description: Auto-triage bugs based on labels
on:
  create:
    type: bug
actions:
  - if: "has_label('crash')"
    then:
      set_priority: critical
      add_label: urgent
  - if: "has_label('ui')"
    then:
      set_priority: low
```

### Storage Interface Extensions

For molecules and formulas, the storage interface would be extended:

```go
// MoleculeStorage handles molecule templates
type MoleculeStorage interface {
    // ListMolecules returns all available molecule templates
    ListMolecules(ctx context.Context) ([]*Molecule, error)

    // GetMolecule returns a molecule by name
    GetMolecule(ctx context.Context, name string) (*Molecule, error)

    // InstantiateMolecule creates issues from a molecule template
    InstantiateMolecule(ctx context.Context, name string, vars map[string]string) ([]string, error)
}

// FormulaStorage handles formula automation
type FormulaStorage interface {
    // ListFormulas returns all available formulas
    ListFormulas(ctx context.Context) ([]*Formula, error)

    // EvaluateFormulas runs applicable formulas for an event
    EvaluateFormulas(ctx context.Context, event Event) error
}
```

These would be implemented as separate packages that compose with the core Storage interface.

## Migration from v1

For users migrating from the current beads (SQLite + JSONL):

```bash
bd migrate-v2          # creates new .beads-v2/ from existing data
bd migrate-v2 --verify # verify migration without switching
bd migrate-v2 --switch # switch to v2 as primary
```

The migration reads from SQLite and writes to the new filesystem format.

## Performance Characteristics (Filesystem Engine)

These benchmarks are specific to the **Filesystem storage engine**. Other engines will have different characteristics (e.g., SQLite may be faster for complex queries but slower for git operations).

| Operation | Expected Time | Notes |
|-----------|---------------|-------|
| `bd create` | 1-5ms | Generate ID, write one file |
| `bd show <id>` | <1ms | Read one file |
| `bd list` | 1-50ms | Read directory, optionally read files for filtering |
| `bd list --all` | 2-100ms | Read two directories |
| `bd close <id>` | 2-10ms | Lock, update, move file |
| `bd dep add A B` | 5-20ms | Lock two files, update both |
| `bd search <query>` | 10-200ms | Read all issue files, search content |

For a repository with 1000 open issues and 5000 closed issues:
- `bd list`: ~50ms (read 1000 small JSON files)
- `bd list --all`: ~200ms (read 6000 files)
- `bd show`: <1ms (read 1 file)

These are rough estimates; actual performance depends on filesystem and disk speed.

## Design Decisions Summary

**Architecture-wide decisions (all engines):**

1. **Storage interface** - Commands don't know about storage details. Enables swappable backends.

2. **No daemon** - Every command is stateless. Fast startup, no background processes.

3. **Contract tests** - All engines must pass the same interface contract tests.

**Filesystem engine decisions:**

4. **No index file** - Filesystem is the source of truth. Eliminates sync bugs.

5. **Flat file structure** - `<id>.json` files, not nested directories. Simpler, git-friendlier.

6. **open/ and closed/ directories** - Status visible in filesystem. Easy compaction.

7. **flock-based locking** - Simple, kernel-managed, works across processes.

8. **Sorted lock acquisition** - Prevents deadlocks in multi-issue operations.

9. **JSON files** - Human-readable, git-diff-friendly, easy to debug.

10. **Atomic writes** - Write-to-temp-then-rename pattern prevents corruption on crash.

---

# Testing Strategy

Testing beads requires verifying correctness under concurrent access, crash scenarios, and data integrity over time. This section describes the testing approach for all storage engines.

## Test Categories

### 1. Interface Contract Tests

Every storage engine must pass the same contract test suite:

```go
// storage/contract_test.go
package storage_test

// RunContractTests runs all interface contract tests against a storage implementation
func RunContractTests(t *testing.T, factory func() Storage) {
    t.Run("Create", func(t *testing.T) { testCreate(t, factory()) })
    t.Run("Get", func(t *testing.T) { testGet(t, factory()) })
    t.Run("Update", func(t *testing.T) { testUpdate(t, factory()) })
    t.Run("Delete", func(t *testing.T) { testDelete(t, factory()) })
    t.Run("List", func(t *testing.T) { testList(t, factory()) })
    t.Run("Dependencies", func(t *testing.T) { testDependencies(t, factory()) })
    t.Run("Hierarchy", func(t *testing.T) { testHierarchy(t, factory()) })
    t.Run("CycleDetection", func(t *testing.T) { testCycleDetection(t, factory()) })
    // ...
}

// Each storage engine runs the same tests:
func TestFilesystemStorage(t *testing.T) {
    RunContractTests(t, func() Storage {
        dir := t.TempDir()
        s := filesystem.New(dir)
        s.Init(context.Background())
        return s
    })
}

func TestSQLiteStorage(t *testing.T) {
    RunContractTests(t, func() Storage {
        // ...
    })
}
```

### 2. Concurrent Access Tests

Critical for verifying locking correctness:

```go
// storage/concurrent_test.go

func TestConcurrentCreates(t *testing.T) {
    s := setupStorage(t)
    var wg sync.WaitGroup
    var ids sync.Map
    errors := make(chan error, 100)

    // Spawn 100 goroutines creating issues simultaneously
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            issue := &Issue{Title: fmt.Sprintf("Issue %d", n)}
            id, err := s.Create(context.Background(), issue)
            if err != nil {
                errors <- err
                return
            }
            // Check for ID collision
            if _, loaded := ids.LoadOrStore(id, true); loaded {
                errors <- fmt.Errorf("duplicate ID: %s", id)
            }
        }(i)
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        t.Error(err)
    }
}

func TestConcurrentUpdatesToSameIssue(t *testing.T) {
    s := setupStorage(t)
    id, _ := s.Create(ctx, &Issue{Title: "Original"})

    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            issue, _ := s.Get(ctx, id)
            issue.Title = fmt.Sprintf("Updated by %d", n)
            s.Update(ctx, issue)
        }(i)
    }
    wg.Wait()

    // Verify issue is not corrupted
    final, err := s.Get(ctx, id)
    require.NoError(t, err)
    require.NotEmpty(t, final.Title)
}

func TestConcurrentDependencyAddition(t *testing.T) {
    // This tests the deadlock prevention (sorted lock acquisition)
    s := setupStorage(t)

    // Create issues
    ids := make([]string, 10)
    for i := range ids {
        ids[i], _ = s.Create(ctx, &Issue{Title: fmt.Sprintf("Issue %d", i)})
    }

    var wg sync.WaitGroup
    // Many goroutines adding dependencies in different orders
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            a := ids[rand.Intn(len(ids))]
            b := ids[rand.Intn(len(ids))]
            if a != b {
                s.AddDependency(ctx, a, b)  // Should not deadlock
            }
        }()
    }

    // If this doesn't complete in 5 seconds, we have a deadlock
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // Success
    case <-time.After(5 * time.Second):
        t.Fatal("deadlock detected")
    }
}
```

### 3. Multi-Process Concurrent Access Tests

Goroutine tests verify in-process locking; these verify cross-process locking:

```bash
#!/bin/bash
# test/multiprocess_stress.sh

set -e

BEADS_DIR=$(mktemp -d)
trap "rm -rf $BEADS_DIR" EXIT

bd init --path "$BEADS_DIR"

# Spawn 20 processes doing concurrent creates
for i in $(seq 1 20); do
    bd create "Issue $i" --path "$BEADS_DIR" &
done
wait

# Verify all 20 were created
COUNT=$(bd list --path "$BEADS_DIR" --format ids | wc -l)
if [ "$COUNT" -ne 20 ]; then
    echo "FAIL: Expected 20 issues, got $COUNT"
    exit 1
fi

echo "PASS: Multi-process creates"
```

```bash
#!/bin/bash
# test/multiprocess_update_race.sh
# Tests that concurrent updates don't corrupt data

set -e

BEADS_DIR=$(mktemp -d)
trap "rm -rf $BEADS_DIR" EXIT

bd init --path "$BEADS_DIR"
ID=$(bd create "Test Issue" --path "$BEADS_DIR")

# 50 processes updating the same issue
for i in $(seq 1 50); do
    bd update "$ID" --title "Updated by $i" --path "$BEADS_DIR" &
done
wait

# Verify issue is still valid JSON and readable
bd show "$ID" --path "$BEADS_DIR" --json | jq . > /dev/null
if [ $? -ne 0 ]; then
    echo "FAIL: Issue corrupted"
    exit 1
fi

# Verify no doctor issues
PROBLEMS=$(bd doctor --path "$BEADS_DIR" 2>&1 | grep -c "problem" || true)
if [ "$PROBLEMS" -ne 0 ]; then
    echo "FAIL: Doctor found problems"
    bd doctor --path "$BEADS_DIR"
    exit 1
fi

echo "PASS: Concurrent updates to same issue"
```

### 4. Data Integrity Tests

Verify data survives create/read/update/delete cycles:

```go
func TestDataIntegrityRoundTrip(t *testing.T) {
    s := setupStorage(t)

    // Create issue with all fields populated
    original := &Issue{
        Title:       "Test Issue",
        Description: "A detailed description\nwith newlines\nand unicode: 日本語",
        Status:      StatusOpen,
        Priority:    PriorityHigh,
        Type:        TypeFeature,
        Labels:      []string{"backend", "urgent"},
        Assignee:    "alice",
        Comments: []Comment{
            {ID: "c-1", Author: "bob", Body: "Comment 1", CreatedAt: time.Now()},
            {ID: "c-2", Author: "carol", Body: "Comment 2", CreatedAt: time.Now()},
        },
    }

    id, err := s.Create(ctx, original)
    require.NoError(t, err)
    original.ID = id

    // Read back and compare
    retrieved, err := s.Get(ctx, id)
    require.NoError(t, err)

    // Deep equality check (ignoring timestamps set by storage)
    assert.Equal(t, original.Title, retrieved.Title)
    assert.Equal(t, original.Description, retrieved.Description)
    assert.Equal(t, original.Labels, retrieved.Labels)
    assert.Len(t, retrieved.Comments, 2)
}

func TestLargeDataSet(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping large dataset test in short mode")
    }

    s := setupStorage(t)

    // Create 1000 issues
    ids := make([]string, 1000)
    for i := range ids {
        id, err := s.Create(ctx, &Issue{
            Title:       fmt.Sprintf("Issue %d", i),
            Description: strings.Repeat("x", 1000),  // 1KB each
        })
        require.NoError(t, err)
        ids[i] = id
    }

    // Verify all readable
    for _, id := range ids {
        _, err := s.Get(ctx, id)
        require.NoError(t, err, "failed to read issue %s", id)
    }

    // Verify list returns all
    all, err := s.List(ctx, nil)
    require.NoError(t, err)
    assert.Len(t, all, 1000)

    // Run doctor
    problems, err := s.Doctor(ctx, false)
    require.NoError(t, err)
    assert.Empty(t, problems)
}
```

### 5. Crash Recovery Tests

Simulate crashes at various points:

```go
func TestCrashDuringWrite(t *testing.T) {
    // This test uses a mock filesystem that fails mid-write
    mockFS := &CrashingFS{
        crashAfterBytes: 50,  // Crash after writing 50 bytes
    }
    s := filesystem.NewWithFS(t.TempDir(), mockFS)
    s.Init(ctx)

    // This should fail
    _, err := s.Create(ctx, &Issue{Title: "Test"})
    require.Error(t, err)

    // Storage should still be in valid state
    // (no partial files, no corruption)
    problems, err := s.Doctor(ctx, false)
    require.NoError(t, err)
    assert.Empty(t, problems, "crash left storage in inconsistent state")
}

func TestCrashDuringClose(t *testing.T) {
    s := setupStorage(t)
    id, _ := s.Create(ctx, &Issue{Title: "Test"})

    // Simulate: file written to closed/ but not removed from open/
    // (crash between write and delete)
    openPath := filepath.Join(s.root, "open", id+".json")
    closedPath := filepath.Join(s.root, "closed", id+".json")

    data, _ := os.ReadFile(openPath)
    os.WriteFile(closedPath, data, 0644)
    // Don't remove openPath - simulating crash

    // Doctor should detect and fix
    problems, _ := s.Doctor(ctx, false)
    assert.NotEmpty(t, problems, "should detect duplicate issue")

    _, err := s.Doctor(ctx, true)  // Fix
    require.NoError(t, err)

    // Now should be clean
    problems, _ = s.Doctor(ctx, false)
    assert.Empty(t, problems)
}
```

### 6. Git Integration Tests

Test behavior after git operations:

```bash
#!/bin/bash
# test/git_merge_conflict.sh

set -e

# Setup two branches that edit the same issue
REPO=$(mktemp -d)
cd "$REPO"
git init
bd init

ID=$(bd create "Original title")
git add .beads
git commit -m "Initial"

# Branch A: change title
git checkout -b branch-a
bd update "$ID" --title "Title from A"
git commit -am "Update from A"

# Branch B: change title differently
git checkout main
git checkout -b branch-b
bd update "$ID" --title "Title from B"
git commit -am "Update from B"

# Merge should conflict
git checkout main
git merge branch-a
git merge branch-b || true  # Expected to conflict

# bd doctor should detect the problem
bd doctor 2>&1 | grep -q "invalid JSON\|parse error" || {
    echo "FAIL: Doctor didn't detect merge conflict corruption"
    exit 1
}

echo "PASS: Git merge conflict detected by doctor"
```

## Test Fixtures and Generators

For complex scenario testing, use generators:

```go
// testutil/generator.go

type IssueGenerator struct {
    storage Storage
    ids     []string
}

// GenerateTree creates a hierarchy of issues
func (g *IssueGenerator) GenerateTree(depth, breadth int) error {
    return g.generateTreeRecursive("", depth, breadth)
}

func (g *IssueGenerator) generateTreeRecursive(parent string, depth, breadth int) error {
    if depth == 0 {
        return nil
    }
    for i := 0; i < breadth; i++ {
        issue := &Issue{
            Title:  fmt.Sprintf("Level %d Issue %d", depth, i),
            Parent: parent,
        }
        id, err := g.storage.Create(ctx, issue)
        if err != nil {
            return err
        }
        g.ids = append(g.ids, id)
        if err := g.generateTreeRecursive(id, depth-1, breadth); err != nil {
            return err
        }
    }
    return nil
}

// GenerateDependencyChain creates A -> B -> C -> ... chain
func (g *IssueGenerator) GenerateDependencyChain(length int) ([]string, error) {
    ids := make([]string, length)
    for i := 0; i < length; i++ {
        id, err := g.storage.Create(ctx, &Issue{Title: fmt.Sprintf("Chain %d", i)})
        if err != nil {
            return nil, err
        }
        ids[i] = id
        if i > 0 {
            if err := g.storage.AddDependency(ctx, id, ids[i-1]); err != nil {
                return nil, err
            }
        }
    }
    return ids, nil
}

// GenerateDependencyDAG creates a complex dependency graph
func (g *IssueGenerator) GenerateDependencyDAG(nodes, edges int) error {
    // Create nodes
    ids := make([]string, nodes)
    for i := range ids {
        id, _ := g.storage.Create(ctx, &Issue{Title: fmt.Sprintf("Node %d", i)})
        ids[i] = id
    }

    // Create random edges (dependencies)
    for i := 0; i < edges; i++ {
        a := rand.Intn(nodes)
        b := rand.Intn(nodes)
        if a != b && a < b {  // Only forward edges to avoid cycles
            g.storage.AddDependency(ctx, ids[b], ids[a])
        }
    }
    return nil
}
```

## CI Integration

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -race -coverprofile=coverage.out

  stress:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test ./... -race -count=10  # Run 10 times to catch races

  multiprocess:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go build -o bd ./cmd/bd
      - run: ./test/multiprocess_stress.sh
      - run: ./test/multiprocess_update_race.sh

  large-dataset:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test ./... -run Large -timeout 10m
```

## Benchmarks

```go
func BenchmarkCreate(b *testing.B) {
    s := setupStorage(b)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        s.Create(ctx, &Issue{Title: "Benchmark"})
    }
}

func BenchmarkListOpen1000(b *testing.B) {
    s := setupStorage(b)
    for i := 0; i < 1000; i++ {
        s.Create(ctx, &Issue{Title: fmt.Sprintf("Issue %d", i)})
    }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        s.List(ctx, nil)
    }
}

func BenchmarkConcurrentReads(b *testing.B) {
    s := setupStorage(b)
    id, _ := s.Create(ctx, &Issue{Title: "Test"})
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            s.Get(ctx, id)
        }
    })
}
```

---

# Gas Town Compatibility Checklist

Commands and flags required by Gas Town (see `GAS_TOWN_REQUIREMENTS.md`). Checked items are already implemented and passing in e2e tests.

## Global Flags

- [x] `--json` — JSON output
- [ ] `--no-daemon` — No-op flag (beads-lite is daemonless by design, but Gas Town passes it)
- [ ] `-q`, `--quiet` — Quiet mode, minimal output
- [ ] `--allow-stale` — No-op flag (no daemon/cache, but Gas Town passes it)

## Core Commands

- [x] `init` — Initialize bead database
  - [x] `--project` (our equivalent of project naming)
  - [ ] `--prefix <prefix>` — Set issue prefix
  - [ ] `--quiet` — Quiet mode (or covered by global `-q`)
- [x] `create` — Create new beads
  - [x] `--type`, `--title`, `--priority`, `--label`
  - [ ] `--labels <labels>` — Alias for `--label` (Gas Town uses `--labels`)
- [x] `show` — Show bead details (with `--json` via global flag)
- [x] `list` — List beads/issues
  - [x] `--status`, `--type`, `--json`
  - [ ] `--tag=<tag>` — Filter by tag (Gas Town uses `--tag`, we have `--labels`)
- [x] `update` — Update bead status/properties
  - [x] `--status`, `--assignee`
- [x] `close` — Close beads
- [ ] `version` — Return semver version string (Gas Town checks for minimum `0.43.0`)

## `config` — Configuration Management

- [ ] `config get <key>` — Get configuration value
- [ ] `config set <key> <value>` — Set configuration value
- Known keys: `issue_prefix`, `allowed_prefixes`, `types.custom`, `routing.mode`

## `slot` — Slot Operations

- [ ] `slot show <bead-id>` — Show slot data for a bead
- [ ] `slot set <bead-id> <name> <value>` — Set a slot value
- [ ] `slot clear <bead-id> <name>` — Clear a slot value

## `gate` — Gate Operations

- [ ] `gate show <gate-id>` — Show gate details
- [ ] `gate wait <gate-id> --notify <agent-id>` — Wait on a gate with notification

## `swarm` — Swarm Operations

- [ ] `swarm status <swarm-id>` — Get swarm status

## `mol` — Molecule Operations (has sub-sub-commands)

- [ ] `mol current <molecule-id>` — Get current molecule state
- [ ] `mol seed --patrol` — Seed a molecule with patrol
- [ ] `mol wisp create <proto-id> --actor <role>` — Create a wisp (sub-sub-command of `mol wisp`)
- [ ] `mol wisp gc` — Garbage collect wisps (sub-sub-command of `mol wisp`)

## `formula` — Formula Operations

- [ ] `formula show <name>` — Show formula details
  - [ ] `--allow-stale` flag
- [ ] `formula list` — List all formulas

## `agent` — Agent Operations

- [ ] `agent state <bead-id> <state>` — Set agent state
- [ ] `agent heartbeat <agent-bead>` — Send agent heartbeat

## `dep` — Dependency Operations

- [x] `dep list <bead-id>` — List dependencies
  - [x] `--direction=<up|down>`
  - [x] `--type=<type>`
  - [x] `--json` (via global flag)
- [x] `dep add` — Add dependency
- [x] `dep remove` — Remove dependency

## Other Commands

- [ ] `cook <formula-name>` — Cook/execute a formula
- [ ] `import` — Import data
- [ ] `migrate` — Run database migrations
- [ ] `sync --import-only` — Sync operations (import only)
- [ ] `prime` — Prime operations
- [x] `stats` — Show statistics
- [x] `ready` — Ready check
- [ ] `label` — Label operations (standalone command; currently labels are managed via `update --add-label`/`--remove-label`)
- [x] `doctor` — Health/diagnostic checks
- [x] `blocked` — Check blocked status

## Already Implemented (not in Gas Town requirements)

These commands exist in beads-lite but are not required by Gas Town:
- `reopen` — Reopen a closed issue
- `delete` — Permanently delete an issue
- `comment add` / `comment list` — Manage issue comments
- `compact` — Remove old closed issues
- `children` — List an issue's children
- `search` — Search issue titles and descriptions
