package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
)

func TestGateShowCommand(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Wait for CI",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityHigh,
		AwaitType: "gh:run",
		AwaitID:   "12345678",
		TimeoutNS: int64(30 * time.Minute),
		Waiters:   []string{"gt-mayor", "gt-deacon"},
	})
	if err != nil {
		t.Fatalf("failed to create gate issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate show command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Gate: "+id) {
		t.Errorf("expected 'Gate: %s' in output, got: %s", id, output)
	}
	if !strings.Contains(output, "Title: Wait for CI") {
		t.Errorf("expected title in output, got: %s", output)
	}
	if !strings.Contains(output, "Status: open") {
		t.Errorf("expected status in output, got: %s", output)
	}
	if !strings.Contains(output, "Await: gh:run 12345678") {
		t.Errorf("expected await info in output, got: %s", output)
	}
	if !strings.Contains(output, "Timeout: 30m0s") {
		t.Errorf("expected timeout in output, got: %s", output)
	}
	if !strings.Contains(output, "Waiters: gt-mayor, gt-deacon") {
		t.Errorf("expected waiters in output, got: %s", output)
	}
	if !strings.Contains(output, "Created:") {
		t.Errorf("expected created timestamp in output, got: %s", output)
	}
}

func TestGateShowNotGateType(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Regular Task",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-gate issue, got nil")
	}
	if !strings.Contains(err.Error(), "not \"gate\"") {
		t.Errorf("expected type error message, got: %s", err.Error())
	}
}

func TestGateShowNotFound(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent gate, got nil")
	}
	if !strings.Contains(err.Error(), "no issue found") {
		t.Errorf("expected 'no issue found' error, got: %s", err.Error())
	}
}

func TestGateShowPrefixMatch(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "Prefix Gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityMedium,
		AwaitType: "timer",
		TimeoutNS: int64(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to create gate issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	prefix := id[:4]
	cmd := newGateShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{prefix})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate show with prefix failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Gate: "+id) {
		t.Errorf("expected full ID in output, got: %s", output)
	}
}

func TestGateShowJSON(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:     "JSON Gate",
		Type:      issuestorage.TypeGate,
		Priority:  issuestorage.PriorityHigh,
		AwaitType: "gh:pr",
		AwaitID:   "99",
		TimeoutNS: int64(10 * time.Minute),
		Waiters:   []string{"gt-witness"},
	})
	if err != nil {
		t.Fatalf("failed to create gate issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    true,
	}

	cmd := newGateShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate show JSON failed: %v", err)
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r["id"] != id {
		t.Errorf("expected ID %s, got %v", id, r["id"])
	}
	if r["issue_type"] != "gate" {
		t.Errorf("expected issue_type 'gate', got %v", r["issue_type"])
	}
	if r["await_type"] != "gh:pr" {
		t.Errorf("expected await_type 'gh:pr', got %v", r["await_type"])
	}
	if r["await_id"] != "99" {
		t.Errorf("expected await_id '99', got %v", r["await_id"])
	}
	// JSON numbers decode as float64
	if r["timeout_ns"] != float64(int64(10*time.Minute)) {
		t.Errorf("expected timeout_ns %v, got %v", int64(10*time.Minute), r["timeout_ns"])
	}
	waiters, ok := r["waiters"].([]interface{})
	if !ok || len(waiters) != 1 || waiters[0] != "gt-witness" {
		t.Errorf("expected waiters [gt-witness], got %v", r["waiters"])
	}
}

func TestGateShowMinimalFields(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	id, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Minimal Gate",
		Type:     issuestorage.TypeGate,
		Priority: issuestorage.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("failed to create gate issue: %v", err)
	}

	var out bytes.Buffer
	app := &App{
		Storage: store,
		Out:     &out,
		JSON:    false,
	}

	cmd := newGateShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate show minimal failed: %v", err)
	}

	output := out.String()
	// Should have gate header and basic fields
	if !strings.Contains(output, "Gate: "+id) {
		t.Errorf("expected gate header, got: %s", output)
	}
	// Should NOT have await/timeout/waiters lines when not set
	if strings.Contains(output, "Await:") {
		t.Errorf("expected no Await line for minimal gate, got: %s", output)
	}
	if strings.Contains(output, "Timeout:") {
		t.Errorf("expected no Timeout line for minimal gate, got: %s", output)
	}
	if strings.Contains(output, "Waiters:") {
		t.Errorf("expected no Waiters line for minimal gate, got: %s", output)
	}
}

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

