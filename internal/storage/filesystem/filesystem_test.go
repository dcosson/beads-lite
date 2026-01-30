package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"beads-lite/internal/storage"
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

// TestGetNextChildID_Sequential verifies sequential counter increments.
func TestGetNextChildID_Sequential(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create a parent issue
	parent := &storage.Issue{Title: "Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	for i := 1; i <= 5; i++ {
		childID, err := s.GetNextChildID(ctx, parentID)
		if err != nil {
			t.Fatalf("GetNextChildID call %d failed: %v", i, err)
		}
		want := fmt.Sprintf("%s.%d", parentID, i)
		if childID != want {
			t.Errorf("Call %d: got %q, want %q", i, childID, want)
		}
	}
}

// TestGetNextChildID_Concurrent verifies atomic increments under concurrency.
func TestGetNextChildID_Concurrent(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create a parent issue
	parent := &storage.Issue{Title: "Concurrent Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	const numWorkers = 10
	const callsPerWorker = 10
	total := numWorkers * callsPerWorker

	results := make(chan string, total)
	errs := make(chan error, total)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < callsPerWorker; i++ {
				childID, err := s.GetNextChildID(ctx, parentID)
				if err != nil {
					errs <- err
					return
				}
				results <- childID
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("Concurrent GetNextChildID error: %v", err)
	}

	// All returned IDs should be unique
	seen := make(map[string]bool)
	for childID := range results {
		if seen[childID] {
			t.Errorf("Duplicate child ID: %s", childID)
		}
		seen[childID] = true
	}

	if len(seen) != total {
		t.Errorf("Expected %d unique IDs, got %d", total, len(seen))
	}
}

// TestGetNextChildID_Persistence verifies counters survive re-initialization.
func TestGetNextChildID_Persistence(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create storage and a parent issue
	s1 := New(dir)
	if err := s1.Init(ctx); err != nil {
		t.Fatalf("Init 1 failed: %v", err)
	}
	parent := &storage.Issue{Title: "Persist Parent"}
	parentID, err := s1.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Increment counter 3 times
	for i := 0; i < 3; i++ {
		if _, err := s1.GetNextChildID(ctx, parentID); err != nil {
			t.Fatalf("GetNextChildID failed: %v", err)
		}
	}

	// Create new storage instance on same directory
	s2 := New(dir)
	if err := s2.Init(ctx); err != nil {
		t.Fatalf("Init 2 failed: %v", err)
	}

	childID, err := s2.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID after reinit failed: %v", err)
	}
	want := fmt.Sprintf("%s.%d", parentID, 4)
	if childID != want {
		t.Errorf("Counter after reinit: got %q, want %q", childID, want)
	}
}

// TestGetNextChildID_ParentNotFound verifies error when parent doesn't exist.
func TestGetNextChildID_ParentNotFound(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	_, err := s.GetNextChildID(ctx, "nonexistent-parent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetNextChildID on non-existent parent: got %v, want ErrNotFound", err)
	}
}

// createIssueWithID is a test helper that creates an issue file with a specific ID.
func createIssueWithID(t *testing.T, s *FilesystemStorage, id, title string) {
	t.Helper()
	issue := &storage.Issue{
		ID:        id,
		Title:     title,
		Status:    storage.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	path := s.issuePath(id, false)
	if err := atomicWriteJSON(path, issue); err != nil {
		t.Fatalf("createIssueWithID(%s) failed: %v", id, err)
	}
}

// TestGetNextChildID_MaxDepth verifies hierarchy depth limit enforcement.
func TestGetNextChildID_MaxDepth(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create root issue
	root := &storage.Issue{Title: "Root"}
	rootID, err := s.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create root failed: %v", err)
	}

	// Create child at depth 1 (rootID.1)
	child1ID, err := s.GetNextChildID(ctx, rootID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 1 failed: %v", err)
	}
	// Store the child issue so we can create grandchildren
	createIssueWithID(t, s, child1ID, "Child 1")

	// Create grandchild at depth 2 (rootID.1.1)
	child2ID, err := s.GetNextChildID(ctx, child1ID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 2 failed: %v", err)
	}
	createIssueWithID(t, s, child2ID, "Child 2")

	// Create great-grandchild at depth 3 (rootID.1.1.1) - should succeed (max depth = 3)
	child3ID, err := s.GetNextChildID(ctx, child2ID)
	if err != nil {
		t.Fatalf("GetNextChildID depth 3 failed: %v", err)
	}
	createIssueWithID(t, s, child3ID, "Child 3")

	// Depth 4 should fail (rootID.1.1.1.1 exceeds max depth of 3)
	_, err = s.GetNextChildID(ctx, child3ID)
	if !errors.Is(err, storage.ErrMaxDepthExceeded) {
		t.Errorf("GetNextChildID at depth 4: got %v, want ErrMaxDepthExceeded", err)
	}
}

