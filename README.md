<p align="center">
  <img src="images/beads-lite-hero-square.jpg" alt="Beads Lite" width="400">
</p>

# Beads Lite

A small, fast, lightweight drop-in replacement for [beads](https://github.com/anthropics/beads). Beads Lite stores issues as plain JSON files in a `.beads/` directory, making them easy to review, diff, and track alongside your code with no database required.

## Install

```bash
go install beads-lite/cmd@latest
```

Or build from source:

```bash
make build    # produces ./bd
```

## Usage

```bash
bd init                              # initialize in current directory
bd create "Fix login bug"            # create an issue
bd list                              # list open issues
bd show bd-a1b2                      # show issue details
bd update bd-a1b2 --status in-progress
bd close bd-a1b2                     # close an issue
```

## Feature Parity with Beads

Beads Lite aims to be a drop-in replacement for the core `bd` command interface.
This table tracks implementation status across major feature areas.

### Config & Setup

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| `bd init` | ✅ | ✅ | |
| `BEADS_DIR` env var | ✅ | ✅ | |
| Config path resolution (walk up CWD, git root) | ✅ | ✅ | |
| `.beads/redirect` files | ✅ | ✅ | |
| `bd config set/get/list/unset` | ✅ | ✅ | |
| `bd config validate` | ✅ | ✅ | |
| Custom status states (`status.custom`) | ✅ | ✅ | |
| Custom types/priorities | ✅ | ⬜ | |

### Issue Tracking

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| Create / show / update / delete | ✅ | ✅ | |
| List with filters (status, priority, type, label, assignee) | ✅ | ✅ | |
| Issue types (task, bug, feature, epic, chore) | ✅ | ✅ | |
| Priorities (P0-P4) | ✅ | ✅ | |
| Statuses (open, in_progress, blocked, deferred, closed) | ✅ | ✅ | |
| `hooked` status | ✅ | ⬜ | For GUPP protocol (agent hook attachment) |
| Close / reopen | ✅ | ✅ | |
| Assignees | ✅ | ✅ | |
| Labels | ✅ | ✅ | |
| Comments (`bd comments`) | ✅ | ✅ | |
| Dependencies (10 typed dep kinds) | ✅ | ✅ | |
| Parent-child hierarchy (dot notation IDs) | ✅ | ✅ | |
| Search | ✅ | ✅ | |
| Doctor (consistency checks) | ✅ | ✅ | |
| Stats | ✅ | ✅ | |
| Compact (prune old closed issues) | ✅ | ✅ | |
| Ready / blocked views | ✅ | ✅ | |
| Batch close with `--continue`/`--suggest-next` | ✅ | ✅ | |
| `bd edit` (open in `$EDITOR`) | ✅ | ⬜ | |
| `bd label` management | ✅ | 🟡 | Labels set via `bd update --label` |
| `bd rename` (rename issue ID) | ✅ | ⬜ | |
| `bd move` / `bd refile` (move between rigs) | ✅ | ⬜ | |
| `bd duplicate` / `bd duplicates` | ✅ | ⬜ | |
| `bd stale` (not updated recently) | ✅ | ⬜ | |
| `bd lint` (check template sections) | ✅ | ⬜ | |
| `bd graph` (dependency graph) | ✅ | 🟡 | `internal/graph` pkg exists, no CLI command |
| Export / import (JSONL) | ✅ | ⬜ | |

> 🟡 **label**: Labels can be set via `bd update --label`, but there's no dedicated `bd label` management command.
> 🟡 **graph**: The `internal/graph` package implements the dependency graph logic, but no `bd graph` CLI command exposes it yet.
> 🟡 **gate**: show, list, wait, add-waiter, resolve are implemented. `gate check` (auto-evaluate conditions) is not yet built.

### Molecular Expression of Work (MEOW)

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| Formulas (template definitions) | ✅ | ✅ | `internal/meow/` |
| `bd formula list` / `show` / `convert` | ✅ | ✅ | |
| `bd mol pour` (instantiate formula) | ✅ | ✅ | |
| `bd mol wisp` (ephemeral instance) | ✅ | ✅ | |
| `bd mol burn` (cascade delete) | ✅ | ✅ | |
| `bd mol squash` (compress to digest) | ✅ | ✅ | |
| `bd mol current` / `progress` / `stale` | ✅ | ✅ | |
| `bd mol gc` (clean old wisps) | ✅ | ✅ | |
| `bd mol bond` (combine protos/mols) | ✅ | ⬜ | |
| `bd mol distill` (extract formula from epic) | ✅ | ⬜ | |
| `bd mol seed --patrol` | ✅ | ⬜ | Verify patrol formulas accessible |
| `bd cook` (compile formula to proto) | ✅ | ✅ | |

### Gas Town (Multi-Agent Coordination)

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| `bd agent` (state, heartbeat) | ✅ | ⬜ | |
| `bd slot` (set, clear, list) | ✅ | ⬜ | Needs KV storage (bl-r2nl) |
| `bd gate` (async coordination) | ✅ | 🟡 | show, list, wait, add-waiter, resolve done; `gate check` missing |
| `bd swarm` (structured epics) | ✅ | ⬜ | |
| Seed patrol (formula seeding) | ✅ | ⬜ | |
| `bd merge-slot` (serialized conflict resolution) | ✅ | ⬜ | |
| `bd audit` (append-only activity log) | ✅ | ⬜ | |
| `bd set-state` / `bd state` | ✅ | ⬜ | |
| `bd mail` | ✅ | ⬜ | Delegates to `gt mail` |

### Routing

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| Issue prefix routing (`routes.json`) | ✅ | ✅ | See ROUTING.md |
| Town root discovery | ✅ | ⬜ | |
| Contributor routing (maintainer/contributor workflows) | ✅ | ⬜ | |

### Compatibility Commands

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| `bd version` | ✅ | ✅ | Returns 0.43.0 (meets gastown minimum) |
| `bd sync` | ✅ | ✅ | No-op (filesystem storage needs no sync) |
| `bd migrate` | ✅ | ✅ | No-op (no DB to migrate) |
| `bd prime` | ✅ | ✅ | No-op |
| `bd import` | ✅ | ✅ | No-op (accepts flags for compatibility) |
| `init --prefix` | ✅ | ⬜ | Has `--project` but not `--prefix` |
| `-q`/`--quiet` global flag | ✅ | ⬜ | |

### Sync & Integrations

| Feature | beads | beads-lite | Notes |
|---------|:-----:|:----------:|-------|
| JSONL sync (`bd sync`) | ✅ | ⬜ | Accepted as no-op for compatibility |
| Daemon (background sync) | ✅ | ⬜ | Not needed (no DB) |
| Dolt backend (branching, history, diff) | ✅ | ⬜ | Out of scope |
| Jira / Linear / GitHub integrations | ✅ | ⬜ | |
| Federation (peer-to-peer sync) | ✅ | ⬜ | |
| Git merge driver | ✅ | ⬜ | |

**Legend:** ✅ implemented | 🟡 partial | ⬜ not yet

## Testing

```bash
make test          # run all tests (unit + e2e)
make test-unit     # unit tests only
make test-e2e      # e2e tests against local ./bd build
make e2e-update    # regenerate expected e2e outputs from reference bd
```
