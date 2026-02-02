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
	store := filesystem.New(dir)
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
	store := filesystem.New(dir)
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
	store := filesystem.New(dir)
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
	store := filesystem.New(dir)
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
	store := filesystem.New(dir)
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
	store := filesystem.New(dir)
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
