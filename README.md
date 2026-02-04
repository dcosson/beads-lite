<p align="center">
  <img src="images/beads-lite-hero-square.jpg" alt="Beads Lite" width="400">
</p>

# Beads Lite

Beads Lite is a lightweight, drop-in replacement for [beads](https://github.com/steveyegge/beads), a CLI task tracker for AI Agents. It's designed to work with [Gas Town](https://steve-yegge.medium.com/welcome-to-gas-town-4f25ee16dd04) and should work with any other scripts or tools you already have built around beads. It supports the Molecular Expression of Work commands for cooking, pouring, and burning formulas, molecules, and wisps.

Beads is a great tool but it can be slow and buggy, particularly it seems in a complicated routing setup like gastown. Beads Lite stores issues as plain JSON files, one issue to a file, in a `.beads/` directory, making them easy to review, diff, and track alongside your code (or to store untracked in a separate directory if you prefer). It's ~10x faster than beads with no split source of truth between sqlite & jsonl files, no background daemon needed, and no global locking. It makes Gas Town noticeably snappier to run.

There are still some gaps listed below, and there may be others in particular flags or semantics, but most of the functionality is already covered. The backend is pluggable behind a simple CRUD interface so it would be simple to add integrations for sqlite, dolt, redis, or other datastores.

## Install

Build from source:

```bash
make build                        # produces ./bd
ln -s "$(pwd)/bd" ~/.local/bin/bd # creates symlink. Make sure ~/.local/bin is on your path.
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

| Feature                                        | beads | beads-lite | Notes |
| ---------------------------------------------- | :---: | :--------: | ----- |
| `bd init`                                      |  ✅   |     ✅     |       |
| `BEADS_DIR` env var                            |  ✅   |     ✅     |       |
| Config path resolution (walk up CWD, git root) |  ✅   |     ✅     |       |
| `.beads/redirect` files                        |  ✅   |     ✅     |       |
| `bd config set/get/list/unset`                 |  ✅   |     ✅     |       |
| `bd config validate`                           |  ✅   |     ✅     |       |
| Custom types (`types.custom`)                  |  ✅   |     ✅     |       |
| Custom statuses (`status.custom`)              |  ✅   |     ✅     |       |

### Issue Tracking

| Feature                                                     | beads | beads-lite | Notes                                                                                   |
| ----------------------------------------------------------- | :---: | :--------: | --------------------------------------------------------------------------------------- |
| Create / show / update / delete                             |  ✅   |     ✅     |                                                                                         |
| List with filters (status, priority, type, label, assignee) |  ✅   |     ✅     |                                                                                         |
| Issue types (task, bug, feature, epic, chore, molecule)     |  ✅   |     ✅     |                                                                                         |
| Molecule types (`mol_type`: swarm, patrol, work)            |  ✅   |     ✅     | Filterable via `--mol-type`                                                             |
| Priorities (P0-P4)                                          |  ✅   |     ✅     |                                                                                         |
| Statuses (open, in_progress, blocked, deferred, closed)     |  ✅   |     ✅     |                                                                                         |
| `hooked` status                                             |  ✅   |     ✅     | For GUPP protocol (agent hook attachment)                                               |
| Close / reopen                                              |  ✅   |     ✅     |                                                                                         |
| Assignees                                                   |  ✅   |     ✅     |                                                                                         |
| Labels                                                      |  ✅   |     ✅     |                                                                                         |
| Comments (`bd comments`)                                    |  ✅   |     ✅     |                                                                                         |
| Dependencies (10 typed dep kinds)                           |  ✅   |     ✅     |                                                                                         |
| Parent-child hierarchy (dot notation IDs)                   |  ✅   |     ✅     |                                                                                         |
| Search                                                      |  ✅   |     ✅     |                                                                                         |
| Doctor (consistency checks)                                 |  ✅   |     ✅     |                                                                                         |
| Stats                                                       |  ✅   |     ✅     |                                                                                         |
| Compact (prune old closed issues)                           |  ✅   |     ✅     |                                                                                         |
| Ready / blocked views                                       |  ✅   |     ✅     |                                                                                         |
| Batch close with `--continue`/`--suggest-next`              |  ✅   |     ✅     |                                                                                         |
| `bd edit` (open in `$EDITOR`)                               |  ✅   |     ✅     |                                                                                         |
| `bd label` management                                       |  ✅   |     ✅     |                                                                                         |
| `bd rename` (rename issue ID)                               |  ✅   |     ⬜     |                                                                                         |
| `bd move` / `bd refile` (move between rigs)                 |  ✅   |     ⬜     |                                                                                         |
| `bd duplicate` / `bd duplicates`                            |  ✅   |     ⬜     |                                                                                         |
| `bd stale` (not updated recently)                           |  ✅   |     ⬜     |                                                                                         |
| `bd lint` (check template sections)                         |  ✅   |     ⬜     |                                                                                         |
| `bd graph` (dependency graph)                               |  ✅   |     🟡     | `internal/graph` pkg exists, no CLI command                                             |
| `bd activity` (real-time mutation feed)                     |  ✅   |     ⬜     | Accepted as no-op; supports `--follow`, `--town`, `--json` flags but produces no output |
| Export / import (JSONL)                                     |  ✅   |     ⬜     |                                                                                         |

> 🟡 **graph**: The `internal/graph` package implements the dependency graph logic, but no `bd graph` CLI command exposes it yet.

### Molecular Expression of Work (MEOW)

| Feature                                      | beads | beads-lite | Notes            |
| -------------------------------------------- | :---: | :--------: | ---------------- |
| Formulas (template definitions)              |  ✅   |     ✅     | `internal/meow/` |
| `bd formula list` / `show` / `convert`       |  ✅   |     ✅     |                  |
| `bd mol pour` (instantiate formula)          |  ✅   |     ✅     |                  |
| `bd mol wisp` (ephemeral instance)           |  ✅   |     ✅     |                  |
| `bd mol burn` (cascade delete)               |  ✅   |     ✅     |                  |
| `bd mol squash` (compress to digest)         |  ✅   |     ✅     |                  |
| `bd mol current` / `progress` / `stale`      |  ✅   |     ✅     |                  |
| `bd mol gc` (clean old wisps)                |  ✅   |     ✅     |                  |
| `bd mol bond` (combine protos/mols)          |  ✅   |     ⬜     |                  |
| `bd mol distill` (extract formula from epic) |  ✅   |     ⬜     |                  |
| `bd mol seed --patrol`                       |  ✅   |     ✅     |                  |
| `bd cook` (compile formula to proto)         |  ✅   |     ✅     |                  |

### Gas Town (Multi-Agent Coordination)

| Feature                                          | beads | beads-lite | Notes                                        |
| ------------------------------------------------ | :---: | :--------: | -------------------------------------------- |
| `bd agent` (state, heartbeat)                    |  ✅   |     ✅     |                                              |
| `bd slot` (set, clear, show)                     |  ✅   |     ✅     | Built on KV storage                          |
| `bd gate` (async coordination)                   |  ✅   |     ✅     | show, list, wait, add-waiter, resolve, check |
| `bd swarm` (validate, create, status, list)      |  ✅   |     ✅     |                                              |
| Seed patrol (formula seeding)                    |  ✅   |     ✅     |                                              |
| `bd merge-slot` (serialized conflict resolution) |  ✅   |     ✅     |                                              |
| `bd audit` (append-only activity log)            |  ✅   |     ⬜     |                                              |
| `bd set-state` / `bd state`                      |  ✅   |     ⬜     |                                              |
| `bd mail`                                        |  ✅   |     ⬜     | Delegates to `gt mail`                       |

### Routing

| Feature                                                | beads | beads-lite | Notes          |
| ------------------------------------------------------ | :---: | :--------: | -------------- |
| Issue prefix routing (`routes.jsonl`)                  |  ✅   |     ✅     | See ROUTING.md |
| Town root discovery                                    |  ✅   |     ✅     |                |
| Contributor routing (maintainer/contributor workflows) |  ✅   |     ⬜     |                |

### Compatibility Commands

| Feature                    | beads | beads-lite | Notes                                    |
| -------------------------- | :---: | :--------: | ---------------------------------------- |
| `bd version`               |  ✅   |     ✅     | Returns 0.43.0 (meets gastown minimum)   |
| `bd sync`                  |  ✅   |     ✅     | No-op (filesystem storage needs no sync) |
| `bd migrate`               |  ✅   |     ✅     | No-op (no DB to migrate)                 |
| `bd prime`                 |  ✅   |     ✅     | No-op                                    |
| `bd import`                |  ✅   |     ✅     | No-op (accepts flags for compatibility)  |
| `init --prefix`            |  ✅   |     ✅     |                                          |
| `-q`/`--quiet` global flag |  ✅   |     ✅     |                                          |

### Sync & Integrations

| Feature                             | beads | beads-lite | Notes                               |
| ----------------------------------- | :---: | :--------: | ----------------------------------- |
| JSONL sync (`bd sync`)              |  ✅   |     ✅     | Accepted as no-op for compatibility |
| Daemon (background sync)            |  ✅   |     ✅     | Not needed (single source of truth) |
| Dolt DB backend                     |  ✅   |     ⬜     |                                     |
| Jira / Linear / GitHub integrations |  ✅   |     ⬜     |                                     |
| Federation (peer-to-peer sync)      |  ✅   |     ⬜     |                                     |
| Git merge driver                    |  ✅   |     ⬜     |                                     |

**Legend:** ✅ implemented | 🟡 partial | ⬜ not yet

## Testing

```bash
make test          # run all tests (unit + e2e)
make test-unit     # unit tests only
make test-e2e      # e2e tests against local ./bd build
make update-e2e    # regenerate expected e2e outputs from reference bd
make bench-e2e     # benchmark beads-lite (also included in test-e2e)
make bench-comparison-e2e  # benchmark against reference bd (requires bd in PATH)
```

### Benchmark

`make test-e2e` includes a happy-path benchmark that exercises create, list, show,
update, and close across 20 issues. `make bench-comparison-e2e` runs the same
workflow against the reference `bd` binary for a side-by-side comparison.

Sample `make bench-comparison-e2e` output:

```
Phase                    beads-lite bd (reference)       diff
──────────────────────────────────────────────────────────────
create (20)                   0.21s          2.34s     -91.1%
list (20x)                    0.11s          1.56s     -92.9%
show (20x)                    0.10s          1.71s     -94.2%
update (20)                   0.20s          1.62s     -87.7%
close (20)                    0.20s          1.58s     -87.2%
final list (20x)              0.10s          1.55s     -93.6%
──────────────────────────────────────────────────────────────
TOTAL                         0.92s         10.35s     -91.1%
```
