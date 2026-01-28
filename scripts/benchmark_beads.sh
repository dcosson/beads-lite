#!/bin/bash

# Benchmark script for beads-lite issue operations
# Usage: ./benchmark_beads.sh [count]
#   count: number of issues to create (default: 10)

set -e

# Configurable bd command - swap this out to test different implementations
BD_CMD="${BD_CMD:-bd}"

# Number of issues to create
COUNT="${1:-10}"

echo "=== Beads Lite Benchmark ==="
echo "Command: $BD_CMD"
echo "Issue count: $COUNT"

# Array to store created issue IDs
declare -a ISSUE_IDS

# --- Block 1: Create issues ---
echo -n "Creating $COUNT issues: "
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
echo "Done"

# --- Block 2: Add dependencies (each issue depends on previous) ---
echo -n "Adding dependencies: "
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
echo "Done"

# --- Block 3: Update descriptions ---
echo -n "Updating descriptions: "
START_UPDATE=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD update "$ISSUE_ID" --description="This is the updated description for benchmark issue $((i + 1)). Created as part of performance testing." > /dev/null
done

END_UPDATE=$(date +%s.%N)
UPDATE_TIME=$(echo "$END_UPDATE - $START_UPDATE" | bc)
echo "Done"

# --- Block 4: Change status to in_progress ---
echo -n "Setting status to in_progress: "
START_INPROGRESS=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD update "$ISSUE_ID" --status=in_progress > /dev/null
done

END_INPROGRESS=$(date +%s.%N)
INPROGRESS_TIME=$(echo "$END_INPROGRESS - $START_INPROGRESS" | bc)
echo "Done"

# --- Block 5: Read all issues (in_progress) ---
echo -n "Reading issues (in_progress): "
START_READ1=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD show "$ISSUE_ID" > /dev/null
done

END_READ1=$(date +%s.%N)
READ1_TIME=$(echo "$END_READ1 - $START_READ1" | bc)
echo "Done"

# --- Block 6: Change status to done (close) ---
echo -n "Closing issues: "
START_CLOSE=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD close "$ISSUE_ID" > /dev/null
done

END_CLOSE=$(date +%s.%N)
CLOSE_TIME=$(echo "$END_CLOSE - $START_CLOSE" | bc)
echo "Done"

# --- Block 7: Read all issues (closed) ---
echo -n "Reading issues (closed): "
START_READ2=$(date +%s.%N)

for i in $(seq 0 $((COUNT - 1))); do
    ISSUE_ID="${ISSUE_IDS[$i]}"
    $BD_CMD show "$ISSUE_ID" > /dev/null
done

END_READ2=$(date +%s.%N)
READ2_TIME=$(echo "$END_READ2 - $START_READ2" | bc)
echo "Done"

# --- Summary ---
TOTAL_TIME=$(echo "$CREATE_TIME + $DEP_TIME + $UPDATE_TIME + $INPROGRESS_TIME + $READ1_TIME + $CLOSE_TIME + $READ2_TIME" | bc)

echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║            BENCHMARK RESULTS                   ║"
echo "╠════════════════════════════════════════════════╣"
printf "║  %-30s %10s     ║\n" "Issues:" "$COUNT"
echo "╠════════════════════════════════════════════════╣"
printf "║  %-30s %10.2fs    ║\n" "Create issues" "$CREATE_TIME"
printf "║  %-30s %10.2fs    ║\n" "Add dependencies" "$DEP_TIME"
printf "║  %-30s %10.2fs    ║\n" "Update descriptions" "$UPDATE_TIME"
printf "║  %-30s %10.2fs    ║\n" "Set status in_progress" "$INPROGRESS_TIME"
printf "║  %-30s %10.2fs    ║\n" "Read issues (in_progress)" "$READ1_TIME"
printf "║  %-30s %10.2fs    ║\n" "Close issues" "$CLOSE_TIME"
printf "║  %-30s %10.2fs    ║\n" "Read issues (closed)" "$READ2_TIME"
echo "╠════════════════════════════════════════════════╣"
printf "║  %-30s %10.2fs    ║\n" "TOTAL" "$TOTAL_TIME"
echo "╚════════════════════════════════════════════════╝"
echo ""
echo "Issue IDs created: ${ISSUE_IDS[*]}"
