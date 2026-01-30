# MEOW: Molecular Expression of Work

This document describes the MEOW component model for beads-lite, based on the
beads reference implementation. MEOW provides reusable workflow templates that
produce trackable work items from regular beads primitives.

## Overview

The simplified lifecycle:

```
Formula  --pour-->  Mol (persistent)   --close-->  Done (stays in history)
                                       --burn--->  Gone (tombstones for sync)
         --wisp-->  Wisp (ephemeral)   --squash->  Digest (permanent summary)
                                       --burn--->  Gone (no trace)
```

The normal path for a mol is to close it — the molecule and all its children
remain in the database and sync history. Burn is for cleanup (abandoned or
failed workflows) and never creates a digest.

Squash is a **wisp-only** operation — it collects ephemeral children and creates
a permanent digest. Burn on wisps hard-deletes with no trace.

A molecule is just an **epic with children and dependencies** derived from a
formula. The `pour` and `wisp` commands read a formula file, resolve variables,
and create real issues directly. The only structural difference between a mol
and a wisp is the `Ephemeral` flag on the Issue struct.

## Formulas

Formulas are **file-based templates** — JSON or TOML files that describe a
workflow's structure, variables, and step dependencies.

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

### Formula Resolution

When a formula is loaded for pour or wisp, the following transformations are
applied before creating issues:

1. Load formula, resolve inheritance (`Extends`)
2. Apply control flow operators (loops, branches, gates)
3. Apply advice rules
4. Apply inline step expansions
5. Apply composition expansions
6. Apply aspects

The result is an in-memory template subgraph with `{{variable}}` placeholders
still intact. Variable substitution happens next, during issue creation.

This in-memory template is known as a `proto` in the MEOW parlance, and `cook`
is the action that turns a formula into a proto, but these are really just
internal implementation details that probably don't need such grand of names.
A proto doesn't persiste to storage or get exposed to the user (there is a
bd cook command that will let you view the proto, but you don't really need
to). For all intents and purposes you just pour or wisp a formula directly to a
molecule or wisp. See the appendix below for more details.

### Variable Resolution

When pouring or wisping, variables are resolved:

1. **Defaults applied first**: Variables with a `Default` value in their
   `VarDef` are filled in automatically.
2. **Required variables validated**: Any variable without a default that was
   not provided via `--var` causes a validation error and `exit(1)`:
   ```
   Error: missing required variables: assignee, repo_url
   Provide them with: --var assignee=<value>
   ```
3. **Substitution**: `{{name}}` patterns are replaced in all issue text fields.
   Variables that remain unmatched after substitution are left as-is in the
   text (the regex replacer skips unknown names).

## Molecules (Mols)

A molecule is a **persistent set of issues** created from a formula.

### Pouring (`bd mol pour`)

1. Takes a formula name + variable values
2. Loads and resolves the formula (see Formula Resolution above)
3. Substitutes `{{variables}}` with provided values
4. Creates real issues in the database with `Ephemeral=false`
5. Warns if the formula specifies `phase: "vapor"` (suggesting wisp instead)

### Structure

A molecule is an **epic** (the root issue) with **child issues** for each
formula step. The structure maps directly to the formula:

- Each formula step becomes a child issue
- `DepParentChild` dependencies link children to the root epic
- `DepBlocks` dependencies encode ordering from `depends_on` in the formula

Steps have normal statuses (`open`, `in_progress`, `closed`, etc.) and are
manipulated with regular `bd` commands. There is nothing special about a
molecule's issues — they are ordinary beads.

### Sync

Molecules are persistent. They are exported to `.beads/issues.jsonl` by
`bd sync` and shared via git like any other issue.

### Lifecycle

- **Burn** (`bd mol burn`): Cascade-deletes the molecule and all children.
  Because mols are persistent, tombstones are created so the deletion syncs
  to remotes. No digest is created.
- **Squash** does **not** work on persistent molecules — it only processes
  ephemeral children. If you need a summary before burning a mol, create a
  digest manually first.

## Wisps

A wisp is an **ephemeral set of issues** created from a formula — the same
structure as a mol, but with `Ephemeral=true` on every issue.

### Creation (`bd mol wisp`)

1. Takes a formula name + variable values (same as pour)
2. Loads and resolves the formula
3. Substitutes variables
4. Creates issues in the local database with `Ephemeral=true`

### Ephemeral Behavior

The `Ephemeral` boolean on the Issue struct controls wisp behavior:

- **Sync export**: Wisps are skipped during `bd sync` export. The filter is
  simple: `if issue.Ephemeral { continue }`.
