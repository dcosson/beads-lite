package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func TestGateListCommand_DefaultListsOpenGates(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create an open gate
	openGateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Wait for CI",
		Priority:  issuestorage.PriorityHigh,
		Type:      issuestorage.TypeGate,
		AwaitType: "gh:run",
		AwaitID:   "12345678",
		Waiters:   []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	// Create a closed gate
	closedGateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Old gate",
		Priority:  issuestorage.PriorityMedium,
		Type:      issuestorage.TypeGate,
		AwaitType: "human",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}
	if err := store.Close(ctx, closedGateID); err != nil {
		t.Fatalf("failed to close gate: %v", err)
	}

	// Create a non-gate issue (should not appear)
	taskID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Regular task",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, openGateID) {
		t.Errorf("expected output to contain open gate %s, got: %s", openGateID, output)
	}
	if strings.Contains(output, closedGateID) {
		t.Errorf("expected output NOT to contain closed gate %s, got: %s", closedGateID, output)
	}
	if strings.Contains(output, taskID) {
		t.Errorf("expected output NOT to contain task %s, got: %s", taskID, output)
	}
}

func TestGateListCommand_AllFlag(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	openGateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Open gate",
		Priority:  issuestorage.PriorityHigh,
		Type:      issuestorage.TypeGate,
		AwaitType: "gh:run",
		AwaitID:   "99999",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	closedGateID, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Closed gate",
		Priority:  issuestorage.PriorityMedium,
		Type:      issuestorage.TypeGate,
		AwaitType: "human",
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}
	if err := store.Close(ctx, closedGateID); err != nil {
		t.Fatalf("failed to close gate: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"list", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate list --all command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, openGateID) {
		t.Errorf("expected output to contain open gate %s, got: %s", openGateID, output)
	}
	if !strings.Contains(output, closedGateID) {
		t.Errorf("expected output to contain closed gate %s, got: %s", closedGateID, output)
	}
}

func TestGateListCommand_NoGates(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Create a non-gate issue
	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Regular task",
		Priority: issuestorage.PriorityHigh,
		Type:     issuestorage.TypeTask,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No gates found.") {
		t.Errorf("expected 'No gates found.' message, got: %s", output)
	}
}

func TestGateListCommand_JSON(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Wait for CI",
		Priority:  issuestorage.PriorityHigh,
		Type:      issuestorage.TypeGate,
		AwaitType: "gh:run",
		AwaitID:   "12345678",
		Waiters:   []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate list JSON command failed: %v", err)
	}

	output := out.String()
	var gates []GateListJSON
	if err := json.Unmarshal([]byte(output), &gates); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v, output: %s", err, output)
	}
	if len(gates) != 1 {
		t.Fatalf("expected 1 gate in JSON output, got %d", len(gates))
	}
	if gates[0].Title != "Wait for CI" {
		t.Errorf("expected title 'Wait for CI', got '%s'", gates[0].Title)
	}
	if gates[0].AwaitType != "gh:run" {
		t.Errorf("expected await_type 'gh:run', got '%s'", gates[0].AwaitType)
	}
	if gates[0].AwaitID != "12345678" {
		t.Errorf("expected await_id '12345678', got '%s'", gates[0].AwaitID)
	}
	if gates[0].Status != "open" {
		t.Errorf("expected status 'open', got '%s'", gates[0].Status)
	}
	if len(gates[0].Waiters) != 2 {
		t.Errorf("expected 2 waiters, got %d", len(gates[0].Waiters))
	}
}

func TestGateListCommand_TextTableOutput(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir)
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	_, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Wait for CI",
		Priority:  issuestorage.PriorityHigh,
		Type:      issuestorage.TypeGate,
		AwaitType: "gh:run",
		AwaitID:   "12345678",
		Waiters:   []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate list command failed: %v", err)
	}

	output := out.String()

	// Verify table header
	if !strings.Contains(output, "ID") {
		t.Errorf("expected table header with 'ID', got: %s", output)
	}
	if !strings.Contains(output, "Await") {
		t.Errorf("expected table header with 'Await', got: %s", output)
	}
	if !strings.Contains(output, "Waiters") {
		t.Errorf("expected table header with 'Waiters', got: %s", output)
	}

	// Verify gate data
	if !strings.Contains(output, "Wait for CI") {
		t.Errorf("expected gate title in output, got: %s", output)
	}
	if !strings.Contains(output, "gh:run") {
		t.Errorf("expected await_type 'gh:run' in output, got: %s", output)
	}
	if !strings.Contains(output, "12345678") {
		t.Errorf("expected await_id '12345678' in output, got: %s", output)
	}
	// Waiter count should be 2
	if !strings.Contains(output, "2") {
		t.Errorf("expected waiter count '2' in output, got: %s", output)
	}
}
