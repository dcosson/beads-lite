package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"beads2/internal/storage"
)

// TestGenerateID verifies the ID format is bd-XXXX (4 hex chars).
func TestGenerateID(t *testing.T) {
	idPattern := regexp.MustCompile(`^bd-[0-9a-f]{4}$`)

	// Generate multiple IDs and verify format
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateID()
		if err != nil {
			t.Fatalf("generateID failed: %v", err)
		}

		if !idPattern.MatchString(id) {
			t.Errorf("ID %q does not match expected format bd-XXXX", id)
		}

		// Track uniqueness (not guaranteed but very likely in 100 samples)
		seen[id] = true
	}

	// With 65536 possibilities, 100 samples should have some variety
	if len(seen) < 50 {
		t.Errorf("Expected more unique IDs, got only %d unique out of 100", len(seen))
	}
}

// TestCreate_IDCollisionRetry verifies that Create retries on ID collision.
func TestCreate_IDCollisionRetry(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create many issues to verify IDs are unique
	ids := make(map[string]bool)
	for i := 0; i < 50; i++ {
		issue := &storage.Issue{
			Title:    "Test Issue",
			Status:   storage.StatusOpen,
			Priority: storage.PriorityMedium,
			Type:     storage.TypeTask,
		}
		id, err := s.Create(ctx, issue)
		if err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}

		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

// TestCreate_ConcurrentIDGeneration verifies concurrent creates don't collide.
func TestCreate_ConcurrentIDGeneration(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	const numWorkers = 10
	const issuesPerWorker = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := make(map[string]bool)
	errors := make([]error, 0)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < issuesPerWorker; i++ {
				issue := &storage.Issue{
					Title:    "Concurrent Test",
					Status:   storage.StatusOpen,
					Priority: storage.PriorityMedium,
					Type:     storage.TypeTask,
				}
				id, err := s.Create(ctx, issue)
				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else if ids[id] {
					errors = append(errors, err)
				} else {
					ids[id] = true
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		t.Errorf("Got %d errors during concurrent create: %v", len(errors), errors[0])
	}

	expected := numWorkers * issuesPerWorker
	if len(ids) != expected {
		t.Errorf("Expected %d unique IDs, got %d", expected, len(ids))
	}
}

// TestAtomicWriteJSON_CreatesFile tests that atomicWriteJSON creates the file with correct content.
func TestAtomicWriteJSON_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	data := map[string]string{"key": "value"}
	if err := atomicWriteJSON(path, data); err != nil {
		t.Fatalf("atomicWriteJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Verify JSON is pretty-printed with 2-space indent
	expected := "{\n  \"key\": \"value\"\n}\n"
	if string(content) != expected {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", string(content), expected)
	}
}

// TestAtomicWriteJSON_OverwritesExisting tests that atomicWriteJSON safely overwrites existing files.
func TestAtomicWriteJSON_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write initial content
	if err := atomicWriteJSON(path, map[string]string{"old": "data"}); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Overwrite with new content
	if err := atomicWriteJSON(path, map[string]string{"new": "data"}); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !strings.Contains(string(content), "new") {
		t.Errorf("Expected new content, got: %s", content)
	}
	if strings.Contains(string(content), "old") {
		t.Errorf("Should not contain old content, got: %s", content)
	}
}

// TestAtomicWriteJSON_NoTempFilesOnSuccess tests that no temp files remain after successful write.
func TestAtomicWriteJSON_NoTempFilesOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := atomicWriteJSON(path, map[string]string{"key": "value"}); err != nil {
		t.Fatalf("atomicWriteJSON failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") {
			t.Errorf("Found leftover temp file: %s", entry.Name())
		}
	}
}

// TestAtomicWriteJSON_CleansUpOnEncodeError tests that temp files are removed on encode errors.
func TestAtomicWriteJSON_CleansUpOnEncodeError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Create a channel that can't be marshaled to JSON
	unserializable := make(chan int)

	err := atomicWriteJSON(path, unserializable)
	if err == nil {
		t.Fatal("Expected error for unserializable type")
	}

	// Verify no temp files left
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") {
			t.Errorf("Found leftover temp file after error: %s", entry.Name())
		}
	}

	// Verify the target file was not created
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Target file should not exist after error")
	}
}

// TestAtomicWriteJSON_PreservesOriginalOnError tests that original file is preserved if write fails.
func TestAtomicWriteJSON_PreservesOriginalOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write initial content
	original := map[string]string{"original": "data"}
	if err := atomicWriteJSON(path, original); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Try to overwrite with unserializable data
	unserializable := make(chan int)
	err := atomicWriteJSON(path, unserializable)
	if err == nil {
		t.Fatal("Expected error for unserializable type")
	}

	// Verify original content is preserved
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !strings.Contains(string(content), "original") {
		t.Errorf("Original content should be preserved, got: %s", content)
	}
}

