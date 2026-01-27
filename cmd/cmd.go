// Package cmd provides CLI commands for beads.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
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
	In      io.Reader
	JSON    bool // output in JSON format
}

// ResolveID resolves a potentially-partial issue ID to its full ID.
// It supports prefix matching (e.g., "bd-a1" can match "bd-a1b2").
func (app *App) ResolveID(ctx context.Context, partial string) (string, error) {
	// First try exact match
	_, err := app.Storage.Get(ctx, partial)
	if err == nil {
		return partial, nil
	}
	if err != storage.ErrNotFound {
		return "", err
	}

	// Try prefix matching
	// List all issues and find matches
	filter := &storage.ListFilter{}
	openIssues, err := app.Storage.List(ctx, filter)
	if err != nil {
		return "", err
	}

	closedStatus := storage.StatusClosed
	closedFilter := &storage.ListFilter{Status: &closedStatus}
	closedIssues, err := app.Storage.List(ctx, closedFilter)
	if err != nil {
		return "", err
	}

	var matches []string
	for _, issue := range openIssues {
		if strings.HasPrefix(issue.ID, partial) {
			matches = append(matches, issue.ID)
		}
	}
	for _, issue := range closedIssues {
		if strings.HasPrefix(issue.ID, partial) {
			matches = append(matches, issue.ID)
		}
	}

	if len(matches) == 0 {
		return "", storage.ErrNotFound
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous ID %q matches multiple issues: %v", partial, matches)
	}

	return matches[0], nil
}

// Confirm prompts the user for confirmation. Returns true if the user confirms.
func (app *App) Confirm(prompt string) bool {
	fmt.Fprintf(app.Out, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(app.In)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// OutputJSON outputs data as JSON if JSON mode is enabled.
func (app *App) OutputJSON(data interface{}) error {
	enc := json.NewEncoder(app.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