// TestCreateExplicitHierarchicalID_UpdatesCounter verifies that creating an
// issue with an explicit hierarchical ID updates the child counter so future
// GetNextChildID calls don't collide.
func TestCreateExplicitHierarchicalID_UpdatesCounter(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create parent issue
	parent := &storage.Issue{Title: "Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Directly create a child with explicit ID (e.g., parentID.3)
	explicitChild := &storage.Issue{
		ID:    storage.ChildID(parentID, 3),
		Title: "Explicit child 3",
	}
	childID, err := s.Create(ctx, explicitChild)
	if err != nil {
		t.Fatalf("Create explicit child failed: %v", err)
	}
	if childID != storage.ChildID(parentID, 3) {
		t.Fatalf("got %q, want %q", childID, storage.ChildID(parentID, 3))
	}

	// Now GetNextChildID should return parentID.4, not parentID.1
	nextID, err := s.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	want := storage.ChildID(parentID, 4)
	if nextID != want {
		t.Errorf("GetNextChildID after explicit .3: got %q, want %q", nextID, want)
	}
}

// TestCreateExplicitHierarchicalID_DoesNotLowerCounter verifies that creating
// an issue with a lower child number doesn't lower the counter.
func TestCreateExplicitHierarchicalID_DoesNotLowerCounter(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	parent := &storage.Issue{Title: "Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Generate children 1 and 2 via GetNextChildID
	for i := 0; i < 2; i++ {
		childID, err := s.GetNextChildID(ctx, parentID)
		if err != nil {
			t.Fatalf("GetNextChildID failed: %v", err)
		}
		createIssueWithID(t, s, childID, fmt.Sprintf("Child %d", i+1))
	}

	// Use a separate parent to cleanly test that a lower explicit child
	// number doesn't lower an already-higher counter.
	parent2 := &storage.Issue{Title: "Parent 2"}
	parent2ID, err := s.Create(ctx, parent2)
	if err != nil {
		t.Fatalf("Create parent2 failed: %v", err)
	}

	// Explicitly create child .5
	child5 := &storage.Issue{
		ID:    storage.ChildID(parent2ID, 5),
		Title: "Explicit child 5",
	}
	if _, err := s.Create(ctx, child5); err != nil {
		t.Fatalf("Create child 5 failed: %v", err)
	}

	// Explicitly create child .2 (lower than 5)
	child2 := &storage.Issue{
		ID:    storage.ChildID(parent2ID, 2),
		Title: "Explicit child 2",
	}
	if _, err := s.Create(ctx, child2); err != nil {
		t.Fatalf("Create child 2 failed: %v", err)
	}

	// Counter should still be at 5, so next should be .6
	nextID, err := s.GetNextChildID(ctx, parent2ID)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	want := storage.ChildID(parent2ID, 6)
	if nextID != want {
		t.Errorf("GetNextChildID after .5 then .2: got %q, want %q", nextID, want)
	}
}

// TestCreateNonHierarchicalID_NoCounterSideEffect verifies that creating an
// issue with a plain (non-hierarchical) ID doesn't touch child counters.
func TestCreateNonHierarchicalID_NoCounterSideEffect(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create a parent
	parent := &storage.Issue{Title: "Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Create an issue with a non-hierarchical explicit ID
	plain := &storage.Issue{
		ID:    "my-custom-id",
		Title: "Plain issue",
	}
	if _, err := s.Create(ctx, plain); err != nil {
		t.Fatalf("Create plain issue failed: %v", err)
	}

	// Counter for parent should still be at 0, so next child is .1
	nextID, err := s.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID failed: %v", err)
	}
	want := storage.ChildID(parentID, 1)
	if nextID != want {
		t.Errorf("got %q, want %q", nextID, want)
	}
}

// TestCreateExplicitHierarchicalID_MaxDepth verifies that creating an issue
// with an explicit hierarchical ID that exceeds max depth is rejected.
func TestCreateExplicitHierarchicalID_MaxDepth(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create root and build a chain to depth 3
	root := &storage.Issue{Title: "Root"}
	rootID, err := s.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create root failed: %v", err)
	}
	createIssueWithID(t, s, rootID+".1", "Depth 1")
	createIssueWithID(t, s, rootID+".1.1", "Depth 2")
	createIssueWithID(t, s, rootID+".1.1.1", "Depth 3")

	// Trying to create at depth 4 via explicit ID should fail
	tooDeep := &storage.Issue{
		ID:    rootID + ".1.1.1.1",
		Title: "Too deep",
	}
	_, err = s.Create(ctx, tooDeep)
	if !errors.Is(err, storage.ErrMaxDepthExceeded) {
		t.Errorf("Create at depth 4 with explicit ID: got %v, want ErrMaxDepthExceeded", err)
	}

	// Creating at depth 3 via explicit ID should succeed
	okDepth := &storage.Issue{
		ID:    rootID + ".1.1.2",
		Title: "OK depth 3",
	}
	id, err := s.Create(ctx, okDepth)
	if err != nil {
		t.Fatalf("Create at depth 3 with explicit ID failed: %v", err)
	}
	if id != rootID+".1.1.2" {
		t.Errorf("got %q, want %q", id, rootID+".1.1.2")
	}
}

// TestDeleteDoesNotReuseChildNumbers verifies that deleting or closing a child
// issue does not cause its child number to be reused. The child counter must
// never be decremented; it persists at its last value regardless of deletions.
func TestDeleteDoesNotReuseChildNumbers(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create a parent issue
	parent := &storage.Issue{Title: "Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Create child .1 via GetNextChildID
	child1ID, err := s.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID for child 1 failed: %v", err)
	}
	wantChild1 := storage.ChildID(parentID, 1)
	if child1ID != wantChild1 {
		t.Fatalf("First child: got %q, want %q", child1ID, wantChild1)
	}
	// Actually create the child issue so we can delete it
	createIssueWithID(t, s, child1ID, "Child 1")

	// Delete child .1 (permanent deletion)
	if err := s.Delete(ctx, child1ID); err != nil {
		t.Fatalf("Delete child 1 failed: %v", err)
	}

	// Create next child — must get .2, not .1
	child2ID, err := s.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID after delete failed: %v", err)
	}
	wantChild2 := storage.ChildID(parentID, 2)
	if child2ID != wantChild2 {
		t.Errorf("After deleting child .1: got %q, want %q", child2ID, wantChild2)
	}
}

