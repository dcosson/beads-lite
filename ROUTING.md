# Routing: Cross-Prefix Issue Resolution

## Requirements

### Problem

A Gas Town deployment has multiple rigs, each with its own `.beads` directory.
Each rig uses a different issue ID prefix (e.g., `hq-`, `bl-`, `gt-`). When
you run `bd show bl-1jzo` from the `hq` rig, the local database doesn't have
that issue — it lives in the `bl` rig's `.beads` directory.

Routing resolves this by mapping ID prefixes to rig paths so point-lookup
commands can transparently find issues in other rigs.

### Prefix

The prefix is everything up to and including the first hyphen:
- `bl-1jzo` → prefix `bl-`
- `hq-abc123` → prefix `hq-`

### Routes File

`routes.jsonll` lives in the town root's `.beads/` directory. Each line is a
JSON object with `prefix` and `path` fields:

```
{"prefix": "hq-", "path": "."}
{"prefix": "bl-", "path": "crew/misc"}
{"prefix": "gt-", "path": "crew/max"}
```

- `prefix`: Issue ID prefix including trailing hyphen (e.g. `hq-`).
- `path`: Relative path from town root to the rig directory (the parent of
  that rig's `.beads/`). `"."` means the town root itself.

### Town Root

The top-level directory containing all rigs. For beads-lite, discover it by
walking up from the current `.beads` directory looking for a `.beads/routes.jsonl`
file. If none found, routing is unavailable (not an error — just no routing).

### Redirect Support

Routed `.beads` directories may contain a `redirect` file pointing to a
different `.beads` directory. The routing system must follow redirects (one
level only) when resolving a route target. This reuses the existing redirect
logic in `config/provider.go`.

### Which Commands Route

**Point lookups** (single ID argument): `bd show`, `bd update`, `bd close`,
`bd dep add/remove`, `bd create --dep`, and future commands like
`bd agent state`, `bd slot set/clear`.

**Bulk queries** stay local-only: `bd list`, `bd ready`, `bd blocked`, etc.

### Resolution Flow

```
1. Extract prefix from issue ID
2. Look up prefix in routes.jsonl
   - No routes file or no match → use local store
   - Match resolves to current .beads dir → use local store
3. Resolve route path to .beads directory:
   <town-root>/<route.path>/.beads
4. Follow redirect file if present in target .beads
5. Read project.name from target config.yaml → derive data dir
6. Open filesystem storage at resolved data dir
7. Perform the lookup/operation
```

### Non-Requirements (skip for now)

- Town root auto-detection via `mayor/town.json`
- User role detection (maintainer vs contributor)
- Multi-repo hydration (`repos:` config)
- mtime caching

---

## Architecture

### Package: `internal/routing`

Standalone package. Imports `config`, `issuestorage`, and `filesystem`.

**Core types:**
- `Router` — resolves issue ID prefixes to rig paths via `routes.jsonl`
- `Getter` — implements `issuestorage.IssueGetter` with routing-aware dispatch

**Key functions:**
- `New(beadsDir)` — creates Router by discovering `routes.jsonl` (walks up
  parent dirs). Returns nil if no routes file found.
- `Router.Resolve(issueID)` — returns `(config.Paths, prefix, isRemote, error)`
- `Router.SameStore(id1, id2)` — reports whether two IDs resolve to the same
  storage location. Handles nil Router (returns true).
- `NewGetter(router, local)` — creates a routing-aware `IssueGetter` that
  dispatches `Get` calls to the correct store based on ID prefix. Falls
  through to the local store when router is nil or prefix doesn't match.

### Interface: `issuestorage.IssueGetter`

```go
type IssueGetter interface {
    Get(ctx context.Context, id string) (*Issue, error)
}
```

`IssueStore` implicitly satisfies this. `routing.Getter` also satisfies it.
Used by functions that only need read-by-ID across rigs:
- `graph.FindMoleculeRoot`, `graph.CollectMoleculeChildren`
- `enrichDependencies`, `ToIssueJSON` in jsonformat.go
- Display paths in `show.go` and `dep.go`

### Command integration: `internal/cmd/app.go`

`App.StorageFor(ctx, id)` returns the `IssueStore` for a given issue ID.
Returns the local store when no routing is needed, or opens a temporary
`FilesystemStorage` at the resolved remote path.

Commands call `app.StorageFor(ctx, id)` for write operations and
`routing.NewGetter(app.Router, app.Storage)` for read-only lookups.

### Cross-Store Dependencies

Dependencies can reference issues in different rigs. The command layer in
`dep.go` handles this:

1. Both issue and dependency are resolved to their respective stores via
   `app.StorageFor`.
2. `Router.SameStore(id1, id2)` determines whether they share a store.
3. **Same store**: delegates to `store.AddDependency` / `store.RemoveDependency`
   (handles cycle detection, parent-child, atomicity internally).
4. **Cross-store**: the command layer handles it directly:
   - **Cycle detection**: BFS using `routing.Getter` to traverse cross-store edges
   - **Add**: `store.Modify(issueID, add dep)` + `depStore.Modify(depID, add dependent)`
   - **Remove**: `store.Modify(issueID, remove dep)` + `depStore.Modify(depID, remove dependent)`

**Constraints:**
- Parent-child dependencies must be same-rig (rejected for cross-store).
  Molecules/hierarchy are always within a single rig.
- Cross-store writes are not atomic (two separate Modify calls). `bd doctor`
  detects and fixes asymmetric dependencies.

`bd create --dep` uses the same cross-store path when the dependency target
is in a different rig.
