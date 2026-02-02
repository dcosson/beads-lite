package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
)

// createTestGate creates a gate issue for testing and returns its ID.
func createTestGate(t *testing.T, store issuestorage.IssueStore) string {
	t.Helper()
	issue := &issuestorage.Issue{
		Title:    "Test gate",
		Type:     issuestorage.TypeGate,
		Priority: issuestorage.PriorityMedium,
		Status:   issuestorage.StatusOpen,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create test gate: %v", err)
	}
	return id
}

func TestGateWaitAddsWaiter(t *testing.T) {
	app, store := setupTestApp(t)
	gateID := createTestGate(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"wait", gateID, "--notify", "beads_lite/polecats/onyx"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate wait failed: %v", err)
	}

	if !strings.Contains(out.String(), "Added beads_lite/polecats/onyx to waiters for "+gateID) {
		t.Errorf("unexpected output: %q", out.String())
	}

	issue, _ := store.Get(context.Background(), gateID)
	if len(issue.Waiters) != 1 || issue.Waiters[0] != "beads_lite/polecats/onyx" {
		t.Errorf("expected [beads_lite/polecats/onyx], got %v", issue.Waiters)
	}
}

func TestGateAddWaiterPositional(t *testing.T) {
	app, store := setupTestApp(t)
	gateID := createTestGate(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"add-waiter", gateID, "beads_lite/crew/planning"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate add-waiter failed: %v", err)
	}

	if !strings.Contains(out.String(), "Added beads_lite/crew/planning to waiters for "+gateID) {
		t.Errorf("unexpected output: %q", out.String())
	}

	issue, _ := store.Get(context.Background(), gateID)
	if len(issue.Waiters) != 1 || issue.Waiters[0] != "beads_lite/crew/planning" {
		t.Errorf("expected [beads_lite/crew/planning], got %v", issue.Waiters)
	}
}

func TestGateWaitDedup(t *testing.T) {
	app, store := setupTestApp(t)
	gateID := createTestGate(t, store)

	// Pre-populate a waiter
	issue, _ := store.Get(context.Background(), gateID)
	issue.Waiters = []string{"beads_lite/polecats/onyx"}
	if err := store.Update(context.Background(), issue); err != nil {
		t.Fatalf("failed to seed waiter: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"wait", gateID, "--notify", "beads_lite/polecats/onyx"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate wait dedup failed: %v", err)
	}

	if !strings.Contains(out.String(), "beads_lite/polecats/onyx already waiting on "+gateID) {
		t.Errorf("expected dedup message, got %q", out.String())
	}

	issue, _ = store.Get(context.Background(), gateID)
	if len(issue.Waiters) != 1 {
		t.Errorf("expected 1 waiter after dedup, got %d", len(issue.Waiters))
	}
}

func TestGateWaitRejectsNonGate(t *testing.T) {
	app, store := setupTestApp(t)
	taskID := createTestIssue(t, store)

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"wait", taskID, "--notify", "someone"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-gate issue")
	}
	if !strings.Contains(err.Error(), "not gate") {
		t.Errorf("expected 'not gate' in error, got %q", err.Error())
	}
}

func TestGateWaitJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	gateID := createTestGate(t, store)
	out := app.Out.(*bytes.Buffer)

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"wait", gateID, "--notify", "beads_lite/polecats/onyx"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate wait JSON failed: %v", err)
	}

	var result IssueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out.String())
	}
	if result.ID != gateID {
		t.Errorf("expected ID %q, got %q", gateID, result.ID)
	}
	if len(result.Waiters) != 1 || result.Waiters[0] != "beads_lite/polecats/onyx" {
		t.Errorf("expected waiters [beads_lite/polecats/onyx], got %v", result.Waiters)
	}
}

func TestGateWaitDedupJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	gateID := createTestGate(t, store)

	// Pre-populate a waiter
	issue, _ := store.Get(context.Background(), gateID)
	issue.Waiters = []string{"beads_lite/polecats/onyx"}
	if err := store.Update(context.Background(), issue); err != nil {
		t.Fatalf("failed to seed waiter: %v", err)
	}

	out := app.Out.(*bytes.Buffer)
	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"wait", gateID, "--notify", "beads_lite/polecats/onyx"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate wait dedup JSON failed: %v", err)
	}

	var result IssueJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out.String())
	}
	if len(result.Waiters) != 1 {
		t.Errorf("expected 1 waiter after dedup, got %d", len(result.Waiters))
	}
}

func TestGateAddWaiterMultiple(t *testing.T) {
	app, store := setupTestApp(t)
	gateID := createTestGate(t, store)

	// Add first waiter
	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"add-waiter", gateID, "agent-a"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first add-waiter failed: %v", err)
	}

	// Add second waiter (need fresh buffer and command)
	app.Out = &bytes.Buffer{}
	cmd2 := newGateCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"add-waiter", gateID, "agent-b"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second add-waiter failed: %v", err)
	}

	issue, _ := store.Get(context.Background(), gateID)
	if len(issue.Waiters) != 2 {
		t.Fatalf("expected 2 waiters, got %d", len(issue.Waiters))
	}
	if issue.Waiters[0] != "agent-a" || issue.Waiters[1] != "agent-b" {
		t.Errorf("expected [agent-a, agent-b], got %v", issue.Waiters)
	}
}
