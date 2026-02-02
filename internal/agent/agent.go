// Package agent provides helpers for managing agent state records in a KV table ("agents").
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"beads-lite/internal/kvstorage"
)

// Agent holds the state record for a single agent.
type Agent struct {
	State        string    `json:"state"`
	LastActivity time.Time `json:"last_activity"`
	RoleType     string    `json:"role_type,omitempty"`
	Rig          string    `json:"rig,omitempty"`
}

// State constants for agent lifecycle.
const (
	StateIdle     = "idle"
	StateSpawning = "spawning"
	StateRunning  = "running"
	StateWorking  = "working"
	StateStuck    = "stuck"
	StateDone     = "done"
	StateStopped  = "stopped"
	StateDead     = "dead"
)

// ValidStates lists the recognised agent states.
var ValidStates = []string{
	StateIdle,
	StateSpawning,
	StateRunning,
	StateWorking,
	StateStuck,
	StateDone,
	StateStopped,
	StateDead,
}

// ValidateState returns an error if s is not a recognised agent state.
func ValidateState(s string) error {
	for _, v := range ValidStates {
		if s == v {
			return nil
		}
	}
	return fmt.Errorf("invalid agent state %q: must be one of %v", s, ValidStates)
}

// GetAgent retrieves the agent record for the given ID.
// Returns kvstorage.ErrKeyNotFound if the agent does not exist.
func GetAgent(ctx context.Context, store kvstorage.KVStore, agentID string) (Agent, error) {
	data, err := store.Get(ctx, agentID)
	if err != nil {
		if errors.Is(err, kvstorage.ErrKeyNotFound) {
			return Agent{}, kvstorage.ErrKeyNotFound
		}
		return Agent{}, fmt.Errorf("getting agent %s: %w", agentID, err)
	}
	var a Agent
	if err := json.Unmarshal(data, &a); err != nil {
		return Agent{}, fmt.Errorf("decoding agent %s: %w", agentID, err)
	}
	return a, nil
}

// SetState upserts the agent's state. If the agent does not exist it is created
// with the given state and LastActivity set to now. If it does exist, State and
// LastActivity are updated and the record is overwritten.
func SetState(ctx context.Context, store kvstorage.KVStore, agentID, state string) error {
	if err := ValidateState(state); err != nil {
		return err
	}

	now := time.Now().UTC()

	a, err := GetAgent(ctx, store, agentID)
	if err != nil {
		if !errors.Is(err, kvstorage.ErrKeyNotFound) {
			return err
		}
		// Create fresh agent
		a = Agent{
			State:        state,
			LastActivity: now,
		}
	} else {
		a.State = state
		a.LastActivity = now
	}

	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("encoding agent %s: %w", agentID, err)
	}
	return store.Set(ctx, agentID, data, kvstorage.SetOptions{})
}

// Heartbeat updates the agent's LastActivity to now. The agent must already
// exist; returns an error if not found.
func Heartbeat(ctx context.Context, store kvstorage.KVStore, agentID string) error {
	a, err := GetAgent(ctx, store, agentID)
	if err != nil {
		if errors.Is(err, kvstorage.ErrKeyNotFound) {
			return fmt.Errorf("agent %s not found: use 'bd agent state' to create it first", agentID)
		}
		return err
	}

	a.LastActivity = time.Now().UTC()

	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("encoding agent %s: %w", agentID, err)
	}
	return store.Update(ctx, agentID, data)
}
