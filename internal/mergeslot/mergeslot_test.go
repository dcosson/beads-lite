package mergeslot

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
	store, err := kvfs.New(dir, "merge-slot")
	if err != nil {
		t.Fatalf("failed to create kv store: %v", err)
	}
	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("failed to init kv store: %v", err)
	}
	return store
}

func TestCreateIdempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Second create should succeed (idempotent)
	if err := Create(ctx, store); err != nil {
		t.Fatalf("second Create failed: %v", err)
	}

	ms, err := Check(ctx, store)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if ms.Status != StatusOpen {
		t.Errorf("expected status=open, got %q", ms.Status)
	}
}

func TestCheckNotCreated(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := Check(ctx, store)
	if err == nil {
		t.Fatal("expected error for uncreated merge slot")
	}
	if !errors.Is(err, kvstorage.ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got: %v", err)
	}
}

func TestAcquireOpen(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	ms, err := Acquire(ctx, store, "refinery-1", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if ms.Status != StatusHeld {
		t.Errorf("expected status=held, got %q", ms.Status)
	}
	if ms.Holder != "refinery-1" {
		t.Errorf("expected holder=refinery-1, got %q", ms.Holder)
	}
}

func TestAcquireHeldNoWait(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := Acquire(ctx, store, "refinery-1", false); err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}

	_, err := Acquire(ctx, store, "polecat-1", false)
	if !errors.Is(err, ErrSlotHeld) {
		t.Fatalf("expected ErrSlotHeld, got: %v", err)
	}

	// Verify no waiters were added
	ms, err := Check(ctx, store)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(ms.Waiters) != 0 {
		t.Errorf("expected no waiters, got %v", ms.Waiters)
	}
}

func TestAcquireHeldWithWait(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := Acquire(ctx, store, "refinery-1", false); err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}

	_, err := Acquire(ctx, store, "polecat-1", true)
	if !errors.Is(err, ErrSlotHeld) {
		t.Fatalf("expected ErrSlotHeld, got: %v", err)
	}

	ms, err := Check(ctx, store)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(ms.Waiters) != 1 || ms.Waiters[0] != "polecat-1" {
		t.Errorf("expected waiters=[polecat-1], got %v", ms.Waiters)
	}
}

func TestAcquireWaitDedup(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := Acquire(ctx, store, "refinery-1", false); err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}

	// Add same waiter twice
	Acquire(ctx, store, "polecat-1", true)
	Acquire(ctx, store, "polecat-1", true)

	ms, err := Check(ctx, store)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if len(ms.Waiters) != 1 {
		t.Errorf("expected 1 waiter (dedup), got %d: %v", len(ms.Waiters), ms.Waiters)
	}
}

func TestReleaseSuccess(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := Acquire(ctx, store, "refinery-1", false); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	ms, firstWaiter, err := Release(ctx, store, "")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if ms.Status != StatusOpen {
		t.Errorf("expected status=open after release, got %q", ms.Status)
	}
	if ms.Holder != "" {
		t.Errorf("expected empty holder after release, got %q", ms.Holder)
	}
	if firstWaiter != "" {
		t.Errorf("expected no first waiter, got %q", firstWaiter)
	}
}

func TestReleaseWithHolderCheck(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := Acquire(ctx, store, "refinery-1", false); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Wrong holder should fail
	_, _, err := Release(ctx, store, "wrong-holder")
	if err == nil {
		t.Fatal("expected error for wrong holder check")
	}

	// Correct holder should succeed
	_, _, err = Release(ctx, store, "refinery-1")
	if err != nil {
		t.Fatalf("Release with correct holder failed: %v", err)
	}
}

func TestReleaseNotHeld(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, _, err := Release(ctx, store, "")
	if err == nil {
		t.Fatal("expected error releasing unheld slot")
	}
}

func TestReleaseReturnsFirstWaiter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := Acquire(ctx, store, "refinery-1", false); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Add waiters
	Acquire(ctx, store, "polecat-1", true)
	Acquire(ctx, store, "polecat-2", true)

	ms, firstWaiter, err := Release(ctx, store, "")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if firstWaiter != "polecat-1" {
		t.Errorf("expected first waiter=polecat-1, got %q", firstWaiter)
	}
	// Waiters should NOT be popped
	if len(ms.Waiters) != 2 {
		t.Errorf("expected 2 waiters (not popped), got %d", len(ms.Waiters))
	}
}

func TestFullCycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create
	if err := Create(ctx, store); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Acquire
	ms, err := Acquire(ctx, store, "refinery-1", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if ms.Status != StatusHeld || ms.Holder != "refinery-1" {
		t.Fatalf("unexpected state after acquire: %+v", ms)
	}

	// Release
	ms, _, err = Release(ctx, store, "refinery-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if ms.Status != StatusOpen {
		t.Fatalf("expected open after release, got %q", ms.Status)
	}

	// Re-acquire by different requester
	ms, err = Acquire(ctx, store, "polecat-1", false)
	if err != nil {
		t.Fatalf("re-Acquire failed: %v", err)
	}
	if ms.Holder != "polecat-1" {
		t.Errorf("expected holder=polecat-1, got %q", ms.Holder)
	}
}
