package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"beads-lite/internal/agent"
	"beads-lite/internal/issuestorage"
	"beads-lite/internal/issuestorage/filesystem"
	kvfs "beads-lite/internal/kvstorage/filesystem"
)

func setupAgentTestApp(t *testing.T) (*App, *filesystem.FilesystemStorage) {
	t.Helper()
	dir := t.TempDir()
	store := filesystem.New(dir, "bd-")
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	slotStore, err := kvfs.New(dir, "slots")
	if err != nil {
		t.Fatalf("failed to create slot store: %v", err)
	}
	if err := slotStore.Init(context.Background()); err != nil {
		t.Fatalf("failed to init slot store: %v", err)
	}
	agentStore, err := kvfs.New(dir, "agents")
	if err != nil {
		t.Fatalf("failed to create agent store: %v", err)
	}
	if err := agentStore.Init(context.Background()); err != nil {
		t.Fatalf("failed to init agent store: %v", err)
	}
	return &App{
		Storage:    store,
		SlotStore:  slotStore,
		AgentStore: agentStore,
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
	}, store
}

func TestAgentStateCreates(t *testing.T) {
	app, _ := setupAgentTestApp(t)
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentStateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "running"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent state failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Set agent agent-1 state to running") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestAgentStateUpdates(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	cmd1 := newAgentStateCmd(NewTestProvider(app))
	cmd1.SetArgs([]string{"agent-1", "running"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first state failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	cmd2 := newAgentStateCmd(NewTestProvider(app))
	cmd2.SetArgs([]string{"agent-1", "working"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second state failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Set agent agent-1 state to working") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestAgentStateInvalidState(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	cmd := newAgentStateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
	if !strings.Contains(err.Error(), "invalid agent state") {
		t.Errorf("expected 'invalid agent state' in error, got: %v", err)
	}
}

func TestAgentStateJSON(t *testing.T) {
	app, _ := setupAgentTestApp(t)
	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentStateCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1", "running"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent state json failed: %v", err)
	}

	var result AgentJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if result.Agent != "agent-1" {
		t.Errorf("expected agent=agent-1, got %q", result.Agent)
	}
	if result.State != "running" {
		t.Errorf("expected state=running, got %q", result.State)
	}
	if result.LastActivity == "" {
		t.Error("expected non-empty last_activity")
	}
}

func TestAgentHeartbeat(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	// Create agent first
	stateCmd := newAgentStateCmd(NewTestProvider(app))
	stateCmd.SetArgs([]string{"agent-1", "running"})
	if err := stateCmd.Execute(); err != nil {
		t.Fatalf("state failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentHeartbeatCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Heartbeat recorded for agent agent-1") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestAgentHeartbeatNotFound(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	cmd := newAgentHeartbeatCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestAgentHeartbeatJSON(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	// Create agent first
	stateCmd := newAgentStateCmd(NewTestProvider(app))
	stateCmd.SetArgs([]string{"agent-1", "running"})
	if err := stateCmd.Execute(); err != nil {
		t.Fatalf("state failed: %v", err)
	}

	app.JSON = true
	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentHeartbeatCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("heartbeat json failed: %v", err)
	}

	var result AgentJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if result.Agent != "agent-1" {
		t.Errorf("expected agent=agent-1, got %q", result.Agent)
	}
	if result.State != "running" {
		t.Errorf("expected state=running, got %q", result.State)
	}
}

func TestAgentShow(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	// Create agent
	stateCmd := newAgentStateCmd(NewTestProvider(app))
	stateCmd.SetArgs([]string{"agent-1", "working"})
	if err := stateCmd.Execute(); err != nil {
		t.Fatalf("state failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent show failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Agent: agent-1") {
		t.Errorf("expected agent header, got: %s", output)
	}
	if !strings.Contains(output, "State: working") {
		t.Errorf("expected state, got: %s", output)
	}
	if !strings.Contains(output, "Hook:  (empty)") {
		t.Errorf("expected empty hook, got: %s", output)
	}
	if !strings.Contains(output, "Role:  (empty)") {
		t.Errorf("expected empty role, got: %s", output)
	}
}

func TestAgentShowNotFound(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	cmd := newAgentShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
}

func TestAgentShowWithSlots(t *testing.T) {
	app, issueStore := setupAgentTestApp(t)
	ctx := context.Background()

	// Create agent
	stateCmd := newAgentStateCmd(NewTestProvider(app))
	stateCmd.SetArgs([]string{"agent-1", "working"})
	if err := stateCmd.Execute(); err != nil {
		t.Fatalf("state failed: %v", err)
	}

	// Create a bead and set it as hook
	beadID, err := issueStore.Create(ctx, &issuestorage.Issue{
		Title:    "Test task",
		Type:     issuestorage.TypeTask,
		Priority: issuestorage.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("create issue failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	slotCmd := newSlotSetCmd(NewTestProvider(app))
	slotCmd.SetArgs([]string{"agent-1", "hook", beadID})
	if err := slotCmd.Execute(); err != nil {
		t.Fatalf("slot set failed: %v", err)
	}

	app.Out = &bytes.Buffer{}
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent show failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, beadID) {
		t.Errorf("expected bead ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Test task") {
		t.Errorf("expected bead title in output, got: %s", output)
	}
}

func TestAgentShowJSON(t *testing.T) {
	app, _ := setupAgentTestApp(t)

	// Create agent
	if err := agent.SetState(context.Background(), app.AgentStore, "agent-1", "working"); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	app.JSON = true
	out := app.Out.(*bytes.Buffer)

	cmd := newAgentShowCmd(NewTestProvider(app))
	cmd.SetArgs([]string{"agent-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent show json failed: %v", err)
	}

	var result AgentJSON
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if result.Agent != "agent-1" {
		t.Errorf("expected agent=agent-1, got %q", result.Agent)
	}
	if result.State != "working" {
		t.Errorf("expected state=working, got %q", result.State)
	}
}
