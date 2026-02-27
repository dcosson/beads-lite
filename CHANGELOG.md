# Changelog

## v0.49.2

### Features
- **`bd graph` command** — visualize the dependency graph across epics with topological wave ordering, cascade-aware blocking, and both tree and JSON output formats
- **Cascade parent blocking** — blockers on a parent epic now cascade down to child tasks in `bd ready`, `bd blocked`, `bd stats`, and `bd show` (config: `graph.cascade_parent_blocking`)
- **Auto-close parent** — closing the last open child of a parent automatically closes the parent (and grandparent, etc.) (config: `graph.auto_close_parent`)
- **Cascade-aware `bd swarm`** — swarm validate and swarm status respect cascade blocking when determining task readiness
- **Inherited blocks in `bd show`** — show command now displays effective blockers including inherited ones from parent epics
- **String shorthand for VarDef in TOML formulas**

### Performance
- Replace git subprocess with file walk-up for `.beads` root detection

### Refactoring
- Separate reference golden tests from main e2e test suite
- Rename `update-e2e` / `update-lite-e2e` make targets to `update-e2e-reference` / `update-e2e-lite` for consistent naming
- Extract `configservice` for path resolution and detection
- Consolidate dependency logic into `routing.IssueStore`
- Move business logic from storage to `issueservice`; move ID helpers to `idgen` package
- Add `ARCHITECTURE.md` explaining service layer pattern

### Tests
- Add E2E golden file tests for graph, cascade, and auto-close features
- Add multi-repo benchmark phases
- Disable `auto_close_parent` in e2e sandbox for reference compatibility

### Docs
- Design doc and review for `bd graph` and cascading parent blockers

## v0.49.1

### Features
- **`bd merge-slot` commands** — create, check, acquire, release merge slots
- **`bd init` with empty `.beads` directory** — allow init when directory exists but is empty
- **`--actor` flag on `bd create`**
- **`--reason` flag on `bd close`**
- **`--ephemeral` flag on `bd create`**
- **`--id` and `--force` flags on `bd create`**
- **`--status=all`** to list all non-deleted issues
- **`bd activity` command** (noop shim for upstream compatibility)
- **Pinned status** with consolidated status name display
- **Cross-store dependency support** via routing
- **Colored `bd show` output** and column-aligned `bd list`
- Sort `bd list` output by priority (P0 first)
- Default list shows all non-closed issues, not just open
- Green checkmark on `bd dep add` output
- Replace `child_counters.json` with filesystem scan
- Restructure `.beads` directory layout
- Make `--labels` the primary flag with hidden `--label` alias
- Update formula list to match upstream grouped format
- Convert Priority from string to int type

### Fixes
- Use `Modify()` for dependency and comment ops to avoid lock file races
- Clean up lock files after operations
- Pass explicit `--prefix` in tests for deterministic issue IDs

### Tests
- Add JSON-mode tests for `close --continue` and `--suggest-next`
- Add `main()` error path test with mocked run and osExit
- Add tests for `kvstorage` and `cmd` packages
- Add `test-unit-coverage` Makefile target
