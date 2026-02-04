# beads-lite

CLI issue tracker. Binary is `bd`. Go project using cobra for CLI, filesystem-based JSON storage.

## Build & Test

- `make build` — build the `bd` binary
- `make test` — run all tests (unit + e2e)
- `make test-unit` — unit tests only
- `make test-e2e` — e2e tests only (builds first)

Pass `ARGS` to filter tests, e.g. `make test-unit ARGS='-run TestCreateWithLabels'`.

## Golden File Tests (e2e/reference)

The `e2etests/reference/` tests are golden file tests. Each `case_<nn>_<name>.go` file runs a sequence of commands against the **reference beads implementation** and stores the output in `e2etests/reference/expected/<nn>_<name>.txt`. The same commands are then run against beads-lite and the outputs are compared.

**Do not edit the `expected/*.txt` files manually.** They are generated from the reference implementation.

To regenerate expected output:
```
# All cases
make update-e2e

# Single case
make update-e2e ARGS='-run "TestE2E/01_create"'
```

Commit the generated `.txt` files after updating.

To validate beads-lite matches the expected output:
```
# All cases
make test-e2e

# Single case
make test-e2e ARGS='-run "TestE2E/01_create"'
```
