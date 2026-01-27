package filesystem

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"beads2/storage"
)

// TestDataIntegrityRoundTrip creates an issue with all fields populated
// (including unicode, newlines) and verifies deep equality after reading back.
func TestDataIntegrityRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fs := New(dir)

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a fully-populated issue with unicode and newlines
	closedAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	original := &storage.Issue{
		Title:       "Test Issue with Unicode: 中文 日本語 🚀",
		Description: "Multi-line description:\n\nLine 1 with special chars: <>&\"\nLine 2 with unicode: éèêë\n\nLine 3 with emoji: 🎉🎊🎁",
		Status:      storage.StatusInProgress,
		Priority:    storage.PriorityHigh,
		Type:        storage.TypeFeature,
		Parent:      "",
		Children:    []string{},
		DependsOn:   []string{},
		Dependents:  []string{},
		Blocks:      []string{},
		BlockedBy:   []string{},
		Labels:      []string{"urgent", "frontend", "unicode-test-测试"},
		Assignee:    "developer@example.com",
		Comments: []storage.Comment{
			{
				ID:        "c-0001",
				Author:    "reviewer",
				Body:      "Comment with newlines:\n\nParagraph 1.\n\nParagraph 2 with unicode: ✓ ✗ →",
				CreatedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:        "c-0002",
				Author:    "开发者", // Chinese for "developer"
				Body:      "这是一条中文评论", // "This is a Chinese comment"
				CreatedAt: time.Date(2026, 1, 11, 14, 30, 0, 0, time.UTC),
			},
		},
		CreatedAt: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 20, 15, 45, 0, 0, time.UTC),
		ClosedAt:  &closedAt,
	}

	// Create the issue
	id, err := fs.Create(ctx, original)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read it back
	retrieved, err := fs.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Update original with the assigned ID for comparison
	original.ID = id

	// Verify deep equality
	if !reflect.DeepEqual(original, retrieved) {
		t.Errorf("Round-trip failed: data mismatch")
		t.Errorf("Original:  %+v", original)
		t.Errorf("Retrieved: %+v", retrieved)

		// Provide more detailed diff
		if original.Title != retrieved.Title {
			t.Errorf("Title mismatch: %q vs %q", original.Title, retrieved.Title)
		}
		if original.Description != retrieved.Description {
			t.Errorf("Description mismatch: %q vs %q", original.Description, retrieved.Description)
		}
		if original.Status != retrieved.Status {
			t.Errorf("Status mismatch: %q vs %q", original.Status, retrieved.Status)
		}
		if original.Priority != retrieved.Priority {
			t.Errorf("Priority mismatch: %q vs %q", original.Priority, retrieved.Priority)
		}
		if original.Type != retrieved.Type {
			t.Errorf("Type mismatch: %q vs %q", original.Type, retrieved.Type)
		}
		if original.Assignee != retrieved.Assignee {
			t.Errorf("Assignee mismatch: %q vs %q", original.Assignee, retrieved.Assignee)
		}
		if !reflect.DeepEqual(original.Labels, retrieved.Labels) {
			t.Errorf("Labels mismatch: %v vs %v", original.Labels, retrieved.Labels)
		}
		if !reflect.DeepEqual(original.Comments, retrieved.Comments) {
			t.Errorf("Comments mismatch: %+v vs %+v", original.Comments, retrieved.Comments)
		}
		if !original.CreatedAt.Equal(retrieved.CreatedAt) {
			t.Errorf("CreatedAt mismatch: %v vs %v", original.CreatedAt, retrieved.CreatedAt)
		}
		if !original.UpdatedAt.Equal(retrieved.UpdatedAt) {
			t.Errorf("UpdatedAt mismatch: %v vs %v", original.UpdatedAt, retrieved.UpdatedAt)
		}
		if original.ClosedAt == nil || retrieved.ClosedAt == nil {
			t.Errorf("ClosedAt nil mismatch: %v vs %v", original.ClosedAt, retrieved.ClosedAt)
		} else if !original.ClosedAt.Equal(*retrieved.ClosedAt) {
			t.Errorf("ClosedAt mismatch: %v vs %v", *original.ClosedAt, *retrieved.ClosedAt)
		}
	}
}

// TestLargeDataSet creates 1000 issues with 1KB descriptions each,
// verifies all are readable, list returns all 1000, and doctor finds no problems.
func TestLargeDataSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large dataset test in short mode")
	}

	ctx := context.Background()
	dir := t.TempDir()
	fs := New(dir)

	if err := fs.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	const numIssues = 1000
	const descriptionSize = 1024 // 1KB

	// Generate a 1KB description
	baseDesc := strings.Repeat("Lorem ipsum dolor sit amet. ", 50)
	description := baseDesc[:descriptionSize]

	// Track created IDs
	createdIDs := make([]string, 0, numIssues)

	// Create 1000 issues
	for i := 0; i < numIssues; i++ {
		issue := &storage.Issue{
			Title:       "Test Issue",
			Description: description,
			Status:      storage.StatusOpen,
			Priority:    storage.PriorityMedium,
			Type:        storage.TypeTask,
			Labels:      []string{"bulk-test"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		id, err := fs.Create(ctx, issue)
		if err != nil {
			t.Fatalf("Create failed at issue %d: %v", i, err)
		}
		createdIDs = append(createdIDs, id)
	}

	// Verify all issues are readable
	for i, id := range createdIDs {
		issue, err := fs.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get failed for issue %d (ID: %s): %v", i, id, err)
		}
		if len(issue.Description) != descriptionSize {
			t.Errorf("Issue %d description size mismatch: got %d, want %d", i, len(issue.Description), descriptionSize)
		}
	}

	// Verify list returns all 1000 issues
	status := storage.StatusOpen
	filter := &storage.ListFilter{
		Status: &status,
	}
	issues, err := fs.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(issues) != numIssues {
		t.Errorf("List returned %d issues, want %d", len(issues), numIssues)
	}

	// Run doctor and verify no problems
	problems, err := fs.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}

	if len(problems) > 0 {
		t.Errorf("Doctor found %d problems:", len(problems))
		for _, p := range problems {
			t.Errorf("  - %s", p)
		}
	}
}
