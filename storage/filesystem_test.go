package storage_test

import (
	"context"
	"testing"

	"beads2/filesystem"
	"beads2/storage"
)

// TestFilesystemStorage runs all contract tests against the filesystem implementation.
func TestFilesystemStorage(t *testing.T) {
	storage.RunContractTests(t, func() storage.Storage {
		dir := t.TempDir()
		s := filesystem.New(dir)
		if err := s.Init(context.Background()); err != nil {
			t.Fatalf("failed to initialize storage: %v", err)
		}
		return s
	})
}

// TestFilesystemConcurrent runs concurrent access tests against the filesystem implementation.
func TestFilesystemConcurrent(t *testing.T) {
	suite := &storage.ConcurrentTestSuite{
		NewStorage: func(t *testing.T) storage.Storage {
			dir := t.TempDir()
			s := filesystem.New(dir)
			if err := s.Init(context.Background()); err != nil {
				t.Fatalf("failed to initialize storage: %v", err)
			}
			return s
		},
	}
	suite.Run(t)
}
