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
		fs := filesystem.New(dir, "bd-")
		if err := fs.Init(context.Background()); err != nil {
			t.Fatalf("Init failed: %v", err)
		}
		return fs
	}
	issuestorage.RunContractTests(t, factory)
}
