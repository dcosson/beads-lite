package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/storage"
)

func TestCloseBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create an issue to close
	issue := &storage.Issue{
		Title:    "Issue to close",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Closed "+id) {
		t.Errorf("expected output to contain 'Closed %s', got %q", id, output)
	}

	// Verify issue was closed
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if got.Status != storage.StatusClosed {
		t.Errorf("expected status %q, got %q", storage.StatusClosed, got.Status)
	}
	if got.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
}

func TestCloseMultiple(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create multiple issues
	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		issue := &storage.Issue{
			Title:    "Issue " + string(rune('A'+i)),
			Status:   storage.StatusOpen,
			Priority: storage.PriorityMedium,
			Type:     storage.TypeTask,
		}
		id, err := store.Create(context.Background(), issue)
		if err != nil {
			t.Fatalf("failed to create issue %d: %v", i, err)
		}
		ids[i] = id
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs(ids)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	output := out.String()
	for _, id := range ids {
		if !strings.Contains(output, "Closed "+id) {
			t.Errorf("expected output to contain 'Closed %s'", id)
		}
	}

	// Verify all issues were closed
	for _, id := range ids {
		got, err := store.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("failed to get issue %s: %v", id, err)
		}
		if got.Status != storage.StatusClosed {
			t.Errorf("issue %s: expected status %q, got %q", id, storage.StatusClosed, got.Status)
		}
	}
}

func TestCloseNonExistent(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent issue")
	}
}

func TestClosePartialFailure(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	errOut := app.Err.(*bytes.Buffer)

	// Create one valid issue
	issue := &storage.Issue{
		Title:    "Valid issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	validID, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{validID, "bd-nonexistent"})
	err = cmd.Execute()

	// Should return an error (for the non-existent one)
	if err == nil {
		t.Error("expected error for partial failure")
	}

	// But the valid one should still be closed
	output := out.String()
	if !strings.Contains(output, "Closed "+validID) {
		t.Errorf("expected output to contain 'Closed %s'", validID)
	}

	// Error should be reported
	errOutput := errOut.String()
	if !strings.Contains(errOutput, "bd-nonexistent") {
		t.Errorf("expected error output to mention non-existent issue, got %q", errOutput)
	}

	// Verify the valid issue was closed
	got, err := store.Get(context.Background(), validID)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if got.Status != storage.StatusClosed {
		t.Errorf("expected status %q, got %q", storage.StatusClosed, got.Status)
	}
}

func TestCloseWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create an issue
	issue := &storage.Issue{
		Title:    "Issue for JSON test",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	closed, ok := result["closed"].([]interface{})
	if !ok {
		t.Fatalf("expected closed to be an array, got %T", result["closed"])
	}
	if len(closed) != 1 {
		t.Errorf("expected 1 closed issue, got %d", len(closed))
	}
	if closed[0].(string) != id {
		t.Errorf("expected closed[0] to be %q, got %q", id, closed[0])
	}
}

func TestCloseJSONWithErrors(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create one valid issue
	issue := &storage.Issue{
		Title:    "Valid issue",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	validID, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{validID, "bd-nonexistent"})
	_ = cmd.Execute() // Ignore error, we want to check JSON output

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Check closed array
	closed, ok := result["closed"].([]interface{})
	if !ok {
		t.Fatalf("expected closed to be an array, got %T", result["closed"])
	}
	if len(closed) != 1 || closed[0].(string) != validID {
		t.Errorf("expected closed to contain %q, got %v", validID, closed)
	}

	// Check errors array
	errors, ok := result["errors"].([]interface{})
	if !ok {
		t.Fatalf("expected errors to be an array, got %T", result["errors"])
	}
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestCloseNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}
}

func TestCloseAlreadyClosed(t *testing.T) {
	app, store := setupTestApp(t)

	// Create and close an issue
	issue := &storage.Issue{
		Title:    "Already closed",
		Status:   storage.StatusOpen,
		Priority: storage.PriorityMedium,
		Type:     storage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Close(context.Background(), id); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Try to close again - should return an error since file is no longer in open/
	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when closing already-closed issue")
	}
}
