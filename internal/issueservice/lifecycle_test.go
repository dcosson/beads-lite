package issueservice

import (
	"context"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func newTestIssueService(t *testing.T) *IssueStore {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	return New(nil, store)
}

func TestModifyCloseAutoClosesAncestors(t *testing.T) {
	ctx := context.Background()
	s := newTestIssueService(t)

	grandparentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Grandparent", Type: issuestorage.TypeEpic})
	parentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeTask})
	childID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask})
	if err := s.AddDependency(ctx, parentID, grandparentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent->grandparent: %v", err)
	}
	if err := s.AddDependency(ctx, childID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add child->parent: %v", err)
	}

	if err := s.Modify(ctx, childID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("close child: %v", err)
	}

	parent, _ := s.Get(ctx, parentID)
	grandparent, _ := s.Get(ctx, grandparentID)
	if parent.Status != issuestorage.StatusClosed {
		t.Fatalf("parent status = %s, want closed", parent.Status)
	}
	if grandparent.Status != issuestorage.StatusClosed {
		t.Fatalf("grandparent status = %s, want closed", grandparent.Status)
	}
}

func TestModifyReopenAutoReopensAncestors(t *testing.T) {
	ctx := context.Background()
	s := newTestIssueService(t)

	grandparentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Grandparent", Type: issuestorage.TypeEpic})
	parentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeTask})
	childID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask})
	if err := s.AddDependency(ctx, parentID, grandparentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent->grandparent: %v", err)
	}
	if err := s.AddDependency(ctx, childID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add child->parent: %v", err)
	}

	for _, id := range []string{grandparentID, parentID, childID} {
		if err := s.Modify(ctx, id, func(i *issuestorage.Issue) error {
			i.Status = issuestorage.StatusClosed
			return nil
		}); err != nil {
			t.Fatalf("close %s: %v", id, err)
		}
	}

	if err := s.Modify(ctx, childID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusOpen
		return nil
	}); err != nil {
		t.Fatalf("reopen child: %v", err)
	}

	parent, _ := s.Get(ctx, parentID)
	grandparent, _ := s.Get(ctx, grandparentID)
	if parent.Status != issuestorage.StatusOpen {
		t.Fatalf("parent status = %s, want open", parent.Status)
	}
	if grandparent.Status != issuestorage.StatusOpen {
		t.Fatalf("grandparent status = %s, want open", grandparent.Status)
	}
}

func TestAddParentChildReopensClosedAncestorsForActiveChild(t *testing.T) {
	ctx := context.Background()
	s := newTestIssueService(t)

	grandparentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Grandparent", Type: issuestorage.TypeEpic})
	parentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeTask})
	childID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask})
	if err := s.AddDependency(ctx, parentID, grandparentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add parent->grandparent: %v", err)
	}
	for _, id := range []string{grandparentID, parentID} {
		if err := s.Modify(ctx, id, func(i *issuestorage.Issue) error {
			i.Status = issuestorage.StatusClosed
			return nil
		}); err != nil {
			t.Fatalf("close %s: %v", id, err)
		}
	}

	if err := s.AddDependency(ctx, childID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add child->parent: %v", err)
	}

	parent, _ := s.Get(ctx, parentID)
	grandparent, _ := s.Get(ctx, grandparentID)
	if parent.Status != issuestorage.StatusOpen {
		t.Fatalf("parent status = %s, want open", parent.Status)
	}
	if grandparent.Status != issuestorage.StatusOpen {
		t.Fatalf("grandparent status = %s, want open", grandparent.Status)
	}
}

func TestAutoParentLifecycleDisabled(t *testing.T) {
	ctx := context.Background()
	s := newTestIssueService(t)
	s.SetAutoCloseParent(false)

	parentID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Parent", Type: issuestorage.TypeEpic})
	childID, _ := s.Create(ctx, &issuestorage.Issue{Title: "Child", Type: issuestorage.TypeTask})
	if err := s.AddDependency(ctx, childID, parentID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("add child->parent: %v", err)
	}
	if err := s.Modify(ctx, childID, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("close child: %v", err)
	}

	parent, _ := s.Get(ctx, parentID)
	if parent.Status == issuestorage.StatusClosed {
		t.Fatalf("parent should not auto-close when disabled")
	}
}
