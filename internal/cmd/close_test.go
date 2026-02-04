package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

func TestCloseBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create an issue to close
	issue := &issuestorage.Issue{
		Title:    "Issue to close",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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
	if got.Status != issuestorage.StatusClosed {
		t.Errorf("expected status %q, got %q", issuestorage.StatusClosed, got.Status)
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
		issue := &issuestorage.Issue{
			Title:    "Issue " + string(rune('A'+i)),
			Status:   issuestorage.StatusOpen,
			Priority: issuestorage.PriorityMedium,
			Type:     issuestorage.TypeTask,
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
		if got.Status != issuestorage.StatusClosed {
			t.Errorf("issue %s: expected status %q, got %q", id, issuestorage.StatusClosed, got.Status)
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
	issue := &issuestorage.Issue{
		Title:    "Valid issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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
	if got.Status != issuestorage.StatusClosed {
		t.Errorf("expected status %q, got %q", issuestorage.StatusClosed, got.Status)
	}
}

func TestCloseWithJSONOutput(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create an issue
	issue := &issuestorage.Issue{
		Title:    "Issue for JSON test",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
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

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 closed issue, got %d", len(result))
	}
	if result[0]["id"].(string) != id {
		t.Errorf("expected id to be %q, got %q", id, result[0]["id"])
	}
	if result[0]["status"].(string) != "closed" {
		t.Errorf("expected status to be closed, got %q", result[0]["status"])
	}
}

func TestCloseJSONWithErrors(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	// Create one valid issue
	issue := &issuestorage.Issue{
		Title:    "Valid issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	validID, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{validID, "bd-nonexistent"})
	_ = cmd.Execute() // Ignore error, we want to check JSON output

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Only the valid issue should be in the output (nonexistent one errored)
	if len(result) != 1 {
		t.Fatalf("expected 1 closed issue, got %d", len(result))
	}
	if result[0]["id"].(string) != validID {
		t.Errorf("expected id to be %q, got %q", validID, result[0]["id"])
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
	issue := &issuestorage.Issue{
		Title:    "Already closed",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	if err := store.Modify(context.Background(), id, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	// Closing again via Modify is idempotent — should succeed without error
	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	err = cmd.Execute()
	if err != nil {
		t.Errorf("closing already-closed issue should succeed (idempotent), got: %v", err)
	}
}

// setupMolecule creates a root epic with 3 child steps (A→B→C chain).
// Each step is a child of root via parent-child, and B blocks-depends on A, C blocks-depends on B.
// Returns root ID, step A ID, step B ID, step C ID.
func setupMolecule(t *testing.T, store issuestorage.IssueStore) (string, string, string, string) {
	t.Helper()
	ctx := context.Background()

	root := &issuestorage.Issue{
		Title:    "Root Epic",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeEpic,
	}
	rootID, err := store.Create(ctx, root)
	if err != nil {
		t.Fatalf("failed to create root: %v", err)
	}

	stepA := &issuestorage.Issue{
		Title:    "Step A",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idA, err := store.Create(ctx, stepA)
	if err != nil {
		t.Fatalf("failed to create step A: %v", err)
	}

	stepB := &issuestorage.Issue{
		Title:    "Step B",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idB, err := store.Create(ctx, stepB)
	if err != nil {
		t.Fatalf("failed to create step B: %v", err)
	}

	stepC := &issuestorage.Issue{
		Title:    "Step C",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	idC, err := store.Create(ctx, stepC)
	if err != nil {
		t.Fatalf("failed to create step C: %v", err)
	}

	// Set parent-child relationships
	if err := store.AddDependency(ctx, idA, rootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child A->root: %v", err)
	}
	if err := store.AddDependency(ctx, idB, rootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child B->root: %v", err)
	}
	if err := store.AddDependency(ctx, idC, rootID, issuestorage.DepTypeParentChild); err != nil {
		t.Fatalf("failed to add parent-child C->root: %v", err)
	}

	// Set blocks dependencies: B depends on A, C depends on B
	if err := store.AddDependency(ctx, idB, idA, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add blocks B->A: %v", err)
	}
	if err := store.AddDependency(ctx, idC, idB, issuestorage.DepTypeBlocks); err != nil {
		t.Fatalf("failed to add blocks C->B: %v", err)
	}

	return rootID, idA, idB, idC
}

func TestCloseContinueAdvancesNextStep(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	_, idA, idB, _ := setupMolecule(t, store)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Closed "+idA) {
		t.Errorf("expected output to contain 'Closed %s', got %q", idA, output)
	}
	if !strings.Contains(output, "Advanced to "+idB) {
		t.Errorf("expected output to contain 'Advanced to %s', got %q", idB, output)
	}

	// Verify step B is now in_progress
	got, err := store.Get(context.Background(), idB)
	if err != nil {
		t.Fatalf("failed to get step B: %v", err)
	}
	if got.Status != issuestorage.StatusInProgress {
		t.Errorf("expected step B status %q, got %q", issuestorage.StatusInProgress, got.Status)
	}
}

func TestCloseContinueNoAuto(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	_, idA, idB, _ := setupMolecule(t, store)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", "--no-auto", idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue --no-auto failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Next step: "+idB) {
		t.Errorf("expected output to contain 'Next step: %s', got %q", idB, output)
	}

	// Verify step B is still open (not claimed)
	got, err := store.Get(context.Background(), idB)
	if err != nil {
		t.Fatalf("failed to get step B: %v", err)
	}
	if got.Status != issuestorage.StatusOpen {
		t.Errorf("expected step B status %q, got %q", issuestorage.StatusOpen, got.Status)
	}
}

func TestCloseSuggestNext(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	_, idA, idB, _ := setupMolecule(t, store)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--suggest-next", idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --suggest-next failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Unblocked: "+idB) {
		t.Errorf("expected output to contain 'Unblocked: %s', got %q", idB, output)
	}
}

func TestCloseContinueNonMolecule(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	// Create a standalone issue (no parent)
	issue := &issuestorage.Issue{
		Title:    "Standalone issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue on standalone issue failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Closed "+id) {
		t.Errorf("expected output to contain 'Closed %s', got %q", id, output)
	}
	// Should print "No more steps" since it's not part of a molecule
	if !strings.Contains(output, "No more steps") {
		t.Errorf("expected output to contain 'No more steps', got %q", output)
	}
}

func TestCloseContinueJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)
	_, idA, idB, _ := setupMolecule(t, store)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue (JSON) failed: %v", err)
	}

	var result CloseWithContinueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out.String())
	}

	if len(result.Closed) != 1 {
		t.Fatalf("expected 1 closed issue, got %d", len(result.Closed))
	}
	if result.Closed[0].ID != idA {
		t.Errorf("expected closed id %q, got %q", idA, result.Closed[0].ID)
	}
	if result.Continue == nil {
		t.Fatal("expected continue block, got nil")
	}
	if !result.Continue.AutoAdvanced {
		t.Error("expected auto_advanced to be true")
	}
	if result.Continue.NextStep == nil {
		t.Fatal("expected next_step, got nil")
	}
	if result.Continue.NextStep.ID != idB {
		t.Errorf("expected next_step id %q, got %q", idB, result.Continue.NextStep.ID)
	}
	if result.Continue.MoleculeComplete {
		t.Error("expected molecule_complete to be false")
	}

	// Verify step B is now in_progress
	got, err := store.Get(context.Background(), idB)
	if err != nil {
		t.Fatalf("failed to get step B: %v", err)
	}
	if got.Status != issuestorage.StatusInProgress {
		t.Errorf("expected step B status %q, got %q", issuestorage.StatusInProgress, got.Status)
	}
}

func TestCloseContinueNoAutoJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)
	_, idA, idB, _ := setupMolecule(t, store)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", "--no-auto", idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue --no-auto (JSON) failed: %v", err)
	}

	var result CloseWithContinueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out.String())
	}

	if result.Continue == nil {
		t.Fatal("expected continue block, got nil")
	}
	if result.Continue.AutoAdvanced {
		t.Error("expected auto_advanced to be false")
	}
	if result.Continue.NextStep == nil {
		t.Fatal("expected next_step, got nil")
	}
	if result.Continue.NextStep.ID != idB {
		t.Errorf("expected next_step id %q, got %q", idB, result.Continue.NextStep.ID)
	}

	// Verify step B is still open (not auto-advanced)
	got, err := store.Get(context.Background(), idB)
	if err != nil {
		t.Fatalf("failed to get step B: %v", err)
	}
	if got.Status != issuestorage.StatusOpen {
		t.Errorf("expected step B status %q, got %q", issuestorage.StatusOpen, got.Status)
	}
}

