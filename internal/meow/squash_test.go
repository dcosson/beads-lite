package meow

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"beads-lite/internal/storage"
	"beads-lite/internal/storage/filesystem"
)

func newSquashStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	s := filesystem.New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func createSquashIssue(t *testing.T, ctx context.Context, s storage.Storage, title string, ephemeral bool) *storage.Issue {
	t.Helper()
	issue := &storage.Issue{
		Title:     title,
		Status:    storage.StatusOpen,
		Priority:  storage.PriorityMedium,
		Type:      storage.TypeTask,
		Ephemeral: ephemeral,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create %q: %v", title, err)
	}
	issue.ID = id
	return issue
}

func TestSquash_WispDigestCreatedChildrenDeleted(t *testing.T) {
	ctx := context.Background()
	s := newSquashStore(t)

	// Create wisp root with ephemeral children.
	root := createSquashIssue(t, ctx, s, "Wisp Root", true)
	childA := createSquashIssue(t, ctx, s, "Step A", true)
	childB := createSquashIssue(t, ctx, s, "Step B", true)
	for _, child := range []*storage.Issue{childA, childB} {
		if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}

	result, err := Squash(ctx, s, SquashOptions{MoleculeID: root.ID})
	if err != nil {
		t.Fatalf("Squash: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Digest should exist and be permanent + closed.
	digest, err := s.Get(ctx, result.DigestID)
	if err != nil {
		t.Fatalf("Get digest: %v", err)
	}
	if digest.Status != storage.StatusClosed {
		t.Errorf("digest status: got %s, want closed", digest.Status)
	}
	if digest.Ephemeral {
		t.Error("digest should be persistent (Ephemeral=false)")
	}
	if digest.Type != storage.TypeTask {
		t.Errorf("digest type: got %s, want task", digest.Type)
	}

	// Ephemeral children should be deleted.
	for _, id := range []string{childA.ID, childB.ID} {
		_, err := s.Get(ctx, id)
		if !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("ephemeral child %s should be deleted, got err: %v", id, err)
		}
	}

	// Result should list squashed IDs.
	if len(result.SquashedIDs) != 2 {
		t.Errorf("squashed IDs: got %d, want 2", len(result.SquashedIDs))
	}
}

func TestSquash_KeepChildren(t *testing.T) {
	ctx := context.Background()
	s := newSquashStore(t)

	root := createSquashIssue(t, ctx, s, "Root", true)
	childA := createSquashIssue(t, ctx, s, "Step A", true)
	childB := createSquashIssue(t, ctx, s, "Step B", true)
	for _, child := range []*storage.Issue{childA, childB} {
		if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}

	result, err := Squash(ctx, s, SquashOptions{
		MoleculeID:   root.ID,
		KeepChildren: true,
	})
	if err != nil {
		t.Fatalf("Squash: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.KeepChildren {
		t.Error("result.KeepChildren should be true")
	}

	// Children should still exist and be promoted to persistent.
	for _, id := range []string{childA.ID, childB.ID} {
		issue, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get promoted child %s: %v", id, err)
		}
		if issue.Ephemeral {
			t.Errorf("child %s should be promoted to persistent", id)
		}
	}

	// Digest should still be created.
	digest, err := s.Get(ctx, result.DigestID)
	if err != nil {
		t.Fatalf("Get digest: %v", err)
	}
	if digest.Status != storage.StatusClosed {
		t.Errorf("digest status: got %s, want closed", digest.Status)
	}
}

func TestSquash_NoEphemeralChildren_NoOp(t *testing.T) {
	ctx := context.Background()
	s := newSquashStore(t)

	// Persistent root with only persistent children — no ephemeral children.
	root := createSquashIssue(t, ctx, s, "Persistent Root", false)
	child := createSquashIssue(t, ctx, s, "Persistent Child", false)
	if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	result, err := Squash(ctx, s, SquashOptions{MoleculeID: root.ID})
	if err != nil {
		t.Fatalf("Squash should not error on no-op: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for no-op, got: %+v", result)
	}
}

func TestSquash_DigestFormat(t *testing.T) {
	ctx := context.Background()
	s := newSquashStore(t)

	root := createSquashIssue(t, ctx, s, "My Molecule", true)
	child := createSquashIssue(t, ctx, s, "Wisp Step", true)
	if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	result, err := Squash(ctx, s, SquashOptions{
		MoleculeID: root.ID,
		Summary:    "Custom summary text",
	})
	if err != nil {
		t.Fatalf("Squash: %v", err)
	}

	digest, err := s.Get(ctx, result.DigestID)
	if err != nil {
		t.Fatalf("Get digest: %v", err)
	}

	// Title format: "Digest: <root title>".
	if digest.Title != "Digest: My Molecule" {
		t.Errorf("title: got %q, want %q", digest.Title, "Digest: My Molecule")
	}

	// Status: closed.
	if digest.Status != storage.StatusClosed {
		t.Errorf("status: got %s, want closed", digest.Status)
	}

	// Ephemeral: false (permanent).
	if digest.Ephemeral {
		t.Error("digest should not be ephemeral")
	}

	// CloseReason.
	wantReason := "Squashed from 1 wisps"
	if digest.CloseReason != wantReason {
		t.Errorf("close reason: got %q, want %q", digest.CloseReason, wantReason)
	}

	// Custom summary used as description.
	if digest.Description != "Custom summary text" {
		t.Errorf("description: got %q, want %q", digest.Description, "Custom summary text")
	}
}

func TestSquash_AutoSummaryFromChildTitles(t *testing.T) {
	ctx := context.Background()
	s := newSquashStore(t)

	root := createSquashIssue(t, ctx, s, "Root", true)
	childA := createSquashIssue(t, ctx, s, "Alpha Step", true)
	childB := createSquashIssue(t, ctx, s, "Beta Step", true)
	for _, child := range []*storage.Issue{childA, childB} {
		if err := s.AddDependency(ctx, child.ID, root.ID, storage.DepTypeParentChild); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}

	result, err := Squash(ctx, s, SquashOptions{MoleculeID: root.ID})
	if err != nil {
		t.Fatalf("Squash: %v", err)
	}

	digest, err := s.Get(ctx, result.DigestID)
	if err != nil {
		t.Fatalf("Get digest: %v", err)
	}

	// Auto-generated summary should include child titles.
	if !strings.Contains(digest.Description, "Alpha Step") {
		t.Errorf("auto summary missing 'Alpha Step': %q", digest.Description)
	}
	if !strings.Contains(digest.Description, "Beta Step") {
		t.Errorf("auto summary missing 'Beta Step': %q", digest.Description)
	}
}
