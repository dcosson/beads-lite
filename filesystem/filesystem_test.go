package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beads2/storage"
)

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
