# beads-lite

CLI issue tracker. Binary is `bd`. Go project using cobra for CLI, filesystem-based JSON storage.

See also: `AGENTS.md` for agent workflow instructions, `ARCHITECTURE.md` for package design.

## Build & Test

- `make build` — build the `bd` binary
- `make test` — run all tests (unit + e2e)
- `make test-unit` — unit tests only
- `make test-e2e-all` — all e2e tests (builds first)
- `make test-e2e-reference` — golden file comparison tests only
- `make check` — fmt + vet + staticcheck

Pass `ARGS` to filter tests, e.g. `make test-unit ARGS='-run TestCreateWithLabels'`.

## Releases

Version is defined in `internal/cmd/version.go`. When bumping the version:

1. Update the `Version` variable in `internal/cmd/version.go`
2. Update `CHANGELOG.md` with a summary of changes since the last version
3. Commit and tag with `v<version>`

**Always update the changelog when bumping the version.** The changelog lives at `CHANGELOG.md` in the repo root.

## Config Flags

Key config options set in `.beads/config.yaml`:

- `graph.auto_close_parent` — automatically close parent when all children are closed (default: `true`)
- `graph.cascade_parent_blocking` — blockers on parent epics cascade to child tasks (default: `true`)

## Golden File Tests (e2e/reference)

The `e2etests/reference/` tests are golden file tests. Each `case_<nn>_<name>.go` file runs a sequence of commands against the **reference beads implementation** and stores the output in `e2etests/reference/expected/<nn>_<name>.txt`. The same commands are then run against beads-lite and the outputs are compared.

**Do not edit the `expected/*.txt` files manually.** They are generated from the reference implementation.

To regenerate expected output:
```
# All reference cases
make update-e2e-reference

# Single case
make update-e2e-reference ARGS='-run "TestE2E/01_create"'

# Lite-only golden files
make update-e2e-lite
```

Commit the generated `.txt` files after updating.

To validate beads-lite matches the expected output:
```
# All cases
make test-e2e-all

# Single case
make test-e2e-all ARGS='-run "TestE2E/01_create"'
```
