#!/bin/bash

# Benchmark script for beads issue operations
# Usage: ./benchmark_beads.sh [count]
#   count: number of issues to create (default: 10)

set -e

# Configurable bd command - swap this out to test different implementations
BD_CMD="${BD_CMD:-bd}"

# Number of issues to create
COUNT="${1:-10}"

echo "=== Beads Benchmark ==="
echo "Command: $BD_CMD"
echo "Issue count: $COUNT"
echo ""

# Array to store created issue IDs
declare -a ISSUE_IDS

# --- Block 1: Create issues ---
echo "--- Creating $COUNT issues ---"
START_CREATE=$(date +%s.%N)

for i in $(seq 1 $COUNT); do
    # Create issue and capture the ID from output
    OUTPUT=$($BD_CMD create --title="Benchmark issue $i" --type=task --priority=2 2>&1)
    # Extract issue ID - looks for "Created issue: <id>" and grabs the ID
    ISSUE_ID=$(echo "$OUTPUT" | grep "Created issue:" | awk '{print $NF}')
    if [ -z "$ISSUE_ID" ]; then
        echo "Error: Could not extract issue ID from output: $OUTPUT"
        exit 1
    fi
    ISSUE_IDS+=("$ISSUE_ID")
done

END_CREATE=$(date +%s.%N)
CREATE_TIME=$(echo "$END_CREATE - $START_CREATE" | bc)
echo "Create time: ${CREATE_TIME}s"
echo ""

# --- Block 2: Add dependencies (each issue depends on previous) ---
echo "--- Adding dependencies ---"
START_DEP=$(date +%s.%N)

for i in $(seq 1 $((COUNT - 1))); do
    CURRENT_IDX=$((i))  # 0-indexed: issue at position i
    PREV_IDX=$((i - 1)) # previous issue
    CURRENT_ID="${ISSUE_IDS[$CURRENT_IDX]}"
    PREV_ID="${ISSUE_IDS[$PREV_IDX]}"
    $BD_CMD dep add "$CURRENT_ID" "$PREV_ID" > /dev/null
done

END_DEP=$(date +%s.%N)
DEP_TIME=$(echo "$END_DEP - $START_DEP" | bc)
echo "Dependency time: ${DEP_TIME}s"
echo ""

# --- Block 3: Update descriptions ---
echo "--- Updating descriptions ---"
START_UPDATE=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD update "$ISSUE_ID" --description="This is the updated description for benchmark issue $((i + 1)). Created as part of performance testing." > /dev/null
done

END_UPDATE=$(date +%s.%N)
UPDATE_TIME=$(echo "$END_UPDATE - $START_UPDATE" | bc)
echo "Update time: ${UPDATE_TIME}s"
echo ""

# --- Block 4: Read all issues ---
echo "--- Reading issues with bd show ---"
START_READ=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD show "$ISSUE_ID" > /dev/null
done

END_READ=$(date +%s.%N)
READ_TIME=$(echo "$END_READ - $START_READ" | bc)
echo "Read time: ${READ_TIME}s"
echo ""

# --- Summary ---
echo "=== Summary ==="
echo "Issues created: $COUNT"
echo "Create time:     ${CREATE_TIME}s"
echo "Dependency time: ${DEP_TIME}s"
echo "Update time:     ${UPDATE_TIME}s"
echo "Read time:       ${READ_TIME}s"
TOTAL_TIME=$(echo "$CREATE_TIME + $DEP_TIME + $UPDATE_TIME + $READ_TIME" | bc)
echo "Total time:      ${TOTAL_TIME}s"
echo ""
echo "Issue IDs created: ${ISSUE_IDS[*]}"
