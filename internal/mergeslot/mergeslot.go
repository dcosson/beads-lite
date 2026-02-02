// Package mergeslot provides a KV-backed mutex for serializing git merge
// conflict resolution. One lock per rig, stored as a single KV entry with
// the fixed key "lock" in a "merge-slot" table.
package mergeslot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"beads-lite/internal/kvstorage"
)

// Status constants for the merge slot.
const (
	StatusOpen = "open"
	StatusHeld = "held"
)

// lockKey is the fixed KV key for the single merge-slot entry per rig.
const lockKey = "lock"

// MergeSlot holds the state of the merge slot mutex.
type MergeSlot struct {
	Status  string   `json:"status"`
	Holder  string   `json:"holder,omitempty"`
	Waiters []string `json:"waiters,omitempty"`
}

// Create initialises the merge slot in the given store. It is idempotent:
// if the slot already exists the call succeeds silently.
func Create(ctx context.Context, store kvstorage.KVStore) error {
	ms := MergeSlot{Status: StatusOpen}
	data, err := json.Marshal(ms)
	if err != nil {
		return fmt.Errorf("encoding merge slot: %w", err)
	}
	err = store.Set(ctx, lockKey, data, kvstorage.SetOptions{FailIfExists: true})
	if err != nil {
		if errors.Is(err, kvstorage.ErrAlreadyExists) {
			return nil // idempotent
		}
		return fmt.Errorf("creating merge slot: %w", err)
	}
	return nil
}

// Check reads the current merge slot state. Returns ErrKeyNotFound if
// the slot has not been created.
func Check(ctx context.Context, store kvstorage.KVStore) (MergeSlot, error) {
	data, err := store.Get(ctx, lockKey)
	if err != nil {
		return MergeSlot{}, fmt.Errorf("reading merge slot: %w", err)
	}
	var ms MergeSlot
	if err := json.Unmarshal(data, &ms); err != nil {
		return MergeSlot{}, fmt.Errorf("decoding merge slot: %w", err)
	}
	return ms, nil
}

// ErrSlotHeld is returned by Acquire when the slot is already held.
var ErrSlotHeld = errors.New("merge slot is held")

// Acquire attempts to acquire the merge slot for the given requester.
//
// If the slot is open, it transitions to held with the requester as holder.
//
// If the slot is already held and wait is true, the requester is appended
// to the waiters list (deduplicated) and ErrSlotHeld is returned.
//
// If the slot is already held and wait is false, ErrSlotHeld is returned
// without modifying the waiters list.
//
// Note: acquire is a Get-then-Set operation, not truly atomic at the
// filesystem level. Gas Town orchestration serializes access so the race
// is not a practical concern.
func Acquire(ctx context.Context, store kvstorage.KVStore, requester string, wait bool) (MergeSlot, error) {
	ms, err := Check(ctx, store)
	if err != nil {
		return MergeSlot{}, err
	}

	if ms.Status == StatusOpen {
		ms.Status = StatusHeld
		ms.Holder = requester
		data, err := json.Marshal(ms)
		if err != nil {
			return MergeSlot{}, fmt.Errorf("encoding merge slot: %w", err)
		}
		if err := store.Update(ctx, lockKey, data); err != nil {
			return MergeSlot{}, fmt.Errorf("updating merge slot: %w", err)
		}
		return ms, nil
	}

	// Slot is held
	if wait {
		// Append requester to waiters, deduplicating
		if !containsWaiter(ms.Waiters, requester) {
			ms.Waiters = append(ms.Waiters, requester)
			data, err := json.Marshal(ms)
			if err != nil {
				return ms, fmt.Errorf("encoding merge slot: %w", err)
			}
			if err := store.Update(ctx, lockKey, data); err != nil {
				return ms, fmt.Errorf("updating merge slot waiters: %w", err)
			}
		}
	}
	return ms, ErrSlotHeld
}

// Release releases the merge slot. If holderCheck is non-empty, the current
// holder must match or an error is returned. On success, the slot transitions
// to open and the holder is cleared. Returns the first waiter (without
// popping it from the list) so the caller can notify them.
func Release(ctx context.Context, store kvstorage.KVStore, holderCheck string) (MergeSlot, string, error) {
	ms, err := Check(ctx, store)
	if err != nil {
		return MergeSlot{}, "", err
	}

	if ms.Status != StatusHeld {
		return ms, "", fmt.Errorf("merge slot is not held (status: %s)", ms.Status)
	}

	if holderCheck != "" && ms.Holder != holderCheck {
		return ms, "", fmt.Errorf("holder mismatch: slot held by %q, not %q", ms.Holder, holderCheck)
	}

	firstWaiter := ""
	if len(ms.Waiters) > 0 {
		firstWaiter = ms.Waiters[0]
	}

	ms.Status = StatusOpen
	ms.Holder = ""

	data, err := json.Marshal(ms)
	if err != nil {
		return MergeSlot{}, "", fmt.Errorf("encoding merge slot: %w", err)
	}
	if err := store.Update(ctx, lockKey, data); err != nil {
		return MergeSlot{}, "", fmt.Errorf("updating merge slot: %w", err)
	}
	return ms, firstWaiter, nil
}

func containsWaiter(waiters []string, requester string) bool {
	for _, w := range waiters {
		if w == requester {
			return true
		}
	}
	return false
}
