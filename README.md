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

## Testing

```bash
make test          # run all tests (unit + e2e)
make test-unit     # unit tests only
make test-e2e      # e2e tests against local ./bd build
make e2e-update    # regenerate expected e2e outputs from reference bd
```
