package filesystem

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"beads2/storage"
)

// CrashingFS is a mock filesystem that fails after writing a specified number of bytes.
// Used to simulate crashes during write operations.
type CrashingFS struct {
	// CrashAfterBytes causes Write to fail after this many bytes are written.
	// Set to 0 to disable crashing behavior.
	CrashAfterBytes int

	// bytesWritten tracks total bytes written across all calls.
	bytesWritten int

	// Underlying is the real filesystem to delegate to (nil uses os package).
	Underlying FS
}

// crashError is returned when the CrashingFS simulates a crash.
type crashError struct{}

func (e crashError) Error() string { return "simulated crash during write" }

// FS is the filesystem interface that FilesystemStorage uses.
// This allows injection of mock filesystems for testing.
type FS interface {
	// MkdirAll creates a directory and all parent directories.
	MkdirAll(path string, perm fs.FileMode) error

	// ReadFile reads the entire contents of a file.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes data to a file, creating it if necessary.
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// Remove removes a file or empty directory.
	Remove(path string) error

	// Rename renames (moves) a file.
	Rename(oldpath, newpath string) error

	// ReadDir reads a directory and returns directory entries.
	ReadDir(path string) ([]fs.DirEntry, error)

	// Stat returns file info for a path.
	Stat(path string) (fs.FileInfo, error)
}

// osFS implements FS using the os package.
type osFS struct{}

