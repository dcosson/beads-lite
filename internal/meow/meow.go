package meow

import (
	"time"
)

// CurrentOptions configures a Current query.
type CurrentOptions struct {
	MoleculeID string
	Actor      string // filter steps assigned to this actor
	Limit      int
	RangeStart string // optional step-id range start
	RangeEnd   string // optional step-id range end
}

// StaleStep describes a step that is blocking progress.
type StaleStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Reason string `json:"reason"` // why the step is considered stale
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
