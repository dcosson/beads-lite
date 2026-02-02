package meow

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func newGCStore(t *testing.T) issuestorage.IssueStore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beads")
	s := filesystem.New(dir, "bd-")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s
}

func createGCIssue(t *testing.T, ctx context.Context, s issuestorage.IssueStore, title string, ephemeral bool) *issuestorage.Issue {
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

// ageIssue backdates an issue's CreatedAt to simulate an old issue.
func ageIssue(t *testing.T, ctx context.Context, s issuestorage.IssueStore, issue *issuestorage.Issue, age time.Duration) {
	t.Helper()
	fresh, err := s.Get(ctx, issue.ID)
	if err != nil {
		t.Fatalf("Get %s: %v", issue.ID, err)
	}
	fresh.CreatedAt = time.Now().Add(-age)
	if err := s.Update(ctx, fresh); err != nil {
		t.Fatalf("Update %s: %v", issue.ID, err)
	}
}

func TestGC_DeletesEphemeralOlderThanThreshold(t *testing.T) {
	ctx := context.Background()
	s := newGCStore(t)

	old := createGCIssue(t, ctx, s, "Old Ephemeral", true)
	ageIssue(t, ctx, s, old, 2*time.Hour)

	result, err := GC(ctx, s, GCOptions{OlderThan: time.Hour})
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("Count: got %d, want 1", result.Count)
	}
	if len(result.RemovedIDs) != 1 || result.RemovedIDs[0] != old.ID {
		t.Errorf("RemovedIDs: got %v, want [%s]", result.RemovedIDs, old.ID)
	}

	// Issue should be gone.
	_, err = s.Get(ctx, old.ID)
	if !errors.Is(err, issuestorage.ErrNotFound) {
		t.Errorf("expected ErrNotFound for deleted issue, got: %v", err)
	}
}

func TestGC_DoesNotDeleteEphemeralNewerThanThreshold(t *testing.T) {
	ctx := context.Background()
	s := newGCStore(t)

	// Just created — well within the 1-hour threshold.
	fresh := createGCIssue(t, ctx, s, "Fresh Ephemeral", true)

	result, err := GC(ctx, s, GCOptions{OlderThan: time.Hour})
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("Count: got %d, want 0", result.Count)
	}

	// Issue should still exist.
	got, err := s.Get(ctx, fresh.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != fresh.ID {
		t.Errorf("ID: got %s, want %s", got.ID, fresh.ID)
	}
}

func TestGC_DoesNotDeletePersistentIssues(t *testing.T) {
	ctx := context.Background()
	s := newGCStore(t)

	// Persistent issue, even if old.
	persistent := createGCIssue(t, ctx, s, "Old Persistent", false)
	ageIssue(t, ctx, s, persistent, 2*time.Hour)

	result, err := GC(ctx, s, GCOptions{OlderThan: time.Hour})
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("Count: got %d, want 0", result.Count)
	}

	// Persistent issue should still exist.
	got, err := s.Get(ctx, persistent.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != persistent.ID {
		t.Errorf("ID: got %s, want %s", got.ID, persistent.ID)
	}
}

func TestGC_NoEphemeralIssues_ReturnsZero(t *testing.T) {
	ctx := context.Background()
	s := newGCStore(t)

	// Only persistent issues.
	createGCIssue(t, ctx, s, "Persistent A", false)
	createGCIssue(t, ctx, s, "Persistent B", false)

	result, err := GC(ctx, s, GCOptions{OlderThan: time.Hour})
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("Count: got %d, want 0", result.Count)
	}
	if len(result.RemovedIDs) != 0 {
		t.Errorf("RemovedIDs: got %v, want empty", result.RemovedIDs)
	}
}

func TestGC_DefaultOlderThan(t *testing.T) {
	ctx := context.Background()
	s := newGCStore(t)

	// Issue older than 1 hour (default threshold).
	old := createGCIssue(t, ctx, s, "Old Ephemeral", true)
	ageIssue(t, ctx, s, old, 2*time.Hour)

	// Fresh issue — should survive.
	fresh := createGCIssue(t, ctx, s, "Fresh Ephemeral", true)

	// OlderThan=0 triggers the 1-hour default.
	result, err := GC(ctx, s, GCOptions{})
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("Count: got %d, want 1", result.Count)
	}

	// Old issue deleted.
	_, err = s.Get(ctx, old.ID)
	if !errors.Is(err, issuestorage.ErrNotFound) {
		t.Errorf("old issue should be deleted, got: %v", err)
	}

	// Fresh issue survives.
	_, err = s.Get(ctx, fresh.ID)
	if err != nil {
		t.Errorf("fresh issue should survive, got: %v", err)
	}
}