- **No tombstones**: When deleted, ephemeral issues are hard-deleted — no
  soft-delete tombstone is created.
- **GC**: `bd mol wisp gc` can bulk-clean ephemeral issues past a time
  threshold (default 1h).

### Lifecycle

- **Burn** (`bd mol burn`): Deletes the wisp and all children with no trace.
- **Squash** (`bd mol squash`): Creates a permanent digest issue, then deletes
  the wisp children (or promotes them with `--keep-children`).

## Squash and Burn

Squash is wisp-only. Burn works on both mols and wisps.

### Squash (Wisps Only)

Squashing produces a **digest** — a permanent summary of the work. It only
operates on **ephemeral children** (`Ephemeral=true`). Running squash on a
persistent molecule with no ephemeral children will print
`"No ephemeral children found"` and exit.

The digest is a new issue created as a child of the root wisp with:

- Auto-generated ID (inherits prefix)
- Title: `"Digest: <root title>"`
- Type: `task` (always `TypeTask`, never epic or other types)
- Status: `closed` (closed immediately on creation)
- `Ephemeral=false` (always permanent, even when squashing a wisp)
- No special labels added
- `CloseReason` set to e.g. `"Squashed from 5 wisps"`
- Content: summary of execution (agent-provided via `--summary`, or
  auto-generated from child titles/descriptions)
- Linked to the root molecule via `DepParentChild` dependency

Post-squash, children are deleted unless `--keep-children` is specified,
in which case they are promoted to `Ephemeral=false`.

### Burn

Burning deletes the molecule and all its children **without creating a digest**.
The behavior differs by phase:

- **Wisp (ephemeral)**: Hard-delete, no tombstones, no trace.
- **Mol (persistent)**: Cascade-delete with tombstones (so the deletion syncs
  to remotes via `bd sync`).

Note: The burn command's help text suggests using `bd mol squash` to preserve a
summary before burning, but squash only works on ephemeral children. For
persistent mols, this is a gap in the reference implementation — there is no
built-in "squash then burn" for persistent molecules.

## The `Ephemeral` Issue Flag

The only MEOW-specific addition to the Issue struct:

```go
Ephemeral bool `json:"ephemeral,omitempty"` // If true, not exported to JSONL
```

Nullable in the database (NULL treated as false).

### How `Ephemeral` Affects the System

Ephemeral issues are **mutable but invisible to work queues**:

| Operation    | Allowed?                      | Guard                                               |
| ------------ | ----------------------------- | --------------------------------------------------- |
| `bd list`    | Yes (not filtered by default) | No special filtering                                |
| `bd ready`   | **Excluded always**           | Hardcoded filter in query layer                     |
| `bd blocked` | **Excluded always**           | Same hardcoded filter                               |
| `bd show`    | Yes                           | No special guard                                    |
| `bd update`  | Yes                           | Can toggle via `--ephemeral` / `--persistent` flags |
| `bd close`   | Yes                           | No special guard                                    |
| `bd delete`  | Yes                           | Hard-delete (no tombstone)                          |
| `bd sync`    | **Skipped on export**         | Filtered during export                              |
| `bd cleanup` | Targetable                    | `--wisp-only` flag for bulk cleanup                 |
| Content hash | **Not included**              | Ephemeral is metadata, not content identity         |

## Workflow Progression

Workflow commands are **molecule-specific**. They walk the parent-child
dependency chain to find the root molecule and determine step ordering.
Regular tasks and epics that aren't part of a molecule do not have these
workflow semantics — for those, use `bd ready` and `bd update --claim`.

Mol and wisp workflows are identical. The `Ephemeral` flag only affects
storage/sync, not progression.

### Assignment Model

Assignment happens at the **root epic level**, not on individual steps. To
start working on a molecule, assign yourself to the root epic and set it to
`in_progress`. Individual steps do not need to be assigned.

`bd mol current` finds your molecule by querying for in_progress issues
assigned to you. When it finds the root epic (an epic with no parent), it
loads the full molecule and displays step-by-step progress.

`bd close --continue` advances the next step to `in_progress` but does not
set its assignee — the step's status alone drives the `[current]` marker in
the display. The root epic's assignment is what ties the molecule to you.

### Viewing Progress

**`bd mol current [molecule-id]`** — Show current position in a molecule.

Displays all steps with status markers: `[done]`, `[current]`, `[ready]`,
`[blocked]`, `[pending]`. Shows a progress summary (X/Y steps complete).

If no molecule-id is given, infers from in_progress issues assigned to the
current actor. Use `--for <agent>` to view another agent's molecules.

Additional flags:

