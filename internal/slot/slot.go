// Package slot provides helpers for managing named attachment points on agent beads.
// Slots are stored as JSON records in a KV table ("slots").
package slot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"beads-lite/internal/kvstorage"
)

// SlotRecord holds the slot values for a single agent.
type SlotRecord struct {
	Hook string `json:"hook,omitempty"`
	Role string `json:"role,omitempty"`
}

// ValidSlots lists the recognised slot names.
var ValidSlots = []string{"hook", "role"}

// GetSlots retrieves the slot record for an agent.
// Returns an empty SlotRecord (not an error) if the key is not found.
func GetSlots(ctx context.Context, store kvstorage.KVStore, agentID string) (SlotRecord, error) {
	data, err := store.Get(ctx, agentID)
	if err != nil {
		if errors.Is(err, kvstorage.ErrKeyNotFound) {
			return SlotRecord{}, nil
		}
		return SlotRecord{}, fmt.Errorf("getting slots for %s: %w", agentID, err)
	}
	var rec SlotRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return SlotRecord{}, fmt.Errorf("decoding slots for %s: %w", agentID, err)
	}
	return rec, nil
}

// SetSlot sets a single slot value on an agent's record.
// Hook cardinality is enforced: if the hook slot is already occupied,
// an error is returned. Role has no cardinality limit and is overwritten.
func SetSlot(ctx context.Context, store kvstorage.KVStore, agentID, slotName, beadID string) error {
	rec, err := GetSlots(ctx, store, agentID)
	if err != nil {
		return err
	}

	switch slotName {
	case "hook":
		if rec.Hook != "" {
			return fmt.Errorf("hook slot on %s is occupied by %s; use 'bd slot clear' first", agentID, rec.Hook)
		}
		rec.Hook = beadID
	case "role":
		rec.Role = beadID
	default:
		return fmt.Errorf("unknown slot %q", slotName)
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding slots for %s: %w", agentID, err)
	}
	return store.Set(ctx, agentID, data, kvstorage.SetOptions{})
}

// ClearSlot clears a single slot value on an agent's record.
// Returns the previous value so the caller can manage status revert.
func ClearSlot(ctx context.Context, store kvstorage.KVStore, agentID, slotName string) (string, error) {
	rec, err := GetSlots(ctx, store, agentID)
	if err != nil {
		return "", err
	}

	var prev string
	switch slotName {
	case "hook":
		prev = rec.Hook
		rec.Hook = ""
	case "role":
		prev = rec.Role
		rec.Role = ""
	default:
		return "", fmt.Errorf("unknown slot %q", slotName)
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("encoding slots for %s: %w", agentID, err)
	}
	return prev, store.Set(ctx, agentID, data, kvstorage.SetOptions{})
}
