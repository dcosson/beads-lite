# E2E Tests

End-to-end tests that exercise `bd` commands via subprocess and compare output against golden master files.

## Architecture

Each test case is a Go function in `case_*.go` that:
1. Sets up a fresh sandbox (temp directory with `bd init`)
2. Runs a sequence of `bd` commands via `Runner.Run()`
3. Normalizes output (replaces dynamic IDs and timestamps with stable placeholders)
4. Returns a string of `=== section ===` blocks

The test framework (`e2e_test.go`) compares each case's output against `expected/<name>.txt`.

## Key Files

| File | Purpose |
|------|---------|
| `helpers.go` | Test case registry (`testCases` slice) and shared helpers (`section`, `mustRun`, `mustExtractID`) |
| `runner.go` | `Runner` struct — executes `bd` commands as subprocesses with `BEADS_DIR` env |
| `normalize.go` | `Normalizer` — replaces issue IDs, comment IDs, and timestamps with deterministic placeholders (`ISSUE_1`, `COMMENT_1`, `TIMESTAMP`) |
| `commands.go` | `knownCommands` registry — tracks which `bd` commands are covered by E2E tests |
| `e2e_test.go` | Test driver — runs all cases, compares against expected output |
| `expected/` | Golden master output files (one per test case) |

## Running Tests

```bash
# Build the beads-lite binary first
make build

# Run all E2E tests
make test-e2e

# Run a specific test case
BD_CMD=./bd go test ./e2etests -v -count=1 -run TestE2E/14_meow

# Run all tests (unit + E2E)
make test
```

## Generating Expected Output

Standard test cases (01-14) use the **reference beads binary** — the original `beads` CLI (not beads-lite) — to generate golden output. This ensures beads-lite produces output identical to the original implementation.

The `BD_REF_CMD` environment variable specifies the path to the reference binary. If unset, the Makefile assumes `beads` is available in your `PATH`.

```bash
# Uses BD_REF_CMD if set, otherwise looks for 'beads' in PATH
make update-e2e

# Or explicitly specify the reference binary:
BD_REF_CMD=/path/to/beads make update-e2e
```

The meow test case (`14_meow`) also uses the reference binary — the reference `beads` CLI supports MEOW commands.

## Writing a New Test Case

1. Create `case_NN_name.go` with a function matching `func(r *Runner, n *Normalizer, sandbox string) (string, error)`
2. Register it in `helpers.go`: `{"NN_name", caseName}`
3. Add any new commands to `knownCommands` in `commands.go`
4. Generate expected output (see above)
5. Run `make test-e2e` to verify

### Pattern for Dynamic IDs

When a test needs to use IDs from one command's output in subsequent commands, extract them from raw JSON **before** normalization:

```go
result, err := mustRun(r, sandbox, "mol", "pour", "my-formula", "--json")
// Extract actual IDs for use in subsequent commands
var pourRes struct {
    RootID   string   `json:"RootID"`
    ChildIDs []string `json:"ChildIDs"`
}
json.Unmarshal([]byte(result.Stdout), &pourRes)
rootID := pourRes.RootID

// Normalize for section output (IDs become ISSUE_1, etc.)
section(&out, "pour", n.NormalizeJSON([]byte(result.Stdout)))

// Use actual ID in next command
result, err = mustRun(r, sandbox, "show", rootID, "--json")
```

### Setting Environment Variables

`Runner.Run()` passes `os.Environ()` to child processes. To set env vars like `BD_ACTOR`:

```go
os.Setenv("BD_ACTOR", "testuser")
defer os.Unsetenv("BD_ACTOR")
// Subsequent Runner.Run() calls will see BD_ACTOR=testuser
```
