package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	kvfs "beads-lite/internal/kvstorage/filesystem"
	"beads-lite/internal/issueservice"
)

func setupSlotTestApp(t *testing.T) (*App, *issueservice.IssueStore) {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	rs := issueservice.New(nil, store)
	slotStore, err := kvfs.New(dir, "slots")
	if err != nil {
		t.Fatalf("failed to create slot store: %v", err)
	}
	if err := slotStore.Init(context.Background()); err != nil {
		t.Fatalf("failed to init slot store: %v", err)
	}
	return &App{
		Storage:   rs,
		SlotStore: slotStore,
		Out:       &bytes.Buffer{},
		Err:       &bytes.Buffer{},
	}, rs
}

func TestSlotShowEmpty(t *testing.T) {
	app, _ := setupSlotTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newSlotShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("slot show failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Agent: agent-1") {
		t.Errorf("expected agent header, got: %s", output)
	}
	if !strings.Contains(output, "Hook:  (empty)") {
		t.Errorf("expected empty hook, got: %s", output)
	}
	if !strings.Contains(output, "Role:  (empty)") {
		t.Errorf("expected empty role, got: %s", output)
	}
}

func TestSlotShowJSON(t *testing.T) {
	app, _ := setupSlotTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newSlotShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("slot show json failed: %v", err)
	}

	var result SlotJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if result.Agent != "agent-1" {
		t.Errorf("expected agent=agent-1, got %q", result.Agent)
	}
}

func TestSlotSetHook(t *testing.T) {
	app, store := setupSlotTestApp(t)
	out := app.Out.(*bytes.Buffer)
	ctx := context.Background()

	// Create a bead to hook
	beadID, err := store.Create(ctx, &issuestorage.Issue{
		Title:    "Fix login bug",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}

	cmd := newSlotSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "hook", beadID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("slot set failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Set hook on agent-1 to "+beadID) {
		t.Errorf("unexpected output: %s", output)
	}

	// Verify the bead's status was set to hooked (GUPP)
	issue, err := store.Get(ctx, beadID)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Status != issuestorage.StatusHooked {
		t.Errorf("expected status=hooked, got %q", issue.Status)
	}
}

func TestSlotSetHookOccupied(t *testing.T) {
	app, store := setupSlotTestApp(t)
	ctx := context.Background()

	bead1, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "First", Type: issuestorage.TypeTask, Priority: issuestorage.PriorityHigh,
	})
	bead2, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "Second", Type: issuestorage.TypeTask, Priority: issuestorage.PriorityHigh,
	})

	cmd1 := newSlotSetCmd(NewTestProvider(app))
	cmd1.SetArgs([]string{"agent-1", "hook", bead1})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first set failed: %v", err)
	}

	// Reset output buffer
	app.Out = &bytes.Buffer{}
	cmd2 := newSlotSetCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"agent-1", "hook", bead2})
	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected error when hook occupied")
	}
	if !strings.Contains(err.Error(), "occupied") {
		t.Errorf("expected 'occupied' in error, got: %v", err)
	}
}

func TestSlotSetInvalidSlotName(t *testing.T) {
	app, _ := setupSlotTestApp(t)

	cmd := newSlotSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "bogus", "bl-xyz"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid slot name")
	}
	if !strings.Contains(err.Error(), "invalid slot") {
		t.Errorf("expected 'invalid slot' in error, got: %v", err)
	}
}

func TestSlotSetNonExistentBead(t *testing.T) {
	app, _ := setupSlotTestApp(t)

	cmd := newSlotSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "hook", "bl-nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent bead")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestSlotSetJSON(t *testing.T) {
	app, store := setupSlotTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)
	ctx := context.Background()

	beadID, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "Test", Type: issuestorage.TypeTask, Priority: issuestorage.PriorityHigh,
	})

	cmd := newSlotSetCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "hook", beadID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("slot set json failed: %v", err)
	}

	var result SlotJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if result.Hook != beadID {
		t.Errorf("expected hook=%s, got %q", beadID, result.Hook)
	}
}

func TestSlotClear(t *testing.T) {
	app, store := setupSlotTestApp(t)
	ctx := context.Background()

	beadID, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "Test", Type: issuestorage.TypeTask, Priority: issuestorage.PriorityHigh,
	})

	// Set the hook first
	setCmd := newSlotSetCmd(NewTestProvider(app))
	setCmd.SetArgs([]string{"agent-1", "hook", beadID})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	clearCmd := newSlotClearCmd(NewTestProvider(app))
	clearCmd.SetArgs([]string{"agent-1", "hook"})
	if err := clearCmd.Execute(); err != nil {
		t.Fatalf("slot clear failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Cleared hook on agent-1") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestSlotClearAlreadyEmpty(t *testing.T) {
	app, _ := setupSlotTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newSlotClearCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "hook"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("slot clear failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "is already empty") {
		t.Errorf("expected 'already empty' message, got: %s", output)
	}
}

func TestSlotClearInvalidSlotName(t *testing.T) {
	app, _ := setupSlotTestApp(t)

	cmd := newSlotClearCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid slot name")
	}
}

func TestSlotClearJSON(t *testing.T) {
	app, store := setupSlotTestApp(t)
	ctx := context.Background()

	beadID, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "Test", Type: issuestorage.TypeTask, Priority: issuestorage.PriorityHigh,
	})

	// Set hook first
	setCmd := newSlotSetCmd(NewTestProvider(app))
	setCmd.SetArgs([]string{"agent-1", "hook", beadID})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Clear with JSON output
	app.JSON = true
	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	clearCmd := newSlotClearCmd(NewTestProvider(app))
	clearCmd.SetArgs([]string{"agent-1", "hook"})
	if err := clearCmd.Execute(); err != nil {
		t.Fatalf("slot clear json failed: %v", err)
	}

	var result SlotJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if result.Hook != "" {
		t.Errorf("expected empty hook after clear, got %q", result.Hook)
	}
}

func TestSlotShowWithTitles(t *testing.T) {
	app, store := setupSlotTestApp(t)
	ctx := context.Background()

	beadID, _ := store.Create(ctx, &issuestorage.Issue{
		Title: "Fix login bug", Type: issuestorage.TypeTask, Priority: issuestorage.PriorityHigh,
	})

	setCmd := newSlotSetCmd(NewTestProvider(app))
	setCmd.SetArgs([]string{"agent-1", "hook", beadID})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	showCmd := newSlotShowCmd(NewTestProvider(app))
	showCmd.SetArgs([]string{"agent-1"})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("slot show failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, beadID) {
		t.Errorf("expected bead ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Fix login bug") {
		t.Errorf("expected bead title in output, got: %s", output)
	}
}
