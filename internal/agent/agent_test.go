package agent

import (
	"context"
	"errors"
	"testing"

	"beads-lite/internal/kvstorage"
	kvfs "beads-lite/internal/kvstorage/filesystem"
)

func newTestStore(t *testing.T) *kvfs.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := kvfs.New(dir, "agents")
	if err != nil {
		t.Fatalf("failed to create kv store: %v", err)
	}
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init kv store: %v", err)
	}
	return store
}

func TestGetAgentNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := GetAgent(ctx, store, "agent-1")
	if !errors.Is(err, kvstorage.ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got: %v", err)
	}
}

func TestSetStateCreatesAgent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetState(ctx, store, "agent-1", StateRunning); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	a, err := GetAgent(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if a.State != StateRunning {
		t.Errorf("expected state=%s, got %q", StateRunning, a.State)
	}
	if a.LastActivity.IsZero() {
		t.Error("expected non-zero LastActivity")
	}
}

func TestSetStateUpdatesExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetState(ctx, store, "agent-1", StateRunning); err != nil {
		t.Fatalf("first SetState failed: %v", err)
	}

	if err := SetState(ctx, store, "agent-1", StateWorking); err != nil {
		t.Fatalf("second SetState failed: %v", err)
	}

	a, err := GetAgent(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if a.State != StateWorking {
		t.Errorf("expected state=%s, got %q", StateWorking, a.State)
	}
}

func TestSetStateInvalidState(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := SetState(ctx, store, "agent-1", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestValidateState(t *testing.T) {
	for _, s := range ValidStates {
		if err := ValidateState(s); err != nil {
			t.Errorf("ValidateState(%q) returned error: %v", s, err)
		}
	}
	if err := ValidateState("invalid"); err == nil {
		t.Error("expected error for invalid state")
	}
}

func TestHeartbeat(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create agent first
	if err := SetState(ctx, store, "agent-1", StateRunning); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	a1, _ := GetAgent(ctx, store, "agent-1")
	firstActivity := a1.LastActivity

	if err := Heartbeat(ctx, store, "agent-1"); err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	a2, err := GetAgent(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if a2.State != StateRunning {
		t.Errorf("heartbeat should not change state, got %q", a2.State)
	}
	if a2.LastActivity.Before(firstActivity) {
		t.Error("expected LastActivity to be updated")
	}
}

func TestHeartbeatNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := Heartbeat(ctx, store, "agent-1")
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
	if got := err.Error(); got != "agent agent-1 not found: use 'bd agent state' to create it first" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestSetStatePreservesFields(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Seed an agent record with RoleType and Rig via direct KV write
	raw := `{"state":"working","last_activity":"2025-01-01T00:00:00Z","role_type":"polecat","rig":"beads_lite"}`
	if err := store.Set(ctx, "agent-1", []byte(raw), kvstorage.SetOptions{}); err != nil {
		t.Fatalf("direct set failed: %v", err)
	}

	// SetState should preserve RoleType and Rig (read-modify-write)
	if err := SetState(ctx, store, "agent-1", StateDone); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	a, err := GetAgent(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if a.RoleType != "polecat" {
		t.Errorf("expected role_type=polecat, got %q", a.RoleType)
	}
	if a.Rig != "beads_lite" {
		t.Errorf("expected rig=beads_lite, got %q", a.Rig)
	}
	if a.State != StateDone {
		t.Errorf("expected state=done, got %q", a.State)
	}
}
