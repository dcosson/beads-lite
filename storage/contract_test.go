package storage

import (
	"context"
	"testing"
	"time"
)

// RunContractTests runs the full contract test suite against a Storage implementation.
// Each storage engine should call this with its own factory function to ensure
// consistent behavior across all implementations.
func RunContractTests(t *testing.T, factory func() Storage) {
	t.Run("Create", func(t *testing.T) { testCreate(t, factory()) })
	t.Run("Get", func(t *testing.T) { testGet(t, factory()) })
	t.Run("Update", func(t *testing.T) { testUpdate(t, factory()) })
	t.Run("Delete", func(t *testing.T) { testDelete(t, factory()) })
	t.Run("List", func(t *testing.T) { testList(t, factory()) })
	t.Run("Close", func(t *testing.T) { testClose(t, factory()) })
	t.Run("Dependencies", func(t *testing.T) { testDependencies(t, factory()) })
	t.Run("Blocking", func(t *testing.T) { testBlocking(t, factory()) })
	t.Run("Hierarchy", func(t *testing.T) { testHierarchy(t, factory()) })
	t.Run("CycleDetection", func(t *testing.T) { testCycleDetection(t, factory()) })
	t.Run("Comments", func(t *testing.T) { testComments(t, factory()) })
}

func testCreate(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	issue := &Issue{
		Title:       "Test Issue",
		Description: "Test description",
		Status:      StatusOpen,
		Priority:    PriorityMedium,
		Type:        TypeTask,
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
		t.Errorf("Priority mismatch: got %q, want %q", got.Priority, issue.Priority)
	}
	if got.Type != issue.Type {
		t.Errorf("Type mismatch: got %q, want %q", got.Type, issue.Type)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func testGet(t *testing.T, s Storage) {
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
	issue := &Issue{
		Title:    "Get Test",
		Status:   StatusOpen,
		Priority: PriorityLow,
		Type:     TypeBug,
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

func testUpdate(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Update non-existent issue should return ErrNotFound
	err := s.Update(ctx, &Issue{ID: "nonexistent-id", Title: "test"})
	if err != ErrNotFound {
		t.Errorf("Update non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue
	issue := &Issue{
		Title:       "Original Title",
		Description: "Original description",
		Status:      StatusOpen,
		Priority:    PriorityLow,
		Type:        TypeTask,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update it
	updated := &Issue{
		ID:          id,
		Title:       "Updated Title",
		Description: "Updated description",
		Status:      StatusInProgress,
		Priority:    PriorityHigh,
		Type:        TypeTask,
		Labels:      []string{"urgent"},
		Assignee:    "alice",
	}
	if err := s.Update(ctx, updated); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify the update
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Update failed: %v", err)
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
		t.Errorf("Priority mismatch: got %q, want %q", got.Priority, updated.Priority)
	}
	if got.Assignee != updated.Assignee {
		t.Errorf("Assignee mismatch: got %q, want %q", got.Assignee, updated.Assignee)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "urgent" {
		t.Errorf("Labels mismatch: got %v, want [urgent]", got.Labels)
	}
}

func testDelete(t *testing.T, s Storage) {
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
	issue := &Issue{
		Title:    "To Delete",
		Status:   StatusOpen,
		Priority: PriorityLow,
		Type:     TypeChore,
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

func testList(t *testing.T, s Storage) {
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
			Title:    spec.title,
			Status:   spec.status,
			Priority: spec.priority,
			Type:     spec.typ,
			Labels:   spec.labels,
		}
		id, err := s.Create(ctx, issue)
		if err != nil {
			t.Fatalf("Create issue %d failed: %v", i, err)
		}
		ids = append(ids, id)
	}

	// List with nil filter should return open issues (default behavior)
	issues, err = s.List(ctx, nil)
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

func testClose(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Close non-existent issue should return ErrNotFound
	err := s.Close(ctx, "nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("Close non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue
	issue := &Issue{
		Title:    "To Close",
		Status:   StatusOpen,
		Priority: PriorityLow,
		Type:     TypeTask,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Close it
	if err := s.Close(ctx, id); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify status changed
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Close failed: %v", err)
	}
	if got.Status != StatusClosed {
		t.Errorf("Status after Close: got %q, want %q", got.Status, StatusClosed)
	}
	if got.ClosedAt == nil {
		t.Error("ClosedAt should be set after Close")
	}

	// Reopen it
	if err := s.Reopen(ctx, id); err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}

	// Verify status changed back
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

func testDependencies(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues
	issueA := &Issue{Title: "Issue A", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}
	issueB := &Issue{Title: "Issue B", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}

	idA, err := s.Create(ctx, issueA)
	if err != nil {
		t.Fatalf("Create A failed: %v", err)
	}
	idB, err := s.Create(ctx, issueB)
	if err != nil {
		t.Fatalf("Create B failed: %v", err)
	}

	// Add dependency: A depends on B
	if err := s.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Verify both sides were updated
	gotA, err := s.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A failed: %v", err)
	}
	if !contains(gotA.DependsOn, idB) {
		t.Errorf("A.DependsOn should contain B; got %v", gotA.DependsOn)
	}

	gotB, err := s.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B failed: %v", err)
	}
	if !contains(gotB.Dependents, idA) {
		t.Errorf("B.Dependents should contain A; got %v", gotB.Dependents)
	}

	// Remove dependency
	if err := s.RemoveDependency(ctx, idA, idB); err != nil {
		t.Fatalf("RemoveDependency failed: %v", err)
	}

	// Verify both sides were updated
	gotA, err = s.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A after remove failed: %v", err)
	}
	if contains(gotA.DependsOn, idB) {
		t.Errorf("A.DependsOn should not contain B after remove; got %v", gotA.DependsOn)
	}

	gotB, err = s.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B after remove failed: %v", err)
	}
	if contains(gotB.Dependents, idA) {
		t.Errorf("B.Dependents should not contain A after remove; got %v", gotB.Dependents)
	}
}

func testBlocking(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues
	issueA := &Issue{Title: "Blocker", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}
	issueB := &Issue{Title: "Blocked", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}

	idA, err := s.Create(ctx, issueA)
	if err != nil {
		t.Fatalf("Create A failed: %v", err)
	}
	idB, err := s.Create(ctx, issueB)
	if err != nil {
		t.Fatalf("Create B failed: %v", err)
	}

	// Add block: A blocks B
	if err := s.AddBlock(ctx, idA, idB); err != nil {
		t.Fatalf("AddBlock failed: %v", err)
	}

	// Verify both sides were updated
	gotA, err := s.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A failed: %v", err)
	}
	if !contains(gotA.Blocks, idB) {
		t.Errorf("A.Blocks should contain B; got %v", gotA.Blocks)
	}

	gotB, err := s.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B failed: %v", err)
	}
	if !contains(gotB.BlockedBy, idA) {
		t.Errorf("B.BlockedBy should contain A; got %v", gotB.BlockedBy)
	}

	// Remove block
	if err := s.RemoveBlock(ctx, idA, idB); err != nil {
		t.Fatalf("RemoveBlock failed: %v", err)
	}

	// Verify both sides were updated
	gotA, err = s.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A after remove failed: %v", err)
	}
	if contains(gotA.Blocks, idB) {
		t.Errorf("A.Blocks should not contain B after remove; got %v", gotA.Blocks)
	}

	gotB, err = s.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B after remove failed: %v", err)
	}
	if contains(gotB.BlockedBy, idA) {
		t.Errorf("B.BlockedBy should not contain A after remove; got %v", gotB.BlockedBy)
	}
}

func testHierarchy(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create parent and child
	parent := &Issue{Title: "Parent", Status: StatusOpen, Priority: PriorityMedium, Type: TypeEpic}
	child := &Issue{Title: "Child", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}

	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}
	childID, err := s.Create(ctx, child)
	if err != nil {
		t.Fatalf("Create child failed: %v", err)
	}

	// Set parent
	if err := s.SetParent(ctx, childID, parentID); err != nil {
		t.Fatalf("SetParent failed: %v", err)
	}

	// Verify both sides
	gotChild, err := s.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child failed: %v", err)
	}
	if gotChild.Parent != parentID {
		t.Errorf("Child.Parent: got %q, want %q", gotChild.Parent, parentID)
	}

	gotParent, err := s.Get(ctx, parentID)
	if err != nil {
		t.Fatalf("Get parent failed: %v", err)
	}
	if !contains(gotParent.Children, childID) {
		t.Errorf("Parent.Children should contain child; got %v", gotParent.Children)
	}

	// Create another parent and re-parent the child
	newParent := &Issue{Title: "New Parent", Status: StatusOpen, Priority: PriorityMedium, Type: TypeEpic}
	newParentID, err := s.Create(ctx, newParent)
	if err != nil {
		t.Fatalf("Create new parent failed: %v", err)
	}

	if err := s.SetParent(ctx, childID, newParentID); err != nil {
		t.Fatalf("SetParent (reparent) failed: %v", err)
	}

	// Verify old parent no longer has child
	gotOldParent, err := s.Get(ctx, parentID)
	if err != nil {
		t.Fatalf("Get old parent failed: %v", err)
	}
	if contains(gotOldParent.Children, childID) {
		t.Errorf("Old parent should not have child; got %v", gotOldParent.Children)
	}

	// Verify new parent has child
	gotNewParent, err := s.Get(ctx, newParentID)
	if err != nil {
		t.Fatalf("Get new parent failed: %v", err)
	}
	if !contains(gotNewParent.Children, childID) {
		t.Errorf("New parent should have child; got %v", gotNewParent.Children)
	}

	// Verify child points to new parent
	gotChild, err = s.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child after reparent failed: %v", err)
	}
	if gotChild.Parent != newParentID {
		t.Errorf("Child.Parent after reparent: got %q, want %q", gotChild.Parent, newParentID)
	}

	// Remove parent
	if err := s.RemoveParent(ctx, childID); err != nil {
		t.Fatalf("RemoveParent failed: %v", err)
	}

	// Verify child has no parent
	gotChild, err = s.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child after RemoveParent failed: %v", err)
	}
	if gotChild.Parent != "" {
		t.Errorf("Child.Parent after RemoveParent: got %q, want empty", gotChild.Parent)
	}

	// Verify new parent no longer has child
	gotNewParent, err = s.Get(ctx, newParentID)
	if err != nil {
		t.Fatalf("Get new parent after RemoveParent failed: %v", err)
	}
	if contains(gotNewParent.Children, childID) {
		t.Errorf("New parent should not have child after RemoveParent; got %v", gotNewParent.Children)
	}
}

