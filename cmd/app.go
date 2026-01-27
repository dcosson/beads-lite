// Package cmd implements the CLI commands for beads.
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"beads2/storage"
)

// App holds application state shared across commands.
type App struct {
	Storage storage.Storage
	Out     io.Writer
	Err     io.Writer
	JSON    bool
}

// ResolveID resolves an issue ID, supporting prefix matching.
// If id is an exact match, returns that issue.
// If id is a prefix matching exactly one issue, returns that issue.
// Returns ErrAmbiguousID if prefix matches multiple issues.
// Returns ErrNotFound if no issues match.
func (app *App) ResolveID(ctx context.Context, id string) (*storage.Issue, error) {
	// Try exact match first
	issue, err := app.Storage.Get(ctx, id)
	if err == nil {
		return issue, nil
	}
	if err != storage.ErrNotFound {
		return nil, err
	}

	// Try prefix matching - list all issues and filter
	// We need to check both open and closed issues
	openFilter := &storage.ListFilter{}
	openIssues, err := app.Storage.List(ctx, openFilter)
	if err != nil {
		return nil, err
	}

	closedStatus := storage.StatusClosed
	closedFilter := &storage.ListFilter{Status: &closedStatus}
	closedIssues, err := app.Storage.List(ctx, closedFilter)
	if err != nil {
		return nil, err
	}

	var matches []*storage.Issue
	for _, issue := range openIssues {
		if strings.HasPrefix(issue.ID, id) {
			matches = append(matches, issue)
		}
	}
	for _, issue := range closedIssues {
		if strings.HasPrefix(issue.ID, id) {
			matches = append(matches, issue)
		}
	}

	switch len(matches) {
	case 0:
		return nil, storage.ErrNotFound
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return nil, fmt.Errorf("%w: %s matches %v", storage.ErrAmbiguousID, id, ids)
	}
}
