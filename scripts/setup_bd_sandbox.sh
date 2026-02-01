#!/bin/bash

# Create a temporary beads-lite sandbox directory for testing
# Usage: ./setup_bd_sandbox.sh
# Prints the sandbox directory path on success

set -e

BD_CMD="${BD_CMD:-bd}"
SANDBOX_DIR=$(mktemp -d "${TMPDIR:-/tmp}/beads-sandbox-XXXXXXXX")

# cd to sandbox before init to:
# 1. Avoid git worktree detection in the original bd command
# 2. Ensure any generated files (AGENTS.md, .gitignore) go in the sandbox, not workspace
cd "$SANDBOX_DIR"

# Initialize a git repo so the reference bd binary's daemon can start
# (it needs a repo fingerprint). Also needed for beads-lite's cwd discovery.
git init -q

# Run init with BEADS_DIR so beads-lite creates .beads/ at the right place.
# The reference binary stores data at BEADS_DIR root (no .beads/ subdirectory).
BEADS_DIR="$SANDBOX_DIR" $BD_CMD init > /dev/null 2>&1

# The reference binary's "config set" looks for .beads/config.yaml even when
# BEADS_DIR is set. Ensure .beads/ exists with a copy of config.yaml so both
# the reference binary and beads-lite can find it.
if [ ! -d "$SANDBOX_DIR/.beads" ]; then
    mkdir -p "$SANDBOX_DIR/.beads/formulas"
    # Copy config if it was created at root level (reference binary layout)
    if [ -f "$SANDBOX_DIR/config.yaml" ]; then
        cp "$SANDBOX_DIR/config.yaml" "$SANDBOX_DIR/.beads/config.yaml"
    fi
fi

echo "$SANDBOX_DIR"
