package filesystem

import (
	"context"
	"testing"

	"beads2/storage"
)

// TestIDCollisionHandling tests that ID collisions are handled correctly
// by the retry mechanism in Create().
func TestIDCollisionHandling(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to initialize storage: %v", err)
	}

	// Create many issues to increase collision probability
	// With 4 hex chars (65536 possibilities) and 1000 issues,
	// we have a reasonable chance of exercising the retry path.
	const numIssues = 1000
	ids := make(map[string]bool)

	for i := 0; i < numIssues; i++ {
		issue := &storage.Issue{
			Title:    "Test Issue",
			Status:   storage.StatusOpen,
			Priority: storage.PriorityMedium,
			Type:     storage.TypeTask,
		}

		id, err := s.Create(ctx, issue)
		if err != nil {
			t.Fatalf("Create failed at iteration %d: %v", i, err)
		}

		if ids[id] {
			t.Fatalf("duplicate ID created: %s", id)
		}
		ids[id] = true
	}

	// Verify all issues can be retrieved
	for id := range ids {
		_, err := s.Get(ctx, id)
		if err != nil {
			t.Errorf("failed to retrieve issue %s: %v", id, err)
		}
	}
}

// TestIDFormat verifies that generated IDs follow the expected format.
func TestIDFormat(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("failed to initialize storage: %v", err)
	}

	issue := &storage.Issue{
		Title:    "Test Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}

	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// ID should be "bd-" followed by 4 hex characters
	if len(id) != 7 {
		t.Errorf("ID length: got %d, want 7", len(id))
	}

	if id[:3] != "bd-" {
		t.Errorf("ID prefix: got %q, want %q", id[:3], "bd-")
	}

	// Check that the remaining 4 characters are valid hex
	for _, c := range id[3:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("ID contains invalid hex character: %c", c)
		}
	}
}