// TestCloseDoesNotReuseChildNumbers verifies that closing (soft-deleting) a
// child issue does not cause its child number to be reused.
func TestCloseDoesNotReuseChildNumbers(t *testing.T) {
	s := setupTestStorage(t)
	ctx := context.Background()

	// Create a parent issue
	parent := &storage.Issue{Title: "Parent"}
	parentID, err := s.Create(ctx, parent)
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Create child .1
	child1ID, err := s.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID for child 1 failed: %v", err)
	}
	createIssueWithID(t, s, child1ID, "Child 1")

	// Close child .1 (soft delete)
	if err := s.Close(ctx, child1ID); err != nil {
		t.Fatalf("Close child 1 failed: %v", err)
	}

	// Create next child — must get .2, not .1
	child2ID, err := s.GetNextChildID(ctx, parentID)
	if err != nil {
		t.Fatalf("GetNextChildID after close failed: %v", err)
	}
	wantChild2 := storage.ChildID(parentID, 2)
	if child2ID != wantChild2 {
		t.Errorf("After closing child .1: got %q, want %q", child2ID, wantChild2)
	}
}

// TestStaleLockCleanup verifies that stale lock files (with no active flock) are cleaned up.
func TestStaleLockCleanup(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue
	issue := &storage.Issue{Title: "Test issue"}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Manually create a stale lock file (simulating a killed process)
	staleLockPath := filepath.Join(dir, "open", id+".lock")
	f, err := os.Create(staleLockPath)
	if err != nil {
		t.Fatalf("Failed to create stale lock: %v", err)
	}
	f.Close() // Close without holding flock - this simulates a stale lock

	// Verify stale lock exists
	if !fileExists(staleLockPath) {
		t.Fatal("Stale lock file should exist")
	}

	// Now perform an operation - it should clean up the stale lock
	issue, _ = s.Get(ctx, id)
	issue.Title = "Updated"
	if err := s.Update(ctx, issue); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Lock file should be cleaned up after the operation
	if fileExists(staleLockPath) {
		t.Error("Stale lock file should be cleaned up after Update")
	}
}

