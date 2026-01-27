package storage_test

import (
	"context"
	"testing"

	"beads2/internal/storage/filesystem"
	"beads2/internal/storage"
)

// TestFilesystemContract runs the storage contract tests against FilesystemStorage.
func TestFilesystemContract(t *testing.T) {
	factory := func() storage.Storage {
		dir := t.TempDir()
		fs := filesystem.New(dir)
		if err := fs.Init(context.Background()); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		return fs
	}
	storage.RunContractTests(t, factory)
}

// TestFilesystemConcurrent runs the concurrent access tests against FilesystemStorage.
func TestFilesystemConcurrent(t *testing.T) {
	suite := &storage.ConcurrentTestSuite{
		NewStorage: func(t *testing.T) storage.Storage {
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