func testCycleDetection(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create three issues for cycle testing
	issueA := &Issue{Title: "A", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}
	issueB := &Issue{Title: "B", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}
	issueC := &Issue{Title: "C", Status: StatusOpen, Priority: PriorityMedium, Type: TypeTask}

	idA, err := s.Create(ctx, issueA)
	if err != nil {
		t.Fatalf("Create A failed: %v", err)
	}
	idB, err := s.Create(ctx, issueB)
	if err != nil {
		t.Fatalf("Create B failed: %v", err)
	}
	idC, err := s.Create(ctx, issueC)
	if err != nil {
		t.Fatalf("Create C failed: %v", err)
	}

	// Test self-dependency cycle
	err = s.AddDependency(ctx, idA, idA)
	if err != ErrCycle {
		t.Errorf("Self-dependency should return ErrCycle; got %v", err)
	}

	// Test direct cycle: A depends on B, then B depends on A
	if err := s.AddDependency(ctx, idA, idB); err != nil {
		t.Fatalf("AddDependency A->B failed: %v", err)
	}
	err = s.AddDependency(ctx, idB, idA)
	if err != ErrCycle {
		t.Errorf("Direct cycle B->A should return ErrCycle; got %v", err)
	}

	// Test transitive cycle: A->B, B->C, then C->A
	if err := s.AddDependency(ctx, idB, idC); err != nil {
		t.Fatalf("AddDependency B->C failed: %v", err)
	}
	err = s.AddDependency(ctx, idC, idA)
	if err != ErrCycle {
		t.Errorf("Transitive cycle C->A should return ErrCycle; got %v", err)
	}

	// Test hierarchy cycle: make A parent of B, then try to make B parent of A
	// First clear the dependency relationships
	if err := s.RemoveDependency(ctx, idA, idB); err != nil {
		t.Fatalf("RemoveDependency A->B failed: %v", err)
	}
	if err := s.RemoveDependency(ctx, idB, idC); err != nil {
		t.Fatalf("RemoveDependency B->C failed: %v", err)
	}

	if err := s.SetParent(ctx, idB, idA); err != nil {
		t.Fatalf("SetParent B->A failed: %v", err)
	}
	err = s.SetParent(ctx, idA, idB)
	if err != ErrCycle {
		t.Errorf("Hierarchy cycle should return ErrCycle; got %v", err)
	}

	// Test deeper hierarchy cycle: A is ancestor of C via B, try to make C parent of A
	if err := s.SetParent(ctx, idC, idB); err != nil {
		t.Fatalf("SetParent C->B failed: %v", err)
	}
	err = s.SetParent(ctx, idA, idC)
	if err != ErrCycle {
		t.Errorf("Deep hierarchy cycle should return ErrCycle; got %v", err)
	}
}

