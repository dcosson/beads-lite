package issuestorage_test

import (
	"context"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

// TestFilesystemContract runs the storage contract tests against FilesystemStorage.
func TestFilesystemContract(t *testing.T) {
	factory := func() issuestorage.IssueStore {
		dir := t.TempDir()
		fs := filesystem.New(dir)
		if err := fs.Init(context.Background()); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		return fs
	}
	issuestorage.RunContractTests(t, factory)
}

// TestFilesystemConcurrent runs the concurrent access tests against FilesystemStorage.
func TestFilesystemConcurrent(t *testing.T) {
	suite := &issuestorage.ConcurrentTestSuite{
		NewStorage: func(t *testing.T) issuestorage.IssueStore {
			dir := t.TempDir()
			fs := filesystem.New(dir)
			if err := fs.Init(context.Background()); err != nil {
				t.Fatalf("Init failed: %v", err)
			}
			return fs
		},
	}
	suite.Run(t)
}
