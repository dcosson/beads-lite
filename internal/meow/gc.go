package meow

import (
	"context"
	"fmt"
	"time"

	"beads-lite/internal/storage"
)

// DefaultGCOlderThan is the default age threshold for GC.
// Ephemeral issues younger than this are kept to avoid deleting
// issues currently being worked on.
const DefaultGCOlderThan = time.Hour

// GC removes ephemeral issues older than the specified duration.
// It queries all issues, filters to those with Ephemeral==true and
// CreatedAt older than opts.OlderThan, then hard-deletes each
// (no tombstones for ephemeral issues).
func GC(ctx context.Context, store storage.Storage, opts GCOptions) (*GCResult, error) {
	olderThan := opts.OlderThan
	if olderThan == 0 {
		olderThan = DefaultGCOlderThan
	}

	cutoff := time.Now().Add(-olderThan)

	// List all issues (nil fields = any).
	issues, err := store.List(ctx, &storage.ListFilter{})
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	var removed []string
	for _, issue := range issues {
		if !issue.Ephemeral {
			continue
		}
		if issue.CreatedAt.After(cutoff) {
			continue
		}
		if err := store.Delete(ctx, issue.ID); err != nil {
			return nil, fmt.Errorf("delete ephemeral issue %s: %w", issue.ID, err)
		}
		removed = append(removed, issue.ID)
	}

	return &GCResult{
		RemovedIDs: removed,
		Count:      len(removed),
	}, nil
}
