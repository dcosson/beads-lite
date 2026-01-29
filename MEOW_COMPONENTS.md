# MEOW: Molecular Expression of Work

This document describes the MEOW component model as implemented in the beads
reference implementation (`bd`). MEOW provides reusable workflow templates that
produce trackable work items from regular beads primitives.

## Overview

The lifecycle is a chemistry metaphor:

```
Formula  --cook-->  Proto  --pour-->  Mol (persistent)  --squash-->  Digest (permanent summary)
                           --wisp-->  Wisp (ephemeral)  --burn---->  Gone (no trace)
                                                        --squash-->  Digest (promotes to permanent)
```

All of these are built on top of the regular Issue struct with two boolean
flags: `IsTemplate` and `Ephemeral`. There are no separate storage tables or
special entity types.

## Formulas

Formulas are **file-based templates** — JSON files that describe a workflow's
structure, variables, and step dependencies.

### Storage

Formulas live on disk in a search path (highest priority first):

1. `.beads/formulas/*.formula.json` (project-level)
2. `~/.beads/formulas/*.formula.json` (user-level)
3. `$GT_ROOT/.beads/formulas/*.formula.json` (orchestrator-level)

They are never stored in the database.

### Structure

```go
type Formula struct {
    Formula     string              // Unique name (e.g., "mol-feature")
    Description string
    Version     int                 // Schema version (currently 1)
    Type        FormulaType         // "workflow", "expansion", or "aspect"
    Extends     []string            // Parent formulas to inherit from
    Vars        map[string]*VarDef  // Variables with defaults/validation
    Steps       []*Step             // Work items to create
    Template    []*Step             // For expansions (template steps)
    Compose     *ComposeRules       // Bonding rules
    Advice      []*AdviceRule       // Step transformations
    Phase       string              // "liquid" (pour) or "vapor" (wisp)
}

type VarDef struct {
    Description string   // What this variable is for
    Default     string   // Value to use if not provided
    Required    bool     // Must be provided (no default)
    Enum        []string // Allowed values (if non-empty)
    Pattern     string   // Regex pattern the value must match
    Type        string   // Expected type: string (default), int, bool
}
```

### TOML Parsing Note

Formulas support both JSON and TOML formats. In TOML, `Vars` is a
`map[string]*VarDef` — each variable is a nested table under `[vars]`:

```toml
[vars.version]
description = "The semantic version to release (e.g., 0.44.0)"
required = true

[vars.component]
description = "Which component to release"
default = "core"
```

The `[vars]` parent table itself carries no metadata — it exists only as
the TOML namespace for the nested `[vars.<name>]` tables. An empty `[vars]`
line before the first variable is optional (just a readability convention).

The reference implementation uses the BurntSushi TOML library
(`github.com/BurntSushi/toml`) for parsing, which unmarshals nested tables
directly into the `map[string]*VarDef` via struct tags.

### Cooking (`bd cook`)

Cooking compiles a formula into a proto. The process:

1. Load formula, resolve inheritance (`Extends`)
2. Apply transformations: control flow (loops, branches, gates), advice rules,
   inline expansions, composition expansions, aspects
3. Output depends on mode:
   - Default (`--mode=compile`): JSON to stdout with `{{variables}}` intact
   - `--persist`: Creates a proto in the database (template epic + child issues)
   - `--mode=runtime` or `--var` flags: Variables substituted before output

### Variable Handling at Cook Time

In the default compile mode, variables remain as `{{placeholders}}` in the
proto. They are not resolved until pour or wisp time. In runtime mode (or when
`--var` flags are provided), substitution happens during cooking via
`substituteFormulaVars()`.

### Variable Resolution at Pour/Wisp Time

When pouring or wisping, variables from the proto are resolved:

1. **Defaults applied first**: Variables with a `Default` value in their
   `VarDef` are filled in automatically via `applyVariableDefaults()`.
