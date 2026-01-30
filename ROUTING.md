# Routing: Cross-Prefix Issue Resolution

Routing allows `bd` commands to find issues that live in a different rig's
database by matching the issue ID prefix to a route table.

## The Problem

A Gas Town deployment has multiple rigs, each with its own `.beads` directory
(or a redirect to a shared one). Each rig uses a different issue prefix
(e.g., `hq-`, `bl-`, `gt-`). When you run `bd show bl-1jzo` from the `hq`
rig, the local database doesn't have that issue — it lives in the `bl` rig's
database.

**Which commands route:**
- Point lookups: `bd show`, `bd agent state`, `bd agent heartbeat`,
  `bd slot set/clear` — these accept a single ID and can route to find it.
- Bulk queries: `bd list`, `bd ready`, `bd blocked` — these query the local
  database only. No routing.

## Core Concepts

### Prefix

The prefix is everything before the first hyphen in an issue ID:
- `bl-1jzo` → prefix `bl`
- `hq-abc123` → prefix `hq`
- `gt-emma` → prefix `gt`

### Routes File

Routes live in `.beads/routes.jsonl` at the town root. One JSON object per
line mapping prefix to a path:

```jsonl
{"prefix": "hq-", "path": "."}
{"prefix": "bl-", "path": "crew/misc"}
{"prefix": "gt-", "path": "crew/max"}
```

- `prefix`: The issue ID prefix including the trailing hyphen.
- `path`: Relative path from the town root to the rig's project directory
  (the parent of `.beads`). `"."` means the town root itself.

### Town Root

The town root is the top-level directory containing all rigs. The reference
implementation discovers it by walking up from CWD looking for
`mayor/town.json`. For beads-lite, this can be simpler — the town root is
wherever `routes.jsonl` is found, or it can be configured.

### Redirect Files

A `.beads/redirect` file contains a single line with a path (relative or
absolute) pointing to a different `.beads` directory. This lets multiple
project directories share one database.

```
# .beads/redirect
../../mayor/rig/.beads
```

Rules:
- Relative paths resolve from the project root (parent of `.beads`)
- Only one level of redirection — no chains
- If the target doesn't exist, fall back to the original `.beads`

## Resolution Flow

When a command receives an issue ID:

```
1. Extract prefix from ID (everything up to and including first hyphen)
2. Check if prefix matches a route in routes.jsonl
   - If no match → use local store (no routing needed)
   - If match but target is the current beads dir → use local store
3. Resolve the route's path to a .beads directory:
   - path "." → <town-root>/.beads
   - path "crew/misc" → <town-root>/crew/misc/.beads
4. Follow redirect if the target .beads has a redirect file
5. Open the target store (read-only is fine for show)
6. Resolve the partial ID and fetch the issue from the target store
7. Close the target store when done
```

## What beads-lite Needs

### Minimum Viable Routing

1. **`routes.jsonl` parser** — Read JSONL file, build prefix→path map
2. **`needsRouting(id)` function** — Extract prefix, check if it maps to a
   different beads directory than the current one
3. **`resolveRoute(id)` function** — Given an ID, return the path to the
   correct `.beads` directory
4. **Redirect file support** — Read `.beads/redirect`, resolve path, follow
   one level
5. **Routed store opener** — Open a temporary read-only store at the resolved
   path, use it for the lookup, close it after

### Integration Points

Only point-lookup commands need routing:
- `bd show <id>` — resolve + get issue
- `bd agent state <id> <state>` — resolve + update agent (read-write)
- `bd agent heartbeat <id>` — resolve + update timestamp (read-write)
- `bd slot set/clear <agent> <slot> [bead]` — resolve agent + resolve bead

Bulk commands (`bd list`, `bd ready`, etc.) stay local-only.

### Filesystem Storage Considerations

The reference implementation opens a second SQLite database for routed
lookups. With beads-lite's filesystem storage, a routed lookup means reading
files from a different `.beads` directory. This is simpler — you just need to
point the filesystem storage at a different root path. No connection
management needed.

### What You Can Skip

- **Town root auto-detection** via `mayor/town.json` — configure it or find
  it via `routes.jsonl` location
- **User role detection** (maintainer vs contributor) — only matters for
  sync write permissions
- **Multi-repo hydration** (`repos:` config with `additional:` paths) — this
  is a separate feature for importing issues from multiple repos into one
  database. Useful but orthogonal to prefix routing.
- **mtime caching** for multi-repo — optimization, not needed initially
