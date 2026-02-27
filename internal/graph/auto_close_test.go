package graph

import (
	"context"
	"errors"
	"testing"

	"beads-lite/internal/issuestorage"
)

type failGetStore struct {
	issuestorage.IssueStore
	failID string
}

func (s *failGetStore) Get(ctx context.Context, id string) (*issuestorage.Issue, error) {
	if id == s.failID {
		return nil, errors.New("injected get failure")
	}
	return s.IssueStore.Get(ctx, id)
}

func closeIssue(t *testing.T, ctx context.Context, s issuestorage.IssueStore, id string) {
	t.Helper()
	if err := s.Modify(ctx, id, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("close %s: %v", id, err)
	}
}

func TestAutoCloseAncestors_LastChildClosesParent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	parent := createIssue(t, ctx, s, "Parent", issuestorage.TypeEpic)
	childA := createIssue(t, ctx, s, "Child A", issuestorage.TypeTask)
	childB := createIssue(t, ctx, s, "Child B", issuestorage.TypeTask)

	for _, childID := range []string{childA.ID, childB.ID} {
		if err := s.AddDependency(ctx, childID, parent.ID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency parent-child %s->%s: %v", childID, parent.ID, err)
		}
	}

	closeIssue(t, ctx, s, childA.ID)
	closeIssue(t, ctx, s, childB.ID)

	closed, err := AutoCloseAncestors(ctx, s, childB.ID, true)
	if err != nil {
		t.Fatalf("AutoCloseAncestors: %v", err)
	}
	if len(closed) != 1 || closed[0] != parent.ID {
		t.Fatalf("closed ancestors = %v, want [%s]", closed, parent.ID)
	}

	gotParent, err := s.Get(ctx, parent.ID)
	if err != nil {
		t.Fatalf("Get parent: %v", err)
	}
	if gotParent.Status != issuestorage.StatusClosed {
		t.Fatalf("parent status = %s, want %s", gotParent.Status, issuestorage.StatusClosed)
	}
	if gotParent.CloseReason != autoCloseReason {
		t.Fatalf("parent close reason = %q, want %q", gotParent.CloseReason, autoCloseReason)
	}
}

func TestAutoCloseAncestors_MultiLevel(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	grandparent := createIssue(t, ctx, s, "Grandparent", issuestorage.TypeEpic)
	parent := createIssue(t, ctx, s, "Parent", issuestorage.TypeTask)
	child := createIssue(t, ctx, s, "Child", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, parent.ID, grandparent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency parent->grandparent: %v", err)
	}
	if err := s.AddDependency(ctx, child.ID, parent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency child->parent: %v", err)
	}

	closeIssue(t, ctx, s, child.ID)

	closed, err := AutoCloseAncestors(ctx, s, child.ID, true)
	if err != nil {
		t.Fatalf("AutoCloseAncestors: %v", err)
	}
	if len(closed) != 2 || closed[0] != parent.ID || closed[1] != grandparent.ID {
		t.Fatalf("closed ancestors = %v, want [%s %s]", closed, parent.ID, grandparent.ID)
	}
}

func TestAutoCloseAncestors_AlreadyClosedAncestorContinuesUp(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	grandparent := createIssue(t, ctx, s, "Grandparent", issuestorage.TypeEpic)
	parent := createIssue(t, ctx, s, "Parent", issuestorage.TypeTask)
	child := createIssue(t, ctx, s, "Child", issuestorage.TypeTask)

	if err := s.AddDependency(ctx, parent.ID, grandparent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency parent->grandparent: %v", err)
	}
	if err := s.AddDependency(ctx, child.ID, parent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency child->parent: %v", err)
	}

	closeIssue(t, ctx, s, parent.ID)
	closeIssue(t, ctx, s, child.ID)

	closed, err := AutoCloseAncestors(ctx, s, child.ID, true)
	if err != nil {
		t.Fatalf("AutoCloseAncestors: %v", err)
	}
	if len(closed) != 1 || closed[0] != grandparent.ID {
		t.Fatalf("closed ancestors = %v, want [%s]", closed, grandparent.ID)
	}
}

func TestAutoCloseAncestors_SkipsGateAndMolecule(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		middleType issuestorage.IssueType
	}{
		{name: "gate", middleType: issuestorage.TypeGate},
		{name: "molecule", middleType: issuestorage.TypeMolecule},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore(t)

			grandparent := createIssue(t, ctx, s, "Grandparent", issuestorage.TypeEpic)
			middle := createIssue(t, ctx, s, "Middle", tt.middleType)
			child := createIssue(t, ctx, s, "Child", issuestorage.TypeTask)

			if err := s.AddDependency(ctx, middle.ID, grandparent.ID, issuestorage.DepTypeParentChild); err != nil {
				t.Fatalf("AddDependency middle->grandparent: %v", err)
			}
			if err := s.AddDependency(ctx, child.ID, middle.ID, issuestorage.DepTypeParentChild); err != nil {
				t.Fatalf("AddDependency child->middle: %v", err)
			}

			closeIssue(t, ctx, s, middle.ID)
			closeIssue(t, ctx, s, child.ID)

			closed, err := AutoCloseAncestors(ctx, s, child.ID, true)
			if err != nil {
				t.Fatalf("AutoCloseAncestors: %v", err)
			}
			if len(closed) != 1 || closed[0] != grandparent.ID {
				t.Fatalf("closed ancestors = %v, want [%s]", closed, grandparent.ID)
			}

			gotMiddle, err := s.Get(ctx, middle.ID)
			if err != nil {
				t.Fatalf("Get middle: %v", err)
			}
			if gotMiddle.CloseReason == autoCloseReason {
				t.Fatalf("excluded type %s should not be auto-closed", tt.middleType)
			}
		})
	}
}

func TestAutoCloseAncestors_GetErrorMidChainReturnsPartial(t *testing.T) {
	ctx := context.Background()
	base := newStore(t)

	grandparent := createIssue(t, ctx, base, "Grandparent", issuestorage.TypeEpic)
	parent := createIssue(t, ctx, base, "Parent", issuestorage.TypeTask)
	child := createIssue(t, ctx, base, "Child", issuestorage.TypeTask)

	if err := base.AddDependency(ctx, parent.ID, grandparent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency parent->grandparent: %v", err)
	}
	if err := base.AddDependency(ctx, child.ID, parent.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency child->parent: %v", err)
	}

	closeIssue(t, ctx, base, child.ID)

	s := &failGetStore{IssueStore: base, failID: grandparent.ID}
	closed, err := AutoCloseAncestors(ctx, s, child.ID, true)
	if err != nil {
		t.Fatalf("AutoCloseAncestors: %v", err)
	}
	if len(closed) != 1 || closed[0] != parent.ID {
		t.Fatalf("closed ancestors = %v, want [%s]", closed, parent.ID)
	}

	gotGrandparent, err := base.Get(ctx, grandparent.ID)
	if err != nil {
		t.Fatalf("Get grandparent: %v", err)
	}
	if gotGrandparent.Status == issuestorage.StatusClosed {
		t.Fatalf("grandparent should remain open after injected mid-chain failure")
	}
}