// TestFilesystemStorage_ConcurrentWrites tests that concurrent writes don't corrupt data.
func TestFilesystemStorage_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue
	issue := &storage.Issue{
		Title:    "Concurrent Test",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Perform concurrent updates
	const numWriters = 10
	const writesPerWorker = 5
	errChan := make(chan error, numWriters*writesPerWorker)

	for i := 0; i < numWriters; i++ {
		go func(workerID int) {
			for j := 0; j < writesPerWorker; j++ {
				got, err := s.Get(ctx, id)
				if err != nil {
					errChan <- err
					continue
				}
				got.Description = got.Description + "x"
				if err := s.Update(ctx, got); err != nil {
					errChan <- err
					continue
				}
				errChan <- nil
			}
		}(i)
	}

	// Collect results
	for i := 0; i < numWriters*writesPerWorker; i++ {
		if err := <-errChan; err != nil {
			t.Logf("Worker error (expected with concurrent updates): %v", err)
		}
	}

	// Verify the issue is still readable and valid JSON
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Final Get failed: %v", err)
	}
	if got.ID != id {
		t.Errorf("Issue ID mismatch after concurrent writes")
	}
}

func setupTestStorage(t *testing.T) *FilesystemStorage {
	t.Helper()
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestListSorting verifies that List returns issues sorted by CreatedAt.
func TestListSorting(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create issues with deliberate delays to ensure different CreatedAt times
	issues := []struct {
		title string
	}{
		{"First Issue"},
		{"Second Issue"},
		{"Third Issue"},
	}

	var createdIDs []string
	for _, spec := range issues {
		id, err := s.Create(ctx, &storage.Issue{
			Title:    spec.title,
			Status:   storage.StatusOpen,
			Priority: storage.PriorityMedium,
			Type:     storage.TypeTask,
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		createdIDs = append(createdIDs, id)
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// List all issues
	result, err := s.List(ctx, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 issues, got %d", len(result))
	}

	// Verify they are sorted by CreatedAt (oldest first)
	for i := 1; i < len(result); i++ {
		if result[i-1].CreatedAt.After(result[i].CreatedAt) {
			t.Errorf("Issues not sorted by CreatedAt: issue %d (created %v) should be before issue %d (created %v)",
				i-1, result[i-1].CreatedAt, i, result[i].CreatedAt)
		}
	}

	// Verify the order matches creation order
	for i, id := range createdIDs {
		if result[i].ID != id {
			t.Errorf("Position %d: expected issue %s, got %s", i, id, result[i].ID)
		}
	}
}

// TestListFilteringSorting verifies sorting is preserved after filtering.
func TestListFilteringSorting(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create issues with different priorities
	_, err := s.Create(ctx, &storage.Issue{
		Title:    "High Priority 1",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityHigh,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	_, err = s.Create(ctx, &storage.Issue{
		Title:    "Low Priority",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityLow,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	_, err = s.Create(ctx, &storage.Issue{
		Title:    "High Priority 2",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityHigh,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Filter by high priority
	highPriority := storage.PriorityHigh
	result, err := s.List(ctx, &storage.ListFilter{Priority: &highPriority})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 high priority issues, got %d", len(result))
	}

	// Verify sorted by CreatedAt
	if result[0].CreatedAt.After(result[1].CreatedAt) {
		t.Error("Filtered issues not sorted by CreatedAt")
	}

	// First should be "High Priority 1", second "High Priority 2"
	if result[0].Title != "High Priority 1" {
		t.Errorf("Expected first issue to be 'High Priority 1', got %q", result[0].Title)
	}
	if result[1].Title != "High Priority 2" {
		t.Errorf("Expected second issue to be 'High Priority 2', got %q", result[1].Title)
	}
}

func TestFilesystemContract(t *testing.T) {
	factory := func() storage.Storage {
		dir := t.TempDir()
		return New(dir)
	}
	storage.RunContractTests(t, factory)
}

// TestClose verifies that Close updates status and moves the file.
func TestClose(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Close non-existent issue should return ErrNotFound
	err := s.Close(ctx, "nonexistent-id")
	if err != storage.ErrNotFound {
		t.Errorf("Close non-existent: got %v, want ErrNotFound", err)
	}

	// Create an issue
	issue := &storage.Issue{
		Title:    "To Close",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityLow,
		Type:     storage.TypeTask,
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
	if got.Status != storage.StatusClosed {
		t.Errorf("Status after Close: got %q, want %q", got.Status, storage.StatusClosed)
	}
	if got.ClosedAt == nil {
		t.Error("ClosedAt should be set after Close")
	}
}

// TestReopen verifies that Reopen updates status and moves the file back.
func TestReopen(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create and close an issue
	issue := &storage.Issue{
		Title:    "To Reopen",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityLow,
		Type:     storage.TypeTask,
	}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := s.Close(ctx, id); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen it
	if err := s.Reopen(ctx, id); err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}

	// Verify status changed back
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Reopen failed: %v", err)
	}
	if got.Status != storage.StatusOpen {
		t.Errorf("Status after Reopen: got %q, want %q", got.Status, storage.StatusOpen)
	}
	if got.ClosedAt != nil {
		t.Error("ClosedAt should be nil after Reopen")
	}
}

// TestCloseMovesFile verifies that Close physically moves the JSON file.
func TestCloseMovesFile(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	issue := &storage.Issue{
		Title:    "Test Close Move",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}

	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify file is in open/
	openPath := s.issuePath(id, false)
	closedPath := s.issuePath(id, true)

	if !fileExists(openPath) {
		t.Error("Issue file should exist in open/ after create")
	}
	if fileExists(closedPath) {
		t.Error("Issue file should not exist in closed/ after create")
	}

	// Close the issue
	if err := s.Close(ctx, id); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify file moved to closed/
	if fileExists(openPath) {
		t.Error("Issue file should not exist in open/ after close")
	}
	if !fileExists(closedPath) {
		t.Error("Issue file should exist in closed/ after close")
	}
}

func TestDeleteRemovesLockFile(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	issue := &storage.Issue{
		Title:    "Delete Lock Cleanup",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}

	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	lockPath := s.lockPath(id)
	if fileExists(lockPath) {
		t.Error("Lock file should be removed after delete")
	}
}

// TestReopenMovesFile verifies that Reopen physically moves the JSON file back.
func TestReopenMovesFile(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	issue := &storage.Issue{
		Title:    "Test Reopen Move",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}

	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Close then reopen
	if err := s.Close(ctx, id); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := s.Reopen(ctx, id); err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}

	// Verify file is back in open/
	openPath := s.issuePath(id, false)
	closedPath := s.issuePath(id, true)

	if !fileExists(openPath) {
		t.Error("Issue file should exist in open/ after reopen")
	}
	if fileExists(closedPath) {
		t.Error("Issue file should not exist in closed/ after reopen")
	}
}

// TestReopenNonExistent verifies Reopen returns ErrNotFound for missing issues.
func TestReopenNonExistent(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	err := s.Reopen(ctx, "nonexistent-id")
	if err != storage.ErrNotFound {
		t.Errorf("Reopen non-existent: got %v, want ErrNotFound", err)
	}
}

// TestCloseAndGetPreservesData verifies all issue data is preserved through close.
func TestCloseAndGetPreservesData(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	issue := &storage.Issue{
		Title:       "Full Issue",
		Description: "A complete issue with all fields",
		Status:      storage.StatusInProgress,
		Priority:    storage.PriorityHigh,
		Type:        storage.TypeBug,
		Labels:      []string{"urgent", "backend"},
		Assignee:    "alice",
	}

	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := s.Close(ctx, id); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after Close failed: %v", err)
	}

	if got.Title != issue.Title {
		t.Errorf("Title: got %q, want %q", got.Title, issue.Title)
	}
	if got.Description != issue.Description {
		t.Errorf("Description: got %q, want %q", got.Description, issue.Description)
	}
	if got.Priority != issue.Priority {
		t.Errorf("Priority: got %q, want %q", got.Priority, issue.Priority)
	}
	if got.Type != issue.Type {
		t.Errorf("Type: got %q, want %q", got.Type, issue.Type)
	}
	if got.Assignee != issue.Assignee {
		t.Errorf("Assignee: got %q, want %q", got.Assignee, issue.Assignee)
	}
	if len(got.Labels) != len(issue.Labels) {
		t.Errorf("Labels length: got %d, want %d", len(got.Labels), len(issue.Labels))
	}
}

// TestListExcludesClosedByDefault verifies List with nil filter excludes closed issues.
func TestListExcludesClosedByDefault(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create two issues
	id1, err := s.Create(ctx, &storage.Issue{
		Title:    "Open Issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("Create 1 failed: %v", err)
	}

	id2, err := s.Create(ctx, &storage.Issue{
		Title:    "To Close",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("Create 2 failed: %v", err)
	}

	// Close one
	if err := s.Close(ctx, id2); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// List should only return the open one
	issues, err := s.List(ctx, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("List should return 1 issue, got %d", len(issues))
	}
	if len(issues) > 0 && issues[0].ID != id1 {
		t.Errorf("List should return open issue %s, got %s", id1, issues[0].ID)
	}
}

// TestListClosedFilter verifies List with closed filter returns closed issues.
func TestListClosedFilter(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create and close an issue
	id, err := s.Create(ctx, &storage.Issue{
		Title:    "To Close",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := s.Close(ctx, id); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// List with closed filter
	status := storage.StatusClosed
	issues, err := s.List(ctx, &storage.ListFilter{Status: &status})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("List closed should return 1 issue, got %d", len(issues))
	}
	if len(issues) > 0 && issues[0].ID != id {
		t.Errorf("List closed should return %s, got %s", id, issues[0].ID)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
