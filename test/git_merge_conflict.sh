#!/bin/bash
# test/git_merge_conflict.sh
#
# Tests that bd doctor detects corruption caused by git merge conflicts.
# When two branches modify the same issue's title, merging creates invalid JSON.

set -e

# Setup: create a temporary git repository
REPO=$(mktemp -d)
trap "rm -rf $REPO" EXIT

cd "$REPO"
git init --initial-branch=main
git config user.email "test@example.com"
git config user.name "Test User"

# Step 1: Initialize beads
bd init

# Step 2: Create an issue on main
ID=$(bd create "Original title")
git add .beads
git commit -m "Initial issue"

# Step 3: Branch A - update title to "Title from A"
git checkout -b branch-a
bd update "$ID" --title "Title from A"
git commit -am "Update from A"

# Step 4: Branch B - update title to "Title from B"
git checkout main
git checkout -b branch-b
bd update "$ID" --title "Title from B"
git commit -am "Update from B"

# Step 5: Merge A into main
git checkout main
git merge branch-a -m "Merge branch-a"

# Step 6: Merge B into main (should conflict)
# The merge will produce invalid JSON due to conflict markers
git merge branch-b -m "Merge branch-b" || true

# If there's a merge conflict, auto-resolve by accepting the merge
# This simulates a user blindly accepting the conflicted file
if [ -f ".beads/open/${ID}.json" ]; then
    git add ".beads/open/${ID}.json"
    git commit -m "Resolve conflict (corrupted)" --no-edit 2>/dev/null || true
fi

# Step 7: Verify bd doctor detects the corruption
# Doctor should report invalid JSON or parse error due to conflict markers
OUTPUT=$(bd doctor 2>&1) || true

if echo "$OUTPUT" | grep -qiE "invalid JSON|parse error|malformed|corrupt|conflict"; then
    echo "PASS: bd doctor detected merge conflict corruption"
    exit 0
else
    echo "FAIL: bd doctor didn't detect merge conflict corruption"
    echo "Doctor output:"
    echo "$OUTPUT"
    exit 1
fi