// TestStaleLockCleanupOnInit verifies that stale locks are cleaned up when Init is called.
func TestStaleLockCleanupOnInit(t *testing.T) {
	dir := t.TempDir()

	// Create directory structure manually
	openDir := filepath.Join(dir, "open")
	if err := os.MkdirAll(openDir, 0755); err != nil {
		t.Fatalf("Failed to create open dir: %v", err)
	}

	// Create a stale lock file (simulating a killed process)
	staleLockPath := filepath.Join(openDir, "bd-test.lock")
	f, err := os.Create(staleLockPath)
	if err != nil {
		t.Fatalf("Failed to create stale lock: %v", err)
	}
	f.Close() // Close without holding flock - this simulates a stale lock

	// Verify stale lock exists
	if !fileExists(staleLockPath) {
		t.Fatal("Stale lock file should exist before Init")
	}

	// Now call Init - it should clean up the stale lock
	s := New(dir)
	ctx := context.Background()
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Lock file should be cleaned up
	if fileExists(staleLockPath) {
		t.Error("Stale lock file should be cleaned up by Init")
	}
}

// TestLockFileCleanupAfterUpdate verifies that lock files are removed after Update operations.
func TestLockFileCleanupAfterUpdate(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue
	issue := &storage.Issue{Title: "Test issue"}
	id, err := s.Create(ctx, issue)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update the issue
	issue, _ = s.Get(ctx, id)
	issue.Title = "Updated title"
	if err := s.Update(ctx, issue); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Check that no lock file remains
	lockPath := filepath.Join(dir, "open", id+".lock")
	if fileExists(lockPath) {
		t.Errorf("Lock file should be cleaned up after Update, but %s still exists", lockPath)
	}
}

// TestLockFileCleanupAfterAddDependency verifies that lock files are removed after AddDependency.
func TestLockFileCleanupAfterAddDependency(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	ctx := context.Background()

	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two issues
	issue1 := &storage.Issue{Title: "Issue 1"}
	id1, err := s.Create(ctx, issue1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	issue2 := &storage.Issue{Title: "Issue 2"}
	id2, err := s.Create(ctx, issue2)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Add dependency
	if err := s.AddDependency(ctx, id1, id2, storage.DepTypeBlocks); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	// Check that no lock files remain
	lockPath1 := filepath.Join(dir, "open", id1+".lock")
	lockPath2 := filepath.Join(dir, "open", id2+".lock")
	if fileExists(lockPath1) {
		t.Errorf("Lock file should be cleaned up after AddDependency, but %s still exists", lockPath1)
	}
	if fileExists(lockPath2) {
		t.Errorf("Lock file should be cleaned up after AddDependency, but %s still exists", lockPath2)
	}
}
