# Beads v2 Design Specification

## Overview

Beads v2 is a complete redesign of the storage layer, replacing the dual SQLite + JSONL sync architecture with a simple filesystem-based approach. The goal is maximum simplicity, speed, and git-native behavior.

**Core principles:**
- The filesystem is the database
- No sync layer, no daemon, no background processes
- Git handles versioning and merging naturally
- Commands complete in milliseconds, not seconds
- Storage engine is swappable via interface

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

**ID format:** `bd-<8 hex chars>` (e.g., `bd-a1b2c3d4`), generated from random bytes.

## Issue Schema

Each `<id>.json` file contains:

```json
{
  "id": "bd-a1b2c3d4",
  "title": "Implement user authentication",
  "description": "Add login/logout functionality...",
  "status": "open",
  "priority": "high",
  "type": "feature",
  "parent": "bd-e5f6g7h8",
  "children": ["bd-i9j0k1l2"],
  "depends_on": ["bd-m3n4o5p6"],
  "blocks": ["bd-q7r8s9t0"],
  "labels": ["auth", "backend"],
  "assignee": "alice",
  "comments": [
    {
      "id": "c-a1b2c3d4",
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

**Status values:** `open`, `in-progress`, `blocked`, `deferred`, `closed`

**Type values:** `task`, `bug`, `feature`, `epic`, `chore`

**Priority values:** `critical`, `high`, `medium`, `low`

Note: The `status` field in JSON and the directory location must agree. The directory is authoritative; if they disagree, the file should be moved to match its status or vice versa. The `bd doctor` command can detect and fix such inconsistencies.

## Locking Strategy

### Issue Locks

Each issue has a corresponding `.lock` file used with `flock(2)`:

```
open/bd-a1b2c3d4.json   # issue data
open/bd-a1b2c3d4.lock   # lock file
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

Example for `bd dep add A B`:
```
sorted = sort(["bd-a1b2", "bd-c3d4"])  # consistent order
lock(sorted[0])
lock(sorted[1])
# update A.depends_on, B.blocks
unlock(sorted[1])
unlock(sorted[0])
```

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
    Parent      string            `json:"parent,omitempty"`
    Children    []string          `json:"children,omitempty"`
    DependsOn   []string          `json:"depends_on,omitempty"`
    Blocks      []string          `json:"blocks,omitempty"`
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
    // Updates both A.depends_on and B.blocks atomically
    AddDependency(ctx context.Context, dependentID, dependencyID string) error

    // RemoveDependency removes a dependency relationship
    RemoveDependency(ctx context.Context, dependentID, dependencyID string) error

    // SetParent sets the parent of an issue
    // Updates both child.parent and parent.children atomically
    SetParent(ctx context.Context, childID, parentID string) error

    // RemoveParent removes the parent relationship
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

    "beads/storage"
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

Creates `.beads/open/` and `.beads/closed/` directories.

#### `bd create`

Create a new issue.

```bash
bd create "Fix login bug"
bd create "Add OAuth support" --type feature --priority high
bd create "Implement caching" --parent bd-a1b2c3d4
bd create "Write tests" --depends-on bd-e5f6g7h8
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
bd show bd-a1b2c3d4
bd show bd-a1b2     # prefix matching OK
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
bd close bd-a1b2c3d4
bd close bd-a1b2 bd-c3d4 bd-e5f6   # close multiple
```

Moves the issue from `open/` to `closed/`, sets status to "closed", sets closed_at timestamp.

#### `bd reopen <id>`

Reopen a closed issue.

```bash
bd reopen bd-a1b2c3d4
```

Moves the issue from `closed/` to `open/`, sets status to "open", clears closed_at.

#### `bd delete <id>`

Permanently delete an issue.

```bash
bd delete bd-a1b2c3d4
bd delete bd-a1b2 --force   # skip confirmation
```

**Flags:**
- `--force, -f` - Skip confirmation prompt

### Dependency Commands

#### `bd dep add <from> <to>`

Add a dependency (from depends on to).

```bash
bd dep add bd-a1b2 bd-c3d4   # a1b2 depends on c3d4
```

Updates both issues atomically:
- Adds `bd-c3d4` to `bd-a1b2.depends_on`
- Adds `bd-a1b2` to `bd-c3d4.blocks`

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

Updates both issues atomically.

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

    "beads/storage"
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
  length: 8

# Actor (used for comments, audit)
actor: "${USER}"
```

Configuration is loaded once at startup and passed to commands.

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

## Performance Characteristics

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

1. **No index file** - Filesystem is the source of truth. Eliminates sync bugs.

2. **Flat file structure** - `<id>.json` files, not nested directories. Simpler, git-friendlier.

3. **open/ and closed/ directories** - Status visible in filesystem. Easy compaction.

4. **flock-based locking** - Simple, kernel-managed, works across processes.

5. **Sorted lock acquisition** - Prevents deadlocks in multi-issue operations.

6. **Storage interface** - Commands don't know about filesystem details. Enables SQLite backend later.

7. **No daemon** - Every command is stateless. Fast startup, no background processes.

8. **JSON files** - Human-readable, git-diff-friendly, easy to debug.