func TestGateListCommand_DefaultListsOpenGates(t *testing.T) {
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
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
	store := filesystem.New(dir, "bd-")
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
	store := filesystem.New(dir, "bd-")
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

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
	store := filesystem.New(dir, "bd-")
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
	store := filesystem.New(dir, "bd-")
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

	if !strings.Contains(output, "ID") {
		t.Errorf("expected table header with 'ID', got: %s", output)
	}
	if !strings.Contains(output, "Await") {
		t.Errorf("expected table header with 'Await', got: %s", output)
	}
	if !strings.Contains(output, "Waiters") {
		t.Errorf("expected table header with 'Waiters', got: %s", output)
	}
	if !strings.Contains(output, "Wait for CI") {
		t.Errorf("expected gate title in output, got: %s", output)
	}
	if !strings.Contains(output, "gh:run") {
		t.Errorf("expected await_type 'gh:run' in output, got: %s", output)
	}
	if !strings.Contains(output, "12345678") {
		t.Errorf("expected await_id '12345678' in output, got: %s", output)
	}
	if !strings.Contains(output, "2") {
		t.Errorf("expected waiter count '2' in output, got: %s", output)
	}
}

func TestGateResolveBasic(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	gate := &issuestorage.Issue{
		Title:    "Wait for CI",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeGate,
	}
	id, err := store.Create(context.Background(), gate)
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve", id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate resolve failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Resolved gate "+id) {
		t.Errorf("expected output to contain 'Resolved gate %s', got %q", id, output)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get gate: %v", err)
	}
	if got.Status != issuestorage.StatusClosed {
		t.Errorf("expected status %q, got %q", issuestorage.StatusClosed, got.Status)
	}
	if got.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
}

func TestGateResolveWithReason(t *testing.T) {
	app, store := setupTestApp(t)
	out := app.Out.(*bytes.Buffer)

	gate := &issuestorage.Issue{
		Title:    "Wait for approval",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeGate,
	}
	id, err := store.Create(context.Background(), gate)
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve", "--reason", "CI passed", id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate resolve --reason failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Resolved gate "+id) {
		t.Errorf("expected output to contain 'Resolved gate %s', got %q", id, output)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get gate: %v", err)
	}
	if got.CloseReason != "CI passed" {
		t.Errorf("expected close_reason %q, got %q", "CI passed", got.CloseReason)
	}
}

func TestGateResolveJSON(t *testing.T) {
	app, store := setupTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	gate := &issuestorage.Issue{
		Title:     "Wait for deploy",
		Status:    issuestorage.StatusOpen,
		Priority:  issuestorage.PriorityMedium,
		Type:      issuestorage.TypeGate,
		AwaitType: "gh:run",
		AwaitID:   "12345",
	}
	id, err := store.Create(context.Background(), gate)
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve", "--reason", "Deploy succeeded", id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate resolve JSON failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["id"].(string) != id {
		t.Errorf("expected id %q, got %q", id, result["id"])
	}
	if result["status"].(string) != "closed" {
		t.Errorf("expected status %q, got %q", "closed", result["status"])
	}
}

func TestGateResolveNotAGate(t *testing.T) {
	app, store := setupTestApp(t)

	issue := &issuestorage.Issue{
		Title:    "Regular task",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeTask,
	}
	id, err := store.Create(context.Background(), issue)
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve", id})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when resolving a non-gate issue")
	}
	if !strings.Contains(err.Error(), "not a gate") {
		t.Errorf("expected error to mention 'not a gate', got %q", err.Error())
	}
}

func TestGateResolveAlreadyClosed(t *testing.T) {
	app, store := setupTestApp(t)

	gate := &issuestorage.Issue{
		Title:    "Already resolved gate",
		Status:   issuestorage.StatusOpen,
		Priority: issuestorage.PriorityMedium,
		Type:     issuestorage.TypeGate,
	}
	id, err := store.Create(context.Background(), gate)
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	if err := store.Close(context.Background(), id); err != nil {
		t.Fatalf("failed to close gate: %v", err)
	}

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve", id})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when resolving already-closed gate")
	}
	if !strings.Contains(err.Error(), "already closed") {
		t.Errorf("expected error to mention 'already closed', got %q", err.Error())
	}
}

func TestGateResolveNonExistent(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve", "bd-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent gate")
	}
}

func TestGateResolveNoArgs(t *testing.T) {
	app, _ := setupTestApp(t)

	cmd := newGateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"resolve"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arguments provided")
	}
}
