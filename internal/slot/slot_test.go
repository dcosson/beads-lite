package slot

import (
	"context"
	"testing"

	kvfs "beads-lite/internal/kvstorage/filesystem"
)

func newTestStore(t *testing.T) *kvfs.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := kvfs.New(dir, "slots")
	if err != nil {
		t.Fatalf("failed to create kv store: %v", err)
	}
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init kv store: %v", err)
	}
	return store
}

func TestGetSlotsEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rec, err := GetSlots(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Hook != "" || rec.Role != "" {
		t.Errorf("expected empty record, got hook=%q role=%q", rec.Hook, rec.Role)
	}
}

func TestSetSlotHook(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetSlot(ctx, store, "agent-1", "hook", "bl-abc"); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}

	rec, err := GetSlots(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetSlots failed: %v", err)
	}
	if rec.Hook != "bl-abc" {
		t.Errorf("expected hook=bl-abc, got %q", rec.Hook)
	}
}

func TestSetSlotHookCardinality(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetSlot(ctx, store, "agent-1", "hook", "bl-abc"); err != nil {
		t.Fatalf("first SetSlot failed: %v", err)
	}

	err := SetSlot(ctx, store, "agent-1", "hook", "bl-def")
	if err == nil {
		t.Fatal("expected error on second hook set, got nil")
	}
	if got := err.Error(); got != "hook slot on agent-1 is occupied by bl-abc; use 'bd slot clear' first" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestSetSlotRole(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetSlot(ctx, store, "agent-1", "role", "bl-role1"); err != nil {
		t.Fatalf("SetSlot role failed: %v", err)
	}

	// Overwriting role should succeed
	if err := SetSlot(ctx, store, "agent-1", "role", "bl-role2"); err != nil {
		t.Fatalf("SetSlot role overwrite failed: %v", err)
	}

	rec, err := GetSlots(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetSlots failed: %v", err)
	}
	if rec.Role != "bl-role2" {
		t.Errorf("expected role=bl-role2, got %q", rec.Role)
	}
}

func TestSetSlotInvalidName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := SetSlot(ctx, store, "agent-1", "bogus", "bl-xyz")
	if err == nil {
		t.Fatal("expected error for invalid slot name")
	}
}

func TestClearSlot(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetSlot(ctx, store, "agent-1", "hook", "bl-abc"); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}

	prev, err := ClearSlot(ctx, store, "agent-1", "hook")
	if err != nil {
		t.Fatalf("ClearSlot failed: %v", err)
	}
	if prev != "bl-abc" {
		t.Errorf("expected prev=bl-abc, got %q", prev)
	}

	rec, err := GetSlots(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetSlots failed: %v", err)
	}
	if rec.Hook != "" {
		t.Errorf("expected empty hook after clear, got %q", rec.Hook)
	}
}

func TestClearSlotAlreadyEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	prev, err := ClearSlot(ctx, store, "agent-1", "hook")
	if err != nil {
		t.Fatalf("ClearSlot failed: %v", err)
	}
	if prev != "" {
		t.Errorf("expected empty prev for non-existent agent, got %q", prev)
	}
}

func TestClearSlotInvalidName(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := ClearSlot(ctx, store, "agent-1", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid slot name")
	}
}

func TestClearThenResetHook(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := SetSlot(ctx, store, "agent-1", "hook", "bl-abc"); err != nil {
		t.Fatalf("SetSlot failed: %v", err)
	}

	if _, err := ClearSlot(ctx, store, "agent-1", "hook"); err != nil {
		t.Fatalf("ClearSlot failed: %v", err)
	}

	// Should succeed after clearing
	if err := SetSlot(ctx, store, "agent-1", "hook", "bl-def"); err != nil {
		t.Fatalf("SetSlot after clear failed: %v", err)
	}

	rec, err := GetSlots(ctx, store, "agent-1")
	if err != nil {
		t.Fatalf("GetSlots failed: %v", err)
	}
	if rec.Hook != "bl-def" {
		t.Errorf("expected hook=bl-def, got %q", rec.Hook)
	}
}
