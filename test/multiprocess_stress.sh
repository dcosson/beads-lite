#!/bin/bash
# test/multiprocess_stress.sh
# Tests that concurrent creates from multiple processes produce unique IDs
# and don't corrupt the storage.

set -e

# Configuration
NUM_PROCESSES=20

# Create temporary beads directory
BEADS_DIR=$(mktemp -d)
trap "rm -rf $BEADS_DIR" EXIT

echo "Testing multi-process concurrent creates..."
echo "  Processes: $NUM_PROCESSES"
echo "  Beads dir: $BEADS_DIR"

# Initialize beads in the temp directory
bd init --path "$BEADS_DIR"

# Spawn processes doing concurrent creates
# Each process creates one issue
pids=()
for i in $(seq 1 $NUM_PROCESSES); do
    bd create "Issue $i" --path "$BEADS_DIR" &
    pids+=($!)
done

# Wait for all processes to complete
failed=0
for pid in "${pids[@]}"; do
    if ! wait "$pid"; then
        echo "Process $pid failed"
        failed=$((failed + 1))
    fi
done

if [ $failed -gt 0 ]; then
    echo "FAIL: $failed processes failed during create"
    exit 1
fi

# Verify all issues were created
COUNT=$(bd list --path "$BEADS_DIR" --format ids | wc -l | tr -d ' ')
if [ "$COUNT" -ne "$NUM_PROCESSES" ]; then
    echo "FAIL: Expected $NUM_PROCESSES issues, got $COUNT"
    bd list --path "$BEADS_DIR"
    exit 1
fi

# Verify all IDs are unique (no collisions)
UNIQUE_COUNT=$(bd list --path "$BEADS_DIR" --format ids | sort -u | wc -l | tr -d ' ')
if [ "$UNIQUE_COUNT" -ne "$NUM_PROCESSES" ]; then
    echo "FAIL: Expected $NUM_PROCESSES unique IDs, got $UNIQUE_COUNT"
    echo "Duplicate IDs detected:"
    bd list --path "$BEADS_DIR" --format ids | sort | uniq -d
    exit 1
fi

# Verify bd doctor finds no problems
PROBLEMS=$(bd doctor --path "$BEADS_DIR" 2>&1 | grep -c "problem\|error\|corrupt" || true)
if [ "$PROBLEMS" -ne 0 ]; then
    echo "FAIL: Doctor found problems after concurrent creates"
    bd doctor --path "$BEADS_DIR"
    exit 1
fi

echo "PASS: Multi-process creates ($NUM_PROCESSES processes, $COUNT unique issues)"
