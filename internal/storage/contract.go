package storage

import (
	"context"
	"errors"
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
	t.Run("Hierarchy", func(t *testing.T) { testHierarchy(t, factory()) })
	t.Run("CycleDetection", func(t *testing.T) { testCycleDetection(t, factory()) })
	t.Run("Comments", func(t *testing.T) { testComments(t, factory()) })
	t.Run("ChildCounters", func(t *testing.T) { testChildCounters(t, factory()) })
	t.Run("HierarchyDepthLimit", func(t *testing.T) { testHierarchyDepthLimit(t, factory()) })
	t.Run("CreateTombstone", func(t *testing.T) { testCreateTombstone(t, factory()) })
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

	// Add dependency: A depends on B (type: blocks)
	if err := s.AddDependency(ctx, idA, idB, DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Verify both sides were updated
	gotA, err := s.Get(ctx, idA)
	if err != nil {
		t.Fatalf("Get A failed: %v", err)
	}
	if !gotA.HasDependency(idB) {
		t.Errorf("A.Dependencies should contain B; got %v", gotA.Dependencies)
	}

	gotB, err := s.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B failed: %v", err)
	}
	if !gotB.HasDependent(idA) {
		t.Errorf("B.Dependents should contain A; got %v", gotB.Dependents)
	}

	// Verify dependency type
	for _, dep := range gotA.Dependencies {
		if dep.ID == idB && dep.Type != DepTypeBlocks {
			t.Errorf("expected dependency type %q, got %q", DepTypeBlocks, dep.Type)
		}
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
	if gotA.HasDependency(idB) {
		t.Errorf("A.Dependencies should not contain B after remove; got %v", gotA.Dependencies)
	}

	gotB, err = s.Get(ctx, idB)
	if err != nil {
		t.Fatalf("Get B after remove failed: %v", err)
	}
	if gotB.HasDependent(idA) {
		t.Errorf("B.Dependents should not contain A after remove; got %v", gotB.Dependents)
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

	// Set parent via AddDependency with parent-child type
	if err := s.AddDependency(ctx, childID, parentID, DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency (parent-child) failed: %v", err)
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
	if !containsStr(gotParent.Children(), childID) {
		t.Errorf("Parent.Children should contain child; got %v", gotParent.Children())
	}

	// Verify parent-child typed dependency exists
	if !gotChild.HasDependency(parentID) {
		t.Errorf("Child should have parent-child dependency on parent; got %v", gotChild.Dependencies)
	}
	if !gotParent.HasDependent(childID) {
		t.Errorf("Parent should have child as dependent; got %v", gotParent.Dependents)
	}

	// Create another parent and re-parent the child
	newParent := &Issue{Title: "New Parent", Status: StatusOpen, Priority: PriorityMedium, Type: TypeEpic}
	newParentID, err := s.Create(ctx, newParent)
	if err != nil {
		t.Fatalf("Create new parent failed: %v", err)
	}

	if err := s.AddDependency(ctx, childID, newParentID, DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency (reparent) failed: %v", err)
	}

	// Verify old parent no longer has child
	gotOldParent, err := s.Get(ctx, parentID)
	if err != nil {
		t.Fatalf("Get old parent failed: %v", err)
	}
	if containsStr(gotOldParent.Children(), childID) {
		t.Errorf("Old parent should not have child; got %v", gotOldParent.Children())
	}

	// Verify new parent has child
	gotNewParent, err := s.Get(ctx, newParentID)
	if err != nil {
		t.Fatalf("Get new parent failed: %v", err)
	}
	if !containsStr(gotNewParent.Children(), childID) {
		t.Errorf("New parent should have child; got %v", gotNewParent.Children())
	}

	// Verify child points to new parent
	gotChild, err = s.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child after reparent failed: %v", err)
	}
	if gotChild.Parent != newParentID {
		t.Errorf("Child.Parent after reparent: got %q, want %q", gotChild.Parent, newParentID)
	}

	// Remove parent via RemoveDependency
	if err := s.RemoveDependency(ctx, childID, newParentID); err != nil {
		t.Fatalf("RemoveDependency (remove parent) failed: %v", err)
	}

	// Verify child has no parent
	gotChild, err = s.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child after remove parent failed: %v", err)
	}
	if gotChild.Parent != "" {
		t.Errorf("Child.Parent after remove parent: got %q, want empty", gotChild.Parent)
	}

	// Verify new parent no longer has child
	gotNewParent, err = s.Get(ctx, newParentID)
	if err != nil {
		t.Fatalf("Get new parent after remove parent failed: %v", err)
	}
	if containsStr(gotNewParent.Children(), childID) {
		t.Errorf("New parent should not have child after remove parent; got %v", gotNewParent.Children())
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
	err = s.AddDependency(ctx, idA, idA, DepTypeBlocks)
	if err != ErrCycle {
		t.Errorf("Self-dependency should return ErrCycle; got %v", err)
	}

	// Test direct cycle: A depends on B, then B depends on A
	if err := s.AddDependency(ctx, idA, idB, DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency A->B failed: %v", err)
	}
	err = s.AddDependency(ctx, idB, idA, DepTypeBlocks)
	if err != ErrCycle {
		t.Errorf("Direct cycle B->A should return ErrCycle; got %v", err)
	}

	// Test transitive cycle: A->B, B->C, then C->A
	if err := s.AddDependency(ctx, idB, idC, DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency B->C failed: %v", err)
	}
	err = s.AddDependency(ctx, idC, idA, DepTypeBlocks)
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

	if err := s.AddDependency(ctx, idB, idA, DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency (parent-child) B->A failed: %v", err)
	}
	err = s.AddDependency(ctx, idA, idB, DepTypeParentChild)
	if err != ErrCycle {
		t.Errorf("Hierarchy cycle should return ErrCycle; got %v", err)
	}

	// Test deeper hierarchy cycle: A is ancestor of C via B, try to make C parent of A
	if err := s.AddDependency(ctx, idC, idB, DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency (parent-child) C->B failed: %v", err)
	}
	err = s.AddDependency(ctx, idA, idC, DepTypeParentChild)
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
		Text:      "First comment",
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
	if got.Comments[0].Text != "First comment" {
		t.Errorf("Comment body: got %q, want %q", got.Comments[0].Text, "First comment")
	}
	if got.Comments[0].ID == 0 {
		t.Error("Comment ID should be set")
	}

	// Add another comment
	comment2 := &Comment{
		Author:    "bob",
		Text:      "Second comment",
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

func testChildCounters(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create parent issues so GetNextChildID can validate them
	parentA := &Issue{Title: "Parent A"}
	idA, err := s.Create(ctx, parentA)
	if err != nil {
		t.Fatalf("Create parent A failed: %v", err)
	}

	parentB := &Issue{Title: "Parent B"}
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

	// Second child should return parentID.2
	childID, err = s.GetNextChildID(ctx, idA)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	wantSecond := idA + ".2"
	if childID != wantSecond {
		t.Errorf("Second child ID: got %q, want %q", childID, wantSecond)
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

func testHierarchyDepthLimit(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a root issue
	root := &Issue{Title: "Root"}
	rootID, err := s.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create root failed: %v", err)
	}

	// Build chain: root -> depth 1 -> depth 2 -> depth 3
	depth1ID, err := s.GetNextChildID(ctx, rootID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 1 failed: %v", err)
	}
	s.Create(ctx, &Issue{ID: depth1ID, Title: "Depth 1"})

	depth2ID, err := s.GetNextChildID(ctx, depth1ID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 2 failed: %v", err)
	}
	s.Create(ctx, &Issue{ID: depth2ID, Title: "Depth 2"})

	depth3ID, err := s.GetNextChildID(ctx, depth2ID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 3 failed: %v", err)
	}
	s.Create(ctx, &Issue{ID: depth3ID, Title: "Depth 3"})

	// Depth 4 should be rejected
	_, err = s.GetNextChildID(ctx, depth3ID)
	if !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("GetNextChildID at depth 4: got %v, want ErrMaxDepthExceeded", err)
	}
}

func testCreateTombstone(t *testing.T, s Storage) {
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue to tombstone
	issue := &Issue{
		Title:    "To Tombstone",
		Status:   StatusOpen,
		Priority: PriorityMedium,
		Type:     TypeTask,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Tombstone it
	if err := s.CreateTombstone(ctx, id, "test-actor", "no longer needed"); err != nil {
		t.Fatalf("CreateTombstone failed: %v", err)
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

	// Tombstone a non-existent issue should return ErrNotFound
	err = s.CreateTombstone(ctx, "nonexistent-id", "actor", "reason")
	if err != ErrNotFound {
		t.Errorf("CreateTombstone non-existent: got %v, want ErrNotFound", err)
	}

	// Tombstone an already-tombstoned issue should return ErrAlreadyTombstoned
	err = s.CreateTombstone(ctx, id, "actor", "reason")
	if !errors.Is(err, ErrAlreadyTombstoned) {
		t.Errorf("Double tombstone: got %v, want ErrAlreadyTombstoned", err)
	}

	// Hard Delete() should still work on tombstoned issues
	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete tombstoned issue failed: %v", err)
	}
	_, err = s.Get(ctx, id)
	if err != ErrNotFound {
		t.Errorf("Get after hard delete of tombstone: got %v, want ErrNotFound", err)
	}

	// Test tombstoning a closed issue
	closedIssue := &Issue{
		Title:    "Closed then tombstoned",
		Status:   StatusOpen,
		Priority: PriorityLow,
		Type:     TypeBug,
	}
	closedID, err := s.Create(ctx, closedIssue)
	if err != nil {
		t.Fatalf("Create closed issue failed: %v", err)
	}
	if err := s.Close(ctx, closedID); err != nil {
		t.Fatalf("Close issue failed: %v", err)
	}
	if err := s.CreateTombstone(ctx, closedID, "actor", "obsolete"); err != nil {
		t.Fatalf("CreateTombstone on closed issue failed: %v", err)
	}
	gotClosed, err := s.Get(ctx, closedID)
	if err != nil {
		t.Fatalf("Get closed-then-tombstoned issue failed: %v", err)
	}
	if gotClosed.Status != StatusTombstone {
		t.Errorf("Closed-then-tombstoned status: got %q, want %q", gotClosed.Status, StatusTombstone)
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
