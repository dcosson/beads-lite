#!/bin/bash
# test/multiprocess_update_race.sh
# Tests that concurrent updates to the same issue don't corrupt data.
# Verifies cross-process locking works correctly.

set -e

# Configuration
NUM_PROCESSES=50

# Create temporary beads directory
BEADS_DIR=$(mktemp -d)
trap "rm -rf $BEADS_DIR" EXIT

echo "Testing multi-process concurrent updates to same issue..."
echo "  Processes: $NUM_PROCESSES"
echo "  Beads dir: $BEADS_DIR"

# Initialize beads in the temp directory
bd init --path "$BEADS_DIR"

# Create a single test issue
ID=$(bd create "Test Issue" --path "$BEADS_DIR")
echo "  Test issue: $ID"

# Spawn processes that all update the same issue concurrently
# Each process updates the title to include its process number
pids=()
for i in $(seq 1 $NUM_PROCESSES); do
    bd update "$ID" --title "Updated by $i" --path "$BEADS_DIR" &
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
    echo "FAIL: $failed processes failed during update"
    exit 1
fi

# Verify issue is still valid JSON and readable
if ! bd show "$ID" --path "$BEADS_DIR" --json | jq . > /dev/null 2>&1; then
    echo "FAIL: Issue corrupted - invalid JSON"
    echo "Raw content:"
    bd show "$ID" --path "$BEADS_DIR" --json || cat "$BEADS_DIR/.beads/issues/open/$ID.json" 2>/dev/null || true
    exit 1
fi

# Verify the issue has a valid title (one of the updates should have won)
TITLE=$(bd show "$ID" --path "$BEADS_DIR" --json | jq -r '.title')
if [ -z "$TITLE" ] || [ "$TITLE" = "null" ]; then
    echo "FAIL: Issue has empty or null title after updates"
    bd show "$ID" --path "$BEADS_DIR"
    exit 1
fi

# Verify title matches expected pattern (should be "Updated by N" for some N)
if ! echo "$TITLE" | grep -qE "^Updated by [0-9]+$"; then
    echo "FAIL: Title doesn't match expected pattern: $TITLE"
    exit 1
fi

# Verify bd doctor finds no problems
PROBLEMS=$(bd doctor --path "$BEADS_DIR" 2>&1 | grep -c "problem\|error\|corrupt" || true)
if [ "$PROBLEMS" -ne 0 ]; then
    echo "FAIL: Doctor found problems after concurrent updates"
    bd doctor --path "$BEADS_DIR"
    exit 1
fi

echo "PASS: Concurrent updates to same issue ($NUM_PROCESSES processes)"
echo "  Final title: $TITLE"
