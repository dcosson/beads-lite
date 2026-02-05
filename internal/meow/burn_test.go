package meow

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	"beads-lite/internal/issueservice"
)

func newBurnStore(t *testing.T) *issueservice.IssueStore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return issueservice.New(nil, s)
}

func createBurnIssue(t *testing.T, ctx context.Context, s issuestorage.IssueStore, title string, ephemeral bool) *issuestorage.Issue {
	t.Helper()
	issue := &issuestorage.Issue{
		Title:     title,
		Status:    issuestorage.StatusOpen,
		Priority:  issuestorage.PriorityMedium,
		Type:      issuestorage.TypeTask,
		Ephemeral: ephemeral,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create %q: %v", title, err)
	}
	issue.ID = id
	return issue
}

func TestBurn_PersistentMolecule(t *testing.T) {
	ctx := context.Background()
	s := newBurnStore(t)

	// Create root epic + persistent children.
	root := createBurnIssue(t, ctx, s, "Root", false)
	if err := s.Modify(ctx, root.ID, func(i *issuestorage.Issue) error {
		i.Type = issuestorage.TypeEpic
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	childA := createBurnIssue(t, ctx, s, "A", false)
	childB := createBurnIssue(t, ctx, s, "B", false)
	for _, child := range []*issuestorage.Issue{childA, childB} {
		if err := s.AddDependency(ctx, child.ID, root.ID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}

	if _, err := Burn(ctx, s, root.ID); err != nil {
		t.Fatalf("Burn: %v", err)
	}

	// Persistent issues should be closed (tombstones created).
	for _, id := range []string{root.ID, childA.ID, childB.ID} {
		issue, err := s.Get(ctx, id)
		if err != nil {
			t.Errorf("tombstone %s not found: %v", id, err)
			continue
		}
		if issue.Status != issuestorage.StatusClosed {
			t.Errorf("tombstone %s: got status %s, want closed", id, issue.Status)
		}
	}
}

func TestBurn_EphemeralWisp(t *testing.T) {
	ctx := context.Background()
	s := newBurnStore(t)

	root := createBurnIssue(t, ctx, s, "Wisp Root", true)
	childA := createBurnIssue(t, ctx, s, "Wisp A", true)
	childB := createBurnIssue(t, ctx, s, "Wisp B", true)
	for _, child := range []*issuestorage.Issue{childA, childB} {
		if err := s.AddDependency(ctx, child.ID, root.ID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}

	if _, err := Burn(ctx, s, root.ID); err != nil {
		t.Fatalf("Burn: %v", err)
	}

	// Ephemeral issues should be completely gone — no tombstones.
	for _, id := range []string{root.ID, childA.ID, childB.ID} {
		_, err := s.Get(ctx, id)
		if !errors.Is(err, issuestorage.ErrNotFound) {
			t.Errorf("ephemeral %s should be deleted, got err: %v", id, err)
		}
	}
}

func TestBurn_WithExternalDeps(t *testing.T) {
	ctx := context.Background()
	s := newBurnStore(t)

	// Create molecule.
	root := createBurnIssue(t, ctx, s, "Root", false)
	child := createBurnIssue(t, ctx, s, "Child", false)
	if err := s.AddDependency(ctx, child.ID, root.ID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatal(err)
	}

	// External issue that blocks the child (child depends on external).
	blocker := createBurnIssue(t, ctx, s, "External Blocker", false)
	if err := s.AddDependency(ctx, child.ID, blocker.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	// External issue that the child blocks (downstream depends on child).
	downstream := createBurnIssue(t, ctx, s, "External Downstream", false)
	if err := s.AddDependency(ctx, downstream.ID, child.ID, issuestorage.DepTypeBlocks); err != nil {
		t.Fatal(err)
	}

	if _, err := Burn(ctx, s, root.ID); err != nil {
		t.Fatalf("Burn: %v", err)
	}

	// External blocker should no longer list the burned child as a dependent.
	b, err := s.Get(ctx, blocker.ID)
	if err != nil {
		t.Fatalf("Get blocker: %v", err)
	}
	if b.HasDependent(child.ID) {
		t.Error("blocker still lists burned child as dependent")
	}

	// External downstream should no longer depend on the burned child.
	d, err := s.Get(ctx, downstream.ID)
	if err != nil {
		t.Fatalf("Get downstream: %v", err)
	}
	if d.HasDependency(child.ID) {
		t.Error("downstream still depends on burned child")
	}
}

func TestBurn_MixedEphemeralPersistent(t *testing.T) {
	ctx := context.Background()
	s := newBurnStore(t)

	// Persistent root with a mix of ephemeral and persistent children.
	root := createBurnIssue(t, ctx, s, "Root", false)
	ephChild := createBurnIssue(t, ctx, s, "Ephemeral Child", true)
	persChild := createBurnIssue(t, ctx, s, "Persistent Child", false)

	for _, child := range []*issuestorage.Issue{ephChild, persChild} {
		if err := s.AddDependency(ctx, child.ID, root.ID, issuestorage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}

	if _, err := Burn(ctx, s, root.ID); err != nil {
		t.Fatalf("Burn: %v", err)
	}

	// Ephemeral child: gone entirely.
	_, err := s.Get(ctx, ephChild.ID)
	if !errors.Is(err, issuestorage.ErrNotFound) {
		t.Errorf("ephemeral child should be deleted, got err: %v", err)
	}

	// Persistent child: closed (tombstone).
	issue, err := s.Get(ctx, persChild.ID)
	if err != nil {
		t.Fatalf("persistent child tombstone not found: %v", err)
	}
	if issue.Status != issuestorage.StatusClosed {
		t.Errorf("persistent child: got status %s, want closed", issue.Status)
	}

	// Persistent root: closed (tombstone).
	issue, err = s.Get(ctx, root.ID)
	if err != nil {
		t.Fatalf("root tombstone not found: %v", err)
	}
	if issue.Status != issuestorage.StatusClosed {
		t.Errorf("root: got status %s, want closed", issue.Status)
	}
}

func TestBurn_NonExistent(t *testing.T) {
	ctx := context.Background()
	s := newBurnStore(t)

	_, err := Burn(ctx, s, "nonexistent-mol")
	if err == nil {
		t.Fatal("expected error for non-existent molecule")
	}
	if !errors.Is(err, issuestorage.ErrNotFound) {
		t.Errorf("expected ErrNotFound in chain, got: %v", err)
	}
}