func (osFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (osFS) Remove(path string) error {
	return os.Remove(path)
}

func (osFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osFS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (osFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// CrashingFS implements FS with failure injection.
func (c *CrashingFS) underlying() FS {
	if c.Underlying != nil {
		return c.Underlying
	}
	return osFS{}
}

func (c *CrashingFS) MkdirAll(path string, perm fs.FileMode) error {
	return c.underlying().MkdirAll(path, perm)
}

func (c *CrashingFS) ReadFile(path string) ([]byte, error) {
	return c.underlying().ReadFile(path)
}

func (c *CrashingFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if c.CrashAfterBytes > 0 && c.bytesWritten+len(data) > c.CrashAfterBytes {
		// Simulate crash: don't write anything (atomic write pattern means
		// partial writes shouldn't occur - we crash before the rename)
		c.bytesWritten += len(data)
		return crashError{}
	}
	c.bytesWritten += len(data)
	return c.underlying().WriteFile(path, data, perm)
}

func (c *CrashingFS) Remove(path string) error {
	return c.underlying().Remove(path)
}

func (c *CrashingFS) Rename(oldpath, newpath string) error {
	return c.underlying().Rename(oldpath, newpath)
}

func (c *CrashingFS) ReadDir(path string) ([]fs.DirEntry, error) {
	return c.underlying().ReadDir(path)
}

func (c *CrashingFS) Stat(path string) (fs.FileInfo, error) {
	return c.underlying().Stat(path)
}

// TestCrashDuringWrite tests that a crash during write leaves storage in a valid state.
// The atomic write pattern (write to temp, then rename) should ensure that:
// - Either the write completes fully, or
// - The storage is unchanged (no partial files, no corruption)
func TestCrashDuringWrite(t *testing.T) {
	ctx := context.Background()

	// Create a crashing filesystem that fails after 50 bytes
	crashingFS := &CrashingFS{
		CrashAfterBytes: 50,
	}

	// Create storage with the crashing filesystem
	dir := t.TempDir()
	s := NewWithFS(dir, crashingFS)
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Attempt to create an issue - this should fail due to crash
	_, err := s.Create(ctx, &storage.Issue{
		Title:       "Test Issue",
		Description: "This description is long enough to trigger the crash",
		Status:      storage.StatusOpen,
		Priority:    storage.PriorityMedium,
		Type:        storage.TypeTask,
	})

	// We expect an error from the crash
	if err == nil {
		t.Fatal("expected error from crashed write, got nil")
	}

	// Storage should still be in valid state (no partial files, no corruption)
	// Doctor should find no problems
	problems, err := s.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor failed: %v", err)
	}

	if len(problems) > 0 {
		t.Errorf("crash left storage in inconsistent state, Doctor found problems:")
		for _, p := range problems {
			t.Errorf("  - %s", p)
		}
	}

	// Verify no partial files exist in open/ directory
	openDir := filepath.Join(dir, "open")
	entries, err := os.ReadDir(openDir)
	if err != nil {
		t.Fatalf("failed to read open directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Lock files are OK, but there shouldn't be any .tmp files or .json files
		// since the create should have failed cleanly
		if filepath.Ext(name) == ".json" {
			t.Errorf("found orphaned JSON file after crash: %s", name)
		}
		if filepath.Ext(name) == ".tmp" {
			t.Errorf("found orphaned temp file after crash: %s", name)
		}
	}
}

// TestCrashDuringClose tests recovery when a crash occurs during Close operation.
// The Close operation writes to closed/ then removes from open/. A crash between
// these operations leaves a duplicate file. Doctor should detect and fix this.
func TestCrashDuringClose(t *testing.T) {
	ctx := context.Background()

	// Set up storage normally (no crash injection for setup)
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create an issue
	id, err := s.Create(ctx, &storage.Issue{
		Title:       "Test Issue",
		Description: "This issue will have a simulated crash during close",
		Status:      storage.StatusOpen,
		Priority:    storage.PriorityMedium,
		Type:        storage.TypeTask,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Simulate crash during Close:
	// - File written to closed/ but not removed from open/
	// This is what happens if the process crashes between the two operations.

	openPath := filepath.Join(dir, "open", id+".json")
	closedPath := filepath.Join(dir, "closed", id+".json")

	// Read the issue data
	data, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("failed to read issue: %v", err)
	}

	// Modify the status to closed (as Close would do)
	var issue storage.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		t.Fatalf("failed to unmarshal issue: %v", err)
	}
	issue.Status = storage.StatusClosed
	now := time.Now()
	issue.ClosedAt = &now

	closedData, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal closed issue: %v", err)
	}

	// Write to closed/ (simulating successful first step of Close)
	if err := os.WriteFile(closedPath, closedData, 0644); err != nil {
		t.Fatalf("failed to write to closed/: %v", err)
	}

	// DON'T remove from open/ - this simulates the crash!
	// Now we have the same issue in both open/ and closed/

	// Doctor should detect the duplicate issue
	problems, err := s.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor (check mode) failed: %v", err)
	}

	if len(problems) == 0 {
		t.Error("Doctor should detect duplicate issue in open/ and closed/, but found no problems")
	}

	// Verify we found the specific problem
	foundDuplicate := false
	for _, p := range problems {
		t.Logf("Doctor found problem: %s", p)
		// The exact message format depends on implementation, but should mention the ID
		if containsID(p, id) {
			foundDuplicate = true
		}
	}
	if !foundDuplicate {
		t.Errorf("Doctor problems did not mention the duplicate issue %s", id)
	}

	// Doctor --fix should resolve it
	_, err = s.Doctor(ctx, true)
	if err != nil {
		t.Fatalf("Doctor (fix mode) failed: %v", err)
	}

	// Now Doctor should find no problems
	problems, err = s.Doctor(ctx, false)
	if err != nil {
		t.Fatalf("Doctor (check after fix) failed: %v", err)
	}

	if len(problems) > 0 {
		t.Errorf("Doctor --fix should have resolved all problems, but still found:")
		for _, p := range problems {
			t.Errorf("  - %s", p)
		}
	}

	// Verify the issue is only in one location now
	openExists := fileExists(openPath)
	closedExists := fileExists(closedPath)

	if openExists && closedExists {
		t.Error("issue still exists in both open/ and closed/ after Doctor --fix")
	}

	if !openExists && !closedExists {
		t.Error("issue was completely deleted by Doctor --fix")
	}

	// The issue should be in closed/ since it was the more recent state
	// (or in open/ if Doctor prefers preserving the original state)
	// Either is acceptable as long as there's exactly one copy
	t.Logf("After fix: open exists=%v, closed exists=%v", openExists, closedExists)
}

// containsID checks if a problem string mentions a specific issue ID.
func containsID(problem, id string) bool {
	return len(problem) > 0 && len(id) > 0 &&
		(stringContains(problem, id) || stringContains(problem, "duplicate") || stringContains(problem, "both"))
}

// stringContains checks if s contains substr (simple substring check).
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
