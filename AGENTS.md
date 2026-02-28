# Agent Instructions — beads-lite

This project uses **bd** (beads-lite) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Key Files

- `CLAUDE.md` — build, test, and release instructions
- `ARCHITECTURE.md` — service layer design and package structure
- `CHANGELOG.md` — release notes, updated every version bump

## Package Layout

```
cmd/                    — main entry point
internal/
  cmd/                  — cobra CLI commands (no business logic)
  issueservice/         — business logic: routing, deps, parent-child
  issuestorage/         — IssueStore interface + filesystem implementation
  graph/                — dependency graph algorithms (blockers, waves, auto-close)
  config/               — config types + yamlstore
  configservice/        — config path resolution and discovery
  routing/              — multi-repo routing rules
  kvstorage/            — simple key-value store (slots, agents, merge-slots)
  idgen/                — issue ID generation
  meow/                 — molecule (epic workflow) logic
  agent/                — agent registration
  slot/ mergeslot/      — slot management
e2etests/               — end-to-end tests
  reference/            — golden file comparison tests against reference beads
  concurrency/          — concurrent operation tests
  prettyoutput/         — colored output tests
```

## Working on This Codebase

### Before You Code
- Read `ARCHITECTURE.md` to understand the service layer pattern
- Business logic goes in `issueservice/` or `graph/`, NOT in `cmd/`
- Storage is pure CRUD — no validation or graph logic in `issuestorage/`

### Testing
- Write unit tests alongside implementation (TDD preferred)
- `make test-unit` for fast iteration, `make test-e2e` before committing
- Golden file tests in `e2etests/reference/` compare against the original beads binary — do NOT edit `expected/*.txt` files manually

### Committing
- Commit after each completed chunk of work with passing tests
- Keep commits focused — one logical change per commit

## Release Checklist

**When bumping the version, you MUST complete ALL of these steps:**

1. Update `Version` in `internal/cmd/version.go`
2. Update `CHANGELOG.md` with all changes since the last version
3. Commit both changes together
4. Tag the commit: `git tag v<version>`

Never bump the version without updating the changelog.

## Landing the Plane (Session Completion)

**When ending a work session**, complete ALL steps below. Work is NOT complete until `git push` succeeds.

1. **File issues for remaining work** — create beads for anything needing follow-up
2. **Run quality gates** (if code changed) — `make test` and `make check`
3. **Update issue status** — close finished work, update in-progress items
4. **Push to remote** — this is mandatory:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Hand off** — provide context for next session via h2 message or issue comments
