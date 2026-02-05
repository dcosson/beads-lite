package issuestorage

import (
	"context"
	"errors"
	"testing"
	"time"

	"beads-lite/internal/idgen"
)

// RunContractTests runs the full contract test suite against a IssueStore implementation.
// Each storage engine should call this with its own factory function to ensure
// consistent behavior across all implementations.
func RunContractTests(t *testing.T, factory func() IssueStore) {
	t.Run("Create", func(t *testing.T) { testCreate(t, factory()) })
	t.Run("Get", func(t *testing.T) { testGet(t, factory()) })
	t.Run("Modify", func(t *testing.T) { testModify(t, factory()) })
	t.Run("Delete", func(t *testing.T) { testDelete(t, factory()) })
	t.Run("List", func(t *testing.T) { testList(t, factory()) })
	t.Run("CloseReopen", func(t *testing.T) { testCloseReopen(t, factory()) })
	t.Run("ChildCounters", func(t *testing.T) { testChildCounters(t, factory()) })
	t.Run("HierarchyDepthLimit", func(t *testing.T) { testHierarchyDepthLimit(t, factory()) })
	t.Run("TombstoneStatus", func(t *testing.T) { testTombstoneStatus(t, factory()) })
}

func testCreate(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	now := time.Now()
	issue := &Issue{
		Title:       "Test Issue",
		Description: "Test description",
		Status:      StatusOpen,
		Priority:    PriorityMedium,
		Type:        TypeTask,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == "" {
		t.Error("Create returned empty ID")
	}

	// Verify the issue was stored
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Create failed: %v", err)
	}
	if got.Title != issue.Title {
		t.Errorf("Title mismatch: got %q, want %q", got.Title, issue.Title)
	}
	if got.Description != issue.Description {
		t.Errorf("Description mismatch: got %q, want %q", got.Description, issue.Description)
	}
	if got.Status != issue.Status {
		t.Errorf("Status mismatch: got %q, want %q", got.Status, issue.Status)
	}
	if got.Priority != issue.Priority {
		t.Errorf("Priority mismatch: got %d, want %d", got.Priority, issue.Priority)
	}
	if got.Type != issue.Type {
		t.Errorf("Type mismatch: got %q, want %q", got.Type, issue.Type)
	}
	// Storage persists timestamps as given (issueservice sets them)
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be persisted")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be persisted")
	}
}

func testGet(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Get non-existent issue should return ErrNotFound
	_, err := s.Get(ctx, "nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("Get non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue and verify we can get it
	now := time.Now()
	issue := &Issue{
		Title:     "Get Test",
		Status:    StatusOpen,
		Priority:  PriorityLow,
		Type:      TypeBug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, id)
	}
	if got.Title != issue.Title {
		t.Errorf("Title mismatch: got %q, want %q", got.Title, issue.Title)
	}
}

