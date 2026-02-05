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
