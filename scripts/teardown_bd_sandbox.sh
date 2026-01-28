#!/bin/bash

# Tear down a beads-lite sandbox directory
# Usage: ./teardown_bd_sandbox.sh <sandbox-dir>
#        ./teardown_bd_sandbox.sh --all

set -e

TMPDIR="${TMPDIR:-/tmp}"
PREFIX="beads-sandbox-"

if [ "$1" = "--all" ]; then
    count=0
    for dir in "$TMPDIR"/${PREFIX}*; do
        [ -d "$dir" ] || continue
        rm -rf "$dir"
        count=$((count + 1))
    done
    echo "Removed $count sandbox(es)"
elif [ -n "$1" ]; then
    if [ ! -d "$1" ]; then
        echo "Error: directory not found: $1" >&2
        exit 1
    fi
    rm -rf "$1"
    echo "Removed $1"
else
    echo "Usage: $0 <sandbox-dir>" >&2
    echo "       $0 --all" >&2
    exit 1
fi