func testComments(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue
	issue := &Issue{
		Title:    "Issue with Comments",
		Status:   StatusOpen,
		Priority: PriorityMedium,
		Type:     TypeTask,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Add a comment
	comment1 := &Comment{
		Author:    "alice",
		Body:      "First comment",
		CreatedAt: time.Now(),
	}
	if err := s.AddComment(ctx, id, comment1); err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}

	// Verify comment was added
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after AddComment failed: %v", err)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("Expected 1 comment, got %d", len(got.Comments))
	}
	if got.Comments[0].Author != "alice" {
		t.Errorf("Comment author: got %q, want %q", got.Comments[0].Author, "alice")
	}
	if got.Comments[0].Body != "First comment" {
		t.Errorf("Comment body: got %q, want %q", got.Comments[0].Body, "First comment")
	}
	if got.Comments[0].ID == "" {
		t.Error("Comment ID should be set")
	}

	// Add another comment
	comment2 := &Comment{
		Author:    "bob",
		Body:      "Second comment",
		CreatedAt: time.Now(),
	}
	if err := s.AddComment(ctx, id, comment2); err != nil {
		t.Fatalf("AddComment 2 failed: %v", err)
	}

	// Verify both comments exist
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after second AddComment failed: %v", err)
	}
	if len(got.Comments) != 2 {
		t.Fatalf("Expected 2 comments, got %d", len(got.Comments))
	}

	// AddComment on non-existent issue should return ErrNotFound
	err = s.AddComment(ctx, "nonexistent-id", comment1)
	if err != ErrNotFound {
		t.Errorf("AddComment on non-existent issue: got %v, want ErrNotFound", err)
	}
}

// contains checks if a slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
