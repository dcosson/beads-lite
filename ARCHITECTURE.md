# Architecture

## Overview

beads-lite uses a layered architecture with clear separation between business logic and storage:

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Commands (cmd/)                      │
│                  create, show, dep add, etc.                 │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  IssueService (issueservice/)                │
│                                                              │
│  • Routing: dispatch to correct storage by issue ID prefix   │
│  • Dependency validation: cycle detection (BFS traversal)    │
│  • Parent-child constraints: same-rig requirement            │
│  • Reparenting: move child between parents atomically        │
│  • Bidirectional linking: maintain Dependencies/Dependents   │
└──────────┬─────────────────────────────┬────────────────────┘
           │                             │
           ▼                             ▼
┌─────────────────────┐       ┌─────────────────────┐
│   Local Storage     │       │   Remote Storage    │
│   (filesystem/)     │       │   (filesystem/)     │
│                     │       │                     │
│   Pure CRUD ops:    │       │   Another repo's    │
│   Create, Get,      │       │   .beads directory  │
│   Modify, Delete,   │       │   accessed via      │
│   List              │       │   routing rules     │
└─────────────────────┘       └─────────────────────┘
```

## Layers

### CLI Commands (`internal/cmd/`)

Commands parse flags/args and call into `IssueService`. They don't contain business logic for dependencies or routing—that's delegated to the service layer.

### IssueService (`internal/issueservice/`)

The service layer wraps storage backends and handles all domain logic:

**Routing** — When configured, dispatches operations to the correct storage backend based on issue ID prefix. For example, `ext-42` might route to a different repo's `.beads` directory while `bd-1` stays local.

**Dependency validation** — Before adding a dependency, performs BFS cycle detection across the dependency graph. This works across storage boundaries (a local issue can depend on a remote issue without creating cycles).

**Parent-child constraints** — Parent-child relationships must be within the same storage ("rig"). Cross-rig parent-child is rejected. Hierarchy cycles are detected by walking the ancestor chain.

**Reparenting** — When a child's parent changes, the service atomically removes the child from the old parent's dependents and adds it to the new parent's dependents.

**Bidirectional linking** — Dependencies are stored on both sides: the source issue's `Dependencies` array and the target issue's `Dependents` array. The service maintains this invariant.

### Storage (`internal/issuestorage/`)

The storage layer is pure CRUD with no business logic:

- `issuestorage.IssueStore` — interface defining Create, Get, Modify, Delete, List, etc.
- `issuestorage/filesystem` — JSON files on disk, one file per issue
- Storage backends don't know about cycles, parent-child rules, or routing

## Why This Separation?

1. **Testability** — Storage can be tested with simple CRUD operations. Service logic can be tested with mock storage.

2. **Single responsibility** — Filesystem code handles file I/O and locking. Service code handles graph invariants.

3. **Routing transparency** — Commands don't need to know whether an issue is local or remote. `app.Storage.Get(ctx, "ext-42")` just works.

4. **Consistent validation** — All code paths go through IssueService, so cycle detection and parent-child rules are always enforced.

## Key Types

```go
// Storage interface (pure CRUD)
type IssueStore interface {
    Create(ctx, issue) (id, error)
    Get(ctx, id) (*Issue, error)
    Modify(ctx, id, func(*Issue) error) error
    Delete(ctx, id) error
    List(ctx, filter) ([]*Issue, error)
    // ...
}

// Service layer (wraps storage, adds business logic)
type IssueService struct {
    router *routing.Router      // optional, for multi-repo
    local  IssueStore           // primary storage
    stores map[string]IssueStore // cached remote stores
}

// Service-only methods (not on interface)
func (s *IssueService) AddDependency(ctx, issueID, dependsOnID, depType) error
func (s *IssueService) RemoveDependency(ctx, issueID, dependsOnID) error
```

## Routing

Routing is configured in `.beads/routing.yaml`:

```yaml
routes:
  ext: /path/to/other/repo/.beads