2. **Required variables validated**: Any variable without a default that was
   not provided via `--var` causes a validation error and `exit(1)`:
   ```
   Error: missing required variables: assignee, repo_url
   Provide them with: --var assignee=<value>
   ```
3. **Substitution**: `{{name}}` patterns are replaced in all issue text fields.
   Variables that remain unmatched after substitution are left as-is in the
   text (the regex replacer skips unknown names).

## Protomols (Protos)

A proto is a **template epic** — a regular issue tree marked as read-only.

### Flags

- `IsTemplate=true` (read-only, excluded from `bd list` by default)
- `Ephemeral` is **not explicitly set** — defaults to `false`

The two flags are independent. `IsTemplate` does not imply anything about
`Ephemeral`.

### Storage

Protos are stored in the main SQLite database as regular issues with
`IsTemplate=true` and the "template" label. They can also be shipped as
built-in molecules in `.beads/molecules.jsonl` files (read-only, loaded at
startup).

### Structure

A proto is a subgraph:

```go
type TemplateSubgraph struct {
    Root         *types.Issue            // Root epic (the proto)
    Issues       []*types.Issue          // All descendant issues
    Dependencies []*types.Dependency     // DAG structure (parent-child + blocking)
    IssueMap     map[string]*types.Issue
    VarDefs      map[string]formula.VarDef
    Phase        string                 // "liquid" or "vapor"
}
```

Each formula step becomes a child issue. Dependencies between steps are
encoded as `Dependency` records (`DepParentChild` for hierarchy, `DepBlocks`
for ordering).

## Molecules (Mols)

A molecule is a **persistent clone of a proto** — real issues that are synced
to git and survive across sessions.

### Pouring (`bd mol pour`)

1. Takes a proto (from DB or formula) + variable values
2. Clones the subgraph with `Ephemeral=false`
3. Substitutes `{{variables}}` with provided values
4. Creates real issues in the database with real IDs
5. Warns if the formula specifies `phase: "vapor"` (suggesting wisp instead)

### Multi-step Storage

Each step in a molecule is a **separate issue** (a regular bead). The root is
an epic; steps are child issues linked by:

- `DepParentChild` dependencies (hierarchy)
- `DepBlocks` dependencies (ordering from `depends_on` in the formula)

Steps have normal statuses (`open`, `in_progress`, `closed`, etc.) and are
manipulated with regular `bd` commands.

### Sync

Molecules are persistent. They are exported to `.beads/issues.jsonl` by
`bd sync` and shared via git like any other issue.

## Wisps

A wisp is an **ephemeral clone of a proto** — exists locally but is never
synced.

### Creation (`bd mol wisp`)

1. Takes a proto + variable values (same as pour)
2. Clones the subgraph with `Ephemeral=true`
3. Creates issues in the local SQLite database only

### Ephemeral Behavior

The `Ephemeral` boolean on the Issue struct controls wisp behavior:

- **Sync export**: Wisps are skipped during `bd sync` export. The filter is
  simple: `if issue.Ephemeral { continue }` in `sync_export.go`.
- **No tombstones**: When deleted, ephemeral issues are hard-deleted — no
  soft-delete tombstone is created.
- **GC**: `bd mol wisp gc` can bulk-clean ephemeral issues past a time
  threshold (default 1h).

### Lifecycle

- **Burn** (`bd mol burn`): Deletes the wisp and all children with no trace.
- **Squash** (`bd mol squash`): Creates a permanent digest issue, then deletes
  the wisp children (or promotes them with `--keep-children`).

## Squashed Digests

Squashing a molecule (persistent or ephemeral) produces a **digest** — a
permanent summary issue.

### Structure

```go
type SquashResult struct {
    MoleculeID    string   // Root mol ID
    DigestID      string   // New digest issue ID
    SquashedIDs   []string // IDs that were squashed
    SquashedCount int
    DeletedCount  int
    KeptChildren  bool     // If --keep-children was used
}
```