func testModify(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Modify non-existent issue should return ErrNotFound
	err := s.Modify(ctx, "nonexistent-id", func(i *Issue) error { i.Title = "test"; return nil })
	if err != ErrNotFound {
		t.Errorf("Modify non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue
	now := time.Now()
	issue := &Issue{
		Title:       "Original Title",
		Description: "Original description",
		Status:      StatusOpen,
		Priority:    PriorityLow,
		Type:        TypeTask,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Modify it
	updated := &Issue{
		ID:          id,
		Title:       "Updated Title",
		Description: "Updated description",
		Status:      StatusInProgress,
		Priority:    PriorityHigh,
		Type:        TypeTask,
		Labels:      []string{"urgent"},
		Assignee:    "alice",
		CreatedAt:   now,
		UpdatedAt:   time.Now(),
	}
	if err := s.Modify(ctx, id, func(i *Issue) error { *i = *updated; return nil }); err != nil {
		t.Fatalf("Modify failed: %v", err)
	}

	// Verify the update
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Modify failed: %v", err)
	}
	if got.Title != updated.Title {
		t.Errorf("Title mismatch: got %q, want %q", got.Title, updated.Title)
	}
	if got.Description != updated.Description {
		t.Errorf("Description mismatch: got %q, want %q", got.Description, updated.Description)
	}
	if got.Status != updated.Status {
		t.Errorf("Status mismatch: got %q, want %q", got.Status, updated.Status)
	}
	if got.Priority != updated.Priority {
		t.Errorf("Priority mismatch: got %d, want %d", got.Priority, updated.Priority)
	}
	if got.Assignee != updated.Assignee {
		t.Errorf("Assignee mismatch: got %q, want %q", got.Assignee, updated.Assignee)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "urgent" {
		t.Errorf("Labels mismatch: got %v, want [urgent]", got.Labels)
	}
}

func testDelete(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Delete non-existent issue should return ErrNotFound
	err := s.Delete(ctx, "nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("Delete non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue
	now := time.Now()
	issue := &Issue{
		Title:     "To Delete",
		Status:    StatusOpen,
		Priority:  PriorityLow,
		Type:      TypeChore,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it exists
	if _, err := s.Get(ctx, id); err != nil {
		t.Fatalf("Get before Delete failed: %v", err)
	}

	// Delete it
	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it no longer exists
	_, err = s.Get(ctx, id)
	if err != ErrNotFound {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}

	// Delete again should return ErrNotFound
	err = s.Delete(ctx, id)
	if err != ErrNotFound {
		t.Errorf("Double Delete: got %v, want ErrNotFound", err)
	}
}

func testList(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Empty list should return empty slice
	issues, err := s.List(ctx, nil)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("List empty: got %d issues, want 0", len(issues))
	}

	// Create several issues
	now := time.Now()
	ids := make([]string, 0, 4)
	for i, spec := range []struct {
		title    string
		status   Status
		priority Priority
		typ      IssueType
		labels   []string
	}{
		{"Task 1", StatusOpen, PriorityHigh, TypeTask, []string{"frontend"}},
		{"Task 2", StatusInProgress, PriorityLow, TypeTask, []string{"backend"}},
		{"Bug 1", StatusOpen, PriorityHigh, TypeBug, []string{"frontend", "urgent"}},
		{"Feature 1", StatusClosed, PriorityMedium, TypeFeature, nil},
	} {
		issue := &Issue{
			Title:     spec.title,
			Status:    spec.status,
			Priority:  spec.priority,
			Type:      spec.typ,
			Labels:    spec.labels,
			CreatedAt: now,
			UpdatedAt: now,
		}
		id, err := s.Create(ctx, issue)
		if err != nil {
			t.Fatalf("Create issue %d failed: %v", i, err)
		}
		ids = append(ids, id)
	}

	// List with nil filter should return open issues (default behavior)
	_, err = s.List(ctx, nil)
	if err != nil {
		t.Fatalf("List nil filter: %v", err)
	}
	// Should get open issues: Task 1, Bug 1 (Task 2 is in_progress, Feature 1 is closed)
	// Actually, the interface says nil filter returns "all open issues" - need to verify what that means

	// Filter by status
	statusOpen := StatusOpen
	issues, err = s.List(ctx, &ListFilter{Status: &statusOpen})
	if err != nil {
		t.Fatalf("List by status: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("List by status open: got %d issues, want 2", len(issues))
	}

	// Filter by priority
	priorityHigh := PriorityHigh
	issues, err = s.List(ctx, &ListFilter{Priority: &priorityHigh})
	if err != nil {
		t.Fatalf("List by priority: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("List by priority high: got %d issues, want 2", len(issues))
	}

	// Filter by type
	typeBug := TypeBug
	issues, err = s.List(ctx, &ListFilter{Type: &typeBug})
	if err != nil {
		t.Fatalf("List by type: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("List by type bug: got %d issues, want 1", len(issues))
	}

	// Filter by labels
	issues, err = s.List(ctx, &ListFilter{Labels: []string{"frontend"}})
	if err != nil {
		t.Fatalf("List by labels: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("List by label frontend: got %d issues, want 2", len(issues))
	}

	// Filter by multiple labels (must have all)
	issues, err = s.List(ctx, &ListFilter{Labels: []string{"frontend", "urgent"}})
	if err != nil {
		t.Fatalf("List by multiple labels: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("List by labels [frontend, urgent]: got %d issues, want 1", len(issues))
	}

	_ = ids // silence unused variable warning
}

func testCloseReopen(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Close non-existent issue should return ErrNotFound
	err := s.Modify(ctx, "nonexistent-id", func(i *Issue) error { i.Status = StatusClosed; return nil })
	if err != ErrNotFound {
		t.Errorf("Close non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue
	now := time.Now()
	issue := &Issue{
		Title:     "To Close",
		Status:    StatusOpen,
		Priority:  PriorityLow,
		Type:      TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Close it (storage handles file movement, issueservice handles ClosedAt)
	closedAt := time.Now()
	if err := s.Modify(ctx, id, func(i *Issue) error {
		i.Status = StatusClosed
		i.ClosedAt = &closedAt
		return nil
	}); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify status changed and issue is retrievable from closed dir
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Close failed: %v", err)
	}
	if got.Status != StatusClosed {
		t.Errorf("Status after Close: got %q, want %q", got.Status, StatusClosed)
	}
	if got.ClosedAt == nil {
		t.Error("ClosedAt should be persisted after Close")
	}

	// Reopen it (storage handles file movement, issueservice handles ClosedAt clearing)
	if err := s.Modify(ctx, id, func(i *Issue) error {
		i.Status = StatusOpen
		i.ClosedAt = nil
		return nil
	}); err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}

	// Verify status changed back and issue is retrievable from open dir
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Reopen failed: %v", err)
	}
	if got.Status != StatusOpen {
		t.Errorf("Status after Reopen: got %q, want %q", got.Status, StatusOpen)
	}
	if got.ClosedAt != nil {
		t.Error("ClosedAt should be nil after Reopen")
	}
}

func testChildCounters(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	now := time.Now()
	makeIssue := func(id, title string) *Issue {
		return &Issue{ID: id, Title: title, Status: StatusOpen, CreatedAt: now, UpdatedAt: now}
	}

	// Create parent issues so GetNextChildID can validate them
	parentA := makeIssue("", "Parent A")
	idA, err := s.Create(ctx, parentA)
	if err != nil {
		t.Fatalf("Create parent A failed: %v", err)
	}

	parentB := makeIssue("", "Parent B")
	idB, err := s.Create(ctx, parentB)
	if err != nil {
		t.Fatalf("Create parent B failed: %v", err)
	}

	// First child should return parentID.1
	childID, err := s.GetNextChildID(ctx, idA)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	wantFirst := idA + ".1"
	if childID != wantFirst {
		t.Errorf("First child ID: got %q, want %q", childID, wantFirst)
	}
	// Create so next scan sees it
	_, err = s.Create(ctx, makeIssue(childID, "Child A.1"))
	if err != nil {
		t.Fatalf("Create child A.1 failed: %v", err)
	}

	// Second child should return parentID.2
	childID, err = s.GetNextChildID(ctx, idA)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	wantSecond := idA + ".2"
	if childID != wantSecond {
		t.Errorf("Second child ID: got %q, want %q", childID, wantSecond)
	}
	_, err = s.Create(ctx, makeIssue(childID, "Child A.2"))
	if err != nil {
		t.Fatalf("Create child A.2 failed: %v", err)
	}

	// Different parent should start at .1
	childID, err = s.GetNextChildID(ctx, idB)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	wantB := idB + ".1"
	if childID != wantB {
		t.Errorf("First child of different parent: got %q, want %q", childID, wantB)
	}
	_, err = s.Create(ctx, makeIssue(childID, "Child B.1"))
	if err != nil {
		t.Fatalf("Create child B.1 failed: %v", err)
	}

	// Original parent should continue from .3
	childID, err = s.GetNextChildID(ctx, idA)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	wantThird := idA + ".3"
	if childID != wantThird {
		t.Errorf("Third child of parent A: got %q, want %q", childID, wantThird)
	}

	// Non-existent parent should return ErrNotFound
	_, err = s.GetNextChildID(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetNextChildID on non-existent parent: got %v, want ErrNotFound", err)
	}
}

func testHierarchyDepthLimit(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	now := time.Now()
	makeIssue := func(id, title string) *Issue {
		return &Issue{ID: id, Title: title, Status: StatusOpen, CreatedAt: now, UpdatedAt: now}
	}

	// Create a root issue
	root := makeIssue("", "Root")
	rootID, err := s.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create root failed: %v", err)
	}

	// Build chain: root -> depth 1 -> depth 2 -> depth 3
	depth1ID, err := s.GetNextChildID(ctx, rootID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 1 failed: %v", err)
	}
	s.Create(ctx, makeIssue(depth1ID, "Depth 1"))

	depth2ID, err := s.GetNextChildID(ctx, depth1ID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 2 failed: %v", err)
	}
	s.Create(ctx, makeIssue(depth2ID, "Depth 2"))

	depth3ID, err := s.GetNextChildID(ctx, depth2ID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 3 failed: %v", err)
	}
	s.Create(ctx, makeIssue(depth3ID, "Depth 3"))

	// Depth 4 should be rejected
	_, err = s.GetNextChildID(ctx, depth3ID)
	if !errors.Is(err, idgen.ErrMaxDepthExceeded) {
		t.Errorf("GetNextChildID at depth 4: got %v, want idgen.ErrMaxDepthExceeded", err)
	}
}

func testTombstoneStatus(t *testing.T, s IssueStore) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue to tombstone
	now := time.Now()
	issue := &Issue{
		Title:     "To Tombstone",
		Status:    StatusOpen,
		Priority:  PriorityMedium,
		Type:      TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Tombstone it via Modify
	if err := s.Modify(ctx, id, func(issue *Issue) error {
		issue.OriginalType = issue.Type
		issue.Status = StatusTombstone
		now := time.Now()
		issue.DeletedAt = &now
		issue.DeletedBy = "test-actor"
		issue.DeleteReason = "no longer needed"
		return nil
	}); err != nil {
		t.Fatalf("Modify to tombstone failed: %v", err)
	}

	// Get() should still return the issue
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after tombstone failed: %v", err)
	}

	// Verify tombstone metadata
	if got.Status != StatusTombstone {
		t.Errorf("Status: got %q, want %q", got.Status, StatusTombstone)
	}
	if got.DeletedAt == nil {
		t.Error("DeletedAt should be set")
	}
	if got.DeletedBy != "test-actor" {
		t.Errorf("DeletedBy: got %q, want %q", got.DeletedBy, "test-actor")
	}
	if got.DeleteReason != "no longer needed" {
		t.Errorf("DeleteReason: got %q, want %q", got.DeleteReason, "no longer needed")
	}
	if got.OriginalType != TypeTask {
		t.Errorf("OriginalType: got %q, want %q", got.OriginalType, TypeTask)
	}
	if got.ClosedAt != nil {
		t.Error("ClosedAt should be nil after tombstone")
	}

	// List(nil) should NOT include tombstoned issue
	openIssues, err := s.List(ctx, nil)
	if err != nil {
		t.Fatalf("List open failed: %v", err)
	}
	for _, iss := range openIssues {
		if iss.ID == id {
			t.Error("Tombstoned issue should not appear in default list")
		}
	}

	// List with StatusTombstone filter SHOULD include it
	tombstoneStatus := StatusTombstone
	tombstones, err := s.List(ctx, &ListFilter{Status: &tombstoneStatus})
	if err != nil {
		t.Fatalf("List tombstones failed: %v", err)
	}
	found := false
	for _, iss := range tombstones {
		if iss.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("Tombstoned issue should appear when filtering by tombstone status")
	}

	// Hard Delete() should still work on tombstoned issues
	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete tombstoned issue failed: %v", err)
	}
	_, err = s.Get(ctx, id)
	if err != ErrNotFound {
		t.Errorf("Get after hard delete of tombstone: got %v, want ErrNotFound", err)
	}

	// Test tombstoning a closed issue (storage persists what it's given)
	closedIssue := &Issue{
		Title:     "Closed then tombstoned",
		Status:    StatusOpen,
		Priority:  PriorityLow,
		Type:      TypeBug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	closedID, err := s.Create(ctx, closedIssue)
	if err != nil {
		t.Fatalf("Create closed issue failed: %v", err)
	}
	closedAt := time.Now()
	if err := s.Modify(ctx, closedID, func(i *Issue) error {
		i.Status = StatusClosed
		i.ClosedAt = &closedAt
		return nil
	}); err != nil {
		t.Fatalf("Close issue failed: %v", err)
	}
	// When tombstoning, the caller (issueservice) is responsible for clearing ClosedAt
	if err := s.Modify(ctx, closedID, func(i *Issue) error {
		i.Status = StatusTombstone
		i.ClosedAt = nil // Caller clears this
		return nil
	}); err != nil {
		t.Fatalf("Modify closed issue to tombstone failed: %v", err)
	}
	gotClosed, err := s.Get(ctx, closedID)
	if err != nil {
		t.Fatalf("Get closed-then-tombstoned issue failed: %v", err)
	}
	if gotClosed.Status != StatusTombstone {
		t.Errorf("Closed-then-tombstoned status: got %q, want %q", gotClosed.Status, StatusTombstone)
	}
	if gotClosed.ClosedAt != nil {
		t.Error("ClosedAt should be nil (caller cleared it)")
	}
}

// containsStr checks if a slice contains a specific string.
func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