func TestCloseContinueJSONNonMolecule(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	issue := &issuestorage.Issue{
		Title:    "Standalone issue",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue (JSON, non-molecule) failed: %v", err)
	}

	var result CloseWithContinueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out.String())
	}

	if result.Continue == nil {
		t.Fatal("expected continue block, got nil")
	}
	if result.Continue.NextStep != nil {
		t.Errorf("expected next_step to be nil for non-molecule, got %+v", result.Continue.NextStep)
	}
	if !result.Continue.MoleculeComplete {
		t.Error("expected molecule_complete to be true for non-molecule")
	}
}

func TestCloseSuggestNextJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)
	_, idA, _, _ := setupMolecule(t, store)

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--suggest-next", idA})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --suggest-next (JSON) failed: %v", err)
	}

	// The JSON output for suggest-next is a plain []IssueJSON (not wrapped)
	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out.String())
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 closed issue in output, got %d", len(result))
	}
	if result[0]["id"].(string) != idA {
		t.Errorf("expected id %q, got %q", idA, result[0]["id"])
	}
}

func TestCloseContinueEndOfMolecule(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)
	ctx := context.Background()
	_, idA, idB, idC := setupMolecule(t, store)

	// Close A and B first so C is the last step
	if err := store.Modify(ctx, idA, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("failed to close A: %v", err)
	}
	if err := store.Modify(ctx, idB, func(i *issuestorage.Issue) error {
		i.Status = issuestorage.StatusClosed
		return nil
	}); err != nil {
		t.Fatalf("failed to close B: %v", err)
	}

	cmd := newCloseCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"--continue", idC})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("close --continue on last step failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Closed "+idC) {
		t.Errorf("expected output to contain 'Closed %s', got %q", idC, output)
	}
	if !strings.Contains(output, "No more steps") {
		t.Errorf("expected output to contain 'No more steps', got %q", output)
	}
}
