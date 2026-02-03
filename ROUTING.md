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
`bd dep add`, and future commands like `bd agent state`, `bd slot set/clear`.

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

## Implementation Plan

### Architecture Assessment

The current codebase has clean separation for this:

- **`config.ResolvePaths()`** centralizes all `.beads` directory discovery. It
  returns `Paths{ConfigDir, ConfigFile, DataDir}`.
- **`filesystem.New(root)`** takes a pre-resolved path. No discovery logic.
- **`resolveFromBase()`** in `provider.go` does exactly what routing needs:
  given a `.beads` path, follow redirects, validate, read `project.name`, and
  return `Paths`. It's currently unexported.
- **`readRedirect()`** in `provider.go` handles redirect file parsing. Also
  unexported.

No new abstractions needed for config or storage. The only change to existing
code is exporting two functions from `config/provider.go`.

### Package: `internal/routing`

New standalone package. No dependency on `cmd` — it takes paths as inputs and
returns resolved paths or opened stores.

#### `routing.go` — Core types and route table

```go
// Route maps a prefix to a rig path relative to town root.
type Route struct {
    Path string `json:"path"`
}

// Router resolves issue IDs to the correct storage location.
type Router struct {
    townRoot    string            // absolute path to town root
    localBeads  string            // absolute path to local .beads dir
    routes      map[string]Route  // prefix → route
}

// New creates a Router by loading routes.jsonl from the given .beads dir.
// Returns nil Router (not an error) if no routes.jsonl exists.
func New(beadsDir string) (*Router, error)

// Resolve returns the Paths for the rig that owns the given issue ID.
// Returns the local paths if no routing is needed (prefix matches local
// or no route found). Uses config.ResolveFromBase() for the target.
func (r *Router) Resolve(issueID string) (config.Paths, bool, error)
//   returns: (paths, isRemote, error)
```

Key behaviors:
- `New()` reads `routes.jsonl` from the provided `.beads` dir. Walks up
  parent directories to find it (the `.beads` that contains `routes.jsonl`
  is the town root's `.beads`). If not found, returns `nil` — caller
  checks for nil and skips routing.
- `Resolve()` extracts prefix, looks up route, constructs
  `<townRoot>/<route.path>/.beads`, calls `config.ResolveFromBase()` to
  follow redirects and get the data dir. Returns whether the result points
  to a remote store.

#### `routing_test.go` — Tests

- Parse valid routes.jsonl (line-delimited JSON)
- Parse empty / missing file
- ExtractPrefix for various ID formats
- Resolve local prefix → returns local paths, isRemote=false
- Resolve remote prefix → returns remote paths, isRemote=true
- Resolve unknown prefix → returns local paths, isRemote=false
- Redirect followed on remote target
- No routes.jsonl → nil Router
- Empty routes.jsonl → nil Router (not an error)

### Changes to `internal/config/provider.go`

Export two existing functions:

```go
// ResolveFromBase resolves Paths from a known .beads directory path.
// Follows redirect files and reads project.name from config.yaml.
// (renamed from resolveFromBase)
func ResolveFromBase(basePath string) (Paths, error)

// ReadRedirect reads the redirect file from a .beads directory.
// Returns "" if no redirect file exists.
// (renamed from readRedirect)
func ReadRedirect(beadsDir string) (string, error)
```

Only `ResolveFromBase` is strictly needed by the routing package. Exporting
`ReadRedirect` is optional but useful for testing. The internal callers in
`provider.go` (`findConfigUpward`, `resolveFromBase`) update to call the
exported names.

### Changes to `internal/cmd/` — Command integration

Commands that do point lookups gain a routing step. Two options:

**Option A: Helper on App** (preferred — keeps routing out of individual commands)

Add to `App`:
```go
type App struct {
    Storage     storage.Storage
    Router      *routing.Router  // nil if no routes.jsonl
    ConfigStore config.Store
    ConfigDir   string
    // ...
}

// StorageFor returns the storage for the given issue ID, routing if needed.
// If the ID belongs to a remote rig, opens a temporary filesystem store.
// Returns the local store if no routing needed.
func (a *App) StorageFor(ctx context.Context, id string) (storage.Storage, error)
```

Commands call `app.StorageFor(ctx, id)` instead of using `app.Storage`
directly. If the router is nil or the prefix is local, it returns
`app.Storage`. Otherwise it opens a new `filesystem.New()` at the resolved
remote path.

**Option B: Middleware/wrapper** — wrap Storage interface with routing logic.
More complex, less transparent. Not recommended.

### Changes to `internal/cmd/root.go` — Wiring

In `AppProvider.init()`, after resolving paths:
```go
router, err := routing.New(paths.ConfigDir)
// err is only for parse errors, nil router means no routes file
```

Pass `router` into `App`.

### Town Root Discovery

The `routing.New(beadsDir)` function needs to find `routes.jsonl`. Strategy:

1. Check `beadsDir/routes.jsonl` — if found, town root = parent of beadsDir
2. Walk up parent directories looking for `.beads/routes.jsonl`
3. Stop at filesystem root
4. If not found, return nil Router

This works because the town root's `.beads/` is the one that contains
`routes.jsonl`. A rig's `.beads/` may be nested deeper (e.g.,
`/repo/crew/misc/.beads/`), but the town root's is at `/repo/.beads/`.

### File Summary

| File | Change |
|------|--------|
| `internal/routing/routing.go` | New: Router struct, New(), Resolve() |
| `internal/routing/routes.go` | New: LoadRoutes(), ExtractPrefix() |
| `internal/routing/routing_test.go` | New: all routing tests |
| `internal/config/provider.go` | Export ResolveFromBase, ReadRedirect |
| `internal/cmd/app.go` | Add Router field, StorageFor() method |
| `internal/cmd/root.go` | Wire Router in AppProvider.init() |
| Point-lookup commands | Call app.StorageFor(id) instead of app.Storage |

### Implementation Order

1. Export functions in `config/provider.go` (tiny, no behavior change)
2. `internal/routing/` package (routes parser, Router, Resolve, tests)
3. Wire into App and root.go
4. Update point-lookup commands to use StorageFor()