The digest is a new issue created as a child of the root mol with:
- Auto-generated ID (inherits prefix)
- Title: `"Digest: <root title>"`
- Type: `task` (always `TypeTask`, never epic or other types)
- Status: `closed` (closed immediately on creation)
- `Ephemeral=false` (always permanent)
- No special labels added
- `CloseReason` set to e.g. `"Squashed from 5 wisps"`
- Content: summary of execution (agent-provided via `--summary`, or
  auto-generated from child titles/descriptions)
- Linked to the root molecule via `DepParentChild` dependency

Post-squash, wisp children are deleted unless `--keep-children` is specified,
in which case they are promoted to `Ephemeral=false`.

## The Two Issue Flags

All MEOW concepts are built on regular issues with two boolean fields on the
`Issue` struct (`internal/types/types.go`):

```go
Ephemeral  bool `json:"ephemeral,omitempty"`    // If true, not exported to JSONL
IsTemplate bool `json:"is_template,omitempty"`  // Read-only template molecule
```

Both are nullable in the database (NULL treated as false).

### How `IsTemplate` Affects the System

Templates are **read-only**. This is enforced at the validation layer
(`internal/validation/issue.go`):

| Operation | Allowed? | Guard |
|-----------|----------|-------|
| `bd list` | Excluded by default | Query filter; opt-in with `--include-templates` |
| `bd ready` | Excluded | Filtered by ID pattern (`-mol-`, `-wisp-`) |
| `bd show` | Yes | No special guard |
| `bd update` | **No** | `NotTemplate()` validator: "cannot modify template: templates are read-only; use 'bd mol pour' to create a work item" |
| `bd close` | **No** | `NotTemplate()` in `ForClose()` validator chain |
| `bd delete` | **No** | `NotTemplate()` in `ForDelete()` validator chain |
| Content hash | **Included** | `IsTemplate` is part of the issue's content hash (identity/version) |

### How `Ephemeral` Affects the System

Ephemeral issues are **mutable but invisible to work queues**:

| Operation | Allowed? | Guard |
|-----------|----------|-------|
| `bd list` | Yes (not filtered by default) | No special filtering |
| `bd ready` | **Excluded always** | Hardcoded: `(i.ephemeral = 0 OR i.ephemeral IS NULL)` in `ready.go` |
| `bd blocked` | **Excluded always** | Same hardcoded filter |
| `bd show` | Yes | No special guard |
| `bd update` | Yes | No guard; can also toggle ephemeral via `--ephemeral` / `--persistent` flags |
| `bd close` | Yes | No special guard |
| `bd delete` | Yes | Hard-delete (no tombstone) |
| `bd sync` | **Skipped on export** | `if issue.Ephemeral { continue }` |
| `bd cleanup` | Targetable | `--wisp-only` flag for bulk cleanup |
| Content hash | **Not included** | `Ephemeral` is metadata, not part of content identity |

### Design Note

The `IsTemplate` flag participates in content hashing (changing it changes the
issue's version/identity). The `Ephemeral` flag does not — it's treated as
operational metadata rather than content.

## Other Ephemeral Concepts

Only wisps use `Ephemeral=true`. Related but distinct concepts:

- **Tombstones**: Soft-deleted issues with TTL, eventually hard-deleted. Not
  ephemeral — they are synced and can be resurrected.
- **Pinned**: Issues with `Pinned=true`. Persistent context markers, not work
  items. Protected from accidental close (requires `--force`).

## Architectural Summary

MEOW is a thin command-layer abstraction over regular beads. The storage
additions are minimal:

- Two boolean fields on Issue (`Ephemeral`, `IsTemplate`)
- A `child_counters` table for hierarchical ID generation
- A `mol_type` column (migration 031) for molecule flavor metadata
  ("swarm", "patrol", "work")
- Formula files on disk (not in the database at all)

Everything else — step tracking, dependencies, status management, hierarchy —
uses existing beads primitives. The `gt mol` and `bd` commands provide the
workflow semantics; the storage layer just sees issues with flags.
