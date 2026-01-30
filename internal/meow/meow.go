package meow

import (
	"context"
	"errors"
	"time"

	"beads-lite/internal/storage"
)

// ErrNotImplemented is returned by functions that require Phase 3/4 work.
var ErrNotImplemented = errors.New("not implemented: requires molecule storage (Phase 3/4)")

// StepInfo describes a single step's current state.
type StepInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Assignee string `json:"assignee,omitempty"`
}

// CurrentOptions configures a Current query.
type CurrentOptions struct {
	MoleculeID string
	Actor      string // filter steps assigned to this actor
	Limit      int
	RangeStart string // optional step-id range start
	RangeEnd   string // optional step-id range end
}

// CurrentResult holds the classified steps for a molecule.
type CurrentResult struct {
	Steps []StepInfo `json:"steps"`
}

// Current returns the steps of a molecule, optionally filtered by actor
// and range. Steps are classified by status.
func Current(_ context.Context, _ storage.Storage, _ CurrentOptions) (*CurrentResult, error) {
	return nil, ErrNotImplemented
}

// ProgressResult holds completion statistics for a molecule.
type ProgressResult struct {
	Total    int     `json:"total"`
	Complete int     `json:"complete"`
	Percent  float64 `json:"percent"`
	Rate     string  `json:"rate,omitempty"` // e.g. "3/day"
	ETA      string  `json:"eta,omitempty"`  // e.g. "2 days"
}

// Progress computes completion statistics for a molecule.
func Progress(_ context.Context, _ storage.Storage, _ string) (*ProgressResult, error) {
	return nil, ErrNotImplemented
}

// StaleStep describes a step that is blocking progress.
type StaleStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Reason string `json:"reason"` // why the step is considered stale
}

// FindStaleSteps returns steps that are blocking progress in a molecule
// (e.g. open steps whose dependencies are all met but that haven't moved).
func FindStaleSteps(_ context.Context, _ storage.Storage, _ string) ([]*StaleStep, error) {
	return nil, ErrNotImplemented
}

// SquashOptions configures a Squash operation (digest creation).
type SquashOptions struct {
	MoleculeID   string
	Summary      string
	KeepChildren bool
}

// SquashResult describes the outcome of a Squash.
type SquashResult struct {
	DigestID     string   `json:"digest_id"`
	SquashedIDs  []string `json:"squashed_ids"`
	KeepChildren bool     `json:"keep_children"`
}

// Squash creates a digest issue summarising a molecule and optionally
// removes child steps.
func Squash(_ context.Context, _ storage.Storage, _ SquashOptions) (*SquashResult, error) {
	return nil, ErrNotImplemented
}

// GCOptions configures a GC (garbage collection) operation.
type GCOptions struct {
	OlderThan time.Duration
}

// GCResult describes the outcome of a GC run.
type GCResult struct {
	RemovedIDs []string `json:"removed_ids"`
	Count      int      `json:"count"`
}

// GC removes ephemeral molecules older than the specified duration.
func GC(_ context.Context, _ storage.Storage, _ GCOptions) (*GCResult, error) {
	return nil, ErrNotImplemented
}