- `--limit <n>` — Show first N steps
- `--range <start-end>` — Show specific step range (e.g., `1-50`)
- `--json` — JSON output

For molecules with >100 steps and no explicit flags, shows a summary instead
of the full step list.

**`bd mol progress [molecule-id]`** — Efficient progress summary.

Shows completion count, percentage, rate (steps/hour), and ETA. Uses indexed
queries rather than loading the full subgraph, so it works on very large
molecules.

**`bd mol stale <molecule-id>`** — Find blocking or stale issues in a molecule.

### Discovering Ready Work

**`bd ready`** — Show all unblocked work across the database. By default this
excludes molecule steps to avoid overwhelming output.

**`bd ready --mol <molecule-id>`** — Show unblocked steps within a specific
molecule. Includes parallel group info (which steps can run concurrently).

Additional flags:

- `--assignee <agent>` — Filter by assignee
- `--unassigned` — Show only unassigned issues
- `--limit <n>` — Max issues to show
- `--label <l>` — Filter by labels

### Claiming Work

**`bd update <id> --claim`** — Atomically set `assignee=current_actor` and
`status=in_progress`. Fails if the issue already has an assignee (prevents
race conditions in multi-agent environments). Works on any issue, not just
molecule steps.

**`bd update <id> --status in_progress`** — Set status without setting
assignee.

### Advancing Through Steps

There is no explicit `bd mol next` command. Advancement happens through close:

**`bd close <id> --continue`** — Close the current step and advance the
next ready step in the molecule to `in_progress`.

**`bd close <id> --continue --no-auto`** — Close the step and show the next
ready step, but don't auto-claim it.

**`bd close <id> --suggest-next`** — Show newly unblocked issues after close
(no auto-claim). Works independently of `--continue`.

The `--continue` flag only works on issues that are part of a molecule. It
calls `findParentMolecule()` internally — if the issue has no parent molecule,
the flag is a no-op.

### Workflow vs Non-Molecule Commands

| Capability            | Molecule steps        | Regular issues      |
| --------------------- | --------------------- | ------------------- |
| View current position | `bd mol current`      | N/A                 |
| Progress tracking     | `bd mol progress`     | N/A                 |
| Auto-advance on close | `bd close --continue` | N/A                 |
| Find ready work       | `bd ready --mol <id>` | `bd ready`          |
| Claim work            | `bd update --claim`   | `bd update --claim` |
| Global status         | `bd status`           | `bd status`         |

## Other Ephemeral-Adjacent Concepts

Only wisps use `Ephemeral=true`. Related but distinct:

- **Tombstones**: Soft-deleted issues with TTL, eventually hard-deleted. Not
  ephemeral — they are synced and can be resurrected.
- **Pinned**: Issues with `Pinned=true`. Persistent context markers, not work
  items. Protected from accidental close (requires `--force`).

## Appendix: Reference Implementation Proto Features (Not in beads-lite)

The reference `bd` implementation has additional proto-related features that
beads-lite does not support. These are documented here for context.

### Persisted Protos and `IsTemplate`

The reference implementation supports persisting protos to the database via
`bd cook --persist`. This writes the formula's issue tree into the database
with an `IsTemplate=true` flag on each issue, marking them as read-only
templates. The `IsTemplate` flag:

- Excludes templates from `bd list` by default (opt-in with `--include-templates`)
- Blocks `bd update`, `bd close`, and `bd delete` via validation guards
- Participates in content hashing (part of issue identity)

This is a legacy pattern. The modern `bd pour` and `bd mol wisp` commands
cook formulas inline — they build an in-memory template subgraph, substitute
variables, and create real issues directly without ever persisting a proto.
The database fallback (looking up a proto by ID) only triggers if formula
resolution fails.

**beads-lite does not implement `IsTemplate` or persisted protos.** Formulas
on disk are the source of truth; there is no need to store an intermediate
template in the database.

### Cook Modes: Compile vs Runtime

The reference `bd cook` command has two output modes:

- **Compile mode** (default): Outputs the resolved formula as JSON with
  `{{variable}}` placeholders intact. Useful for inspecting template structure.
- **Runtime mode** (`--mode=runtime` or `--var` flags): Substitutes all
  variables before outputting. Useful for previewing what pour/wisp would
  create.

Both modes output to stdout and do not create issues. The `--persist` flag
always stores in compile mode regardless of other flags — `--var` values
passed alongside `--persist` are displayed in the CLI output but not baked
into the stored proto.

**beads-lite does not need separate cook modes.** If a `cook` command is
provided, it serves only as a preview/dry-run tool. The operational pipeline
is formula → pour/wisp → issues.