```

With this config:
- `bd-1` → local storage
- `ext-42` → `/path/to/other/repo/.beads`

The IssueService caches opened remote stores for the session lifetime.

## Pluggable Storage Pattern

beads-lite uses a consistent pattern for pluggable storage engines that avoids import cycles:

```
┌──────────────────────────────┐
│     Service Layer            │  (issueservice/, configservice/)
│  - Business logic            │
│  - Can import storage +      │
│    storage engines           │
└──────────────┬───────────────┘
               │ imports
               ▼
┌──────────────────────────────┐
│     Storage Engine           │  (issuestorage/filesystem/, config/yamlstore/)
│  - Concrete implementation   │
│  - Imports base types only   │
└──────────────┬───────────────┘
               │ imports
               ▼
┌──────────────────────────────┐
│     Base Types/Interface     │  (issuestorage/, config/)
│  - Interface definitions     │
│  - Data types (Issue, Paths) │
│  - No implementation deps    │
└──────────────────────────────┘
```

This three-layer structure allows:

1. **Storage engines to be pluggable** — The interface lives in the base package, implementations in subpackages
2. **Service layer to use concrete engines** — Services can import both base types and specific engine implementations
3. **No import cycles** — Base types don't import engines, engines don't import services

Examples:

- **Issue storage**: `issuestorage.IssueStore` interface → `issuestorage/filesystem.Store` implementation → `issueservice.IssueStore` service
- **Config storage**: `config.Store` interface + `config.Paths` type → `config/yamlstore.YAMLStore` implementation → `configservice.ResolvePaths()` service functions

**Note**: `kvstorage/` (used for slots, agents, merge-slots) intentionally does not follow this pattern. It's a simple key-value store with no business logic, so commands use the storage layer directly without a service wrapper.

## Config Discovery

The `configservice` package handles finding the `.beads` directory. Discovery order:

1. **`BEADS_DIR` env var** — If set, use that path directly (fastest path)
2. **Walk up from CWD** — Look for `.beads/config.yaml` in current directory, then parent, etc.
3. **Git worktree fallback** — If not found and in a git worktree, also check the main repo root

### Git Root Boundary

When walking up, we stop at the git repository root to avoid escaping repo boundaries. This is detected by looking for a `.git` directory or file (for worktrees), not by running `git rev-parse`.

**Why not subprocess?** Performance. Each `git rev-parse` call takes ~5ms due to process spawn overhead. With pure file walk-up, detection takes ~0.004ms (1,300x faster). For 20 commands, this saves ~100ms.

**Edge cases** where behavior differs from `git rev-parse`:

- `GIT_DIR` / `GIT_WORK_TREE` env vars: These override where git looks for the repo, but beads-lite ignores them. We use the physical `.git` location.
- If `GIT_DIR` points elsewhere but there's a local `.git`, we stop at the local `.git` boundary.
- If `GIT_WORK_TREE` defines a virtual boundary with no physical `.git`, we walk past it.

These are obscure configurations. For typical usage (normal repos, worktrees, submodules), behavior is identical. The tradeoff is: we optimize for the 99.9% case at the cost of slightly different behavior for exotic git setups.

### Redirect Files

A `.beads/redirect` file can point to a different `.beads` directory. This is followed during discovery (one level only). Used for shared beads directories across multiple repos.

### Future: Configurable Discovery Mode

Currently beads-lite uses the `git-dir` behavior (stop at `.git`). A future enhancement could make this configurable:

```yaml
# .beads/config.yaml
discovery_mode: git-dir  # default
```

| Mode | Behavior |
|------|----------|
| `git-dir` (default, current) | Walk up from CWD, stop at `.git` directory/file |
| `git-worktree` | Use `$GIT_WORK_TREE/.beads`. Error if `GIT_WORK_TREE` is not set. |
| `none` | Walk up from CWD to filesystem root, ignore git entirely |

The `BEADS_DIR` env var would continue to take precedence over any mode.
