#!/bin/bash

# Create a temporary beads-lite sandbox directory for testing
# Usage: ./setup_bd_sandbox.sh
# Prints the sandbox directory path on success

set -e

BD_CMD="${BD_CMD:-bd}"
SANDBOX_DIR=$(mktemp -d "${TMPDIR:-/tmp}/beads-sandbox-XXXXXXXX")

$BD_CMD init --path "$SANDBOX_DIR" > /dev/null 2>&1

echo "$SANDBOX_DIR"
