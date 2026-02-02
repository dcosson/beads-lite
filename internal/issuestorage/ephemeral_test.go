package issuestorage

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEphemeralFieldRoundTrip(t *testing.T) {
	issue := &Issue{
		ID:        "test-eph-1",
		Title:     "Ephemeral issue",
		Status:    StatusOpen,
		Priority:  PriorityMedium,
		Type:      TypeTask,
		Ephemeral: true,
		CreatedAt: time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got Issue
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !got.Ephemeral {
		t.Error("round-trip: Ephemeral should be true, got false")
	}
}

func TestEphemeralOmitemptyFalse(t *testing.T) {
	issue := &Issue{
		ID:        "test-eph-2",
		Title:     "Non-ephemeral issue",
		Status:    StatusOpen,
		Priority:  PriorityMedium,
		Type:      TypeTask,
		Ephemeral: false,
		CreatedAt: time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// With omitempty, Ephemeral=false should NOT appear in JSON output
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if _, exists := raw["ephemeral"]; exists {
		t.Errorf("omitempty: 'ephemeral' key should not appear when false, got JSON: %s", string(data))
	}
}

func TestEphemeralDefaultFalse(t *testing.T) {
	// Simulate an existing issue JSON that was created before the Ephemeral field existed
	oldJSON := `{
		"id": "test-eph-3",
		"title": "Pre-existing issue",
		"status": "open",
		"priority": "medium",
		"type": "task",
		"created_at": "2026-01-30T00:00:00Z",
		"updated_at": "2026-01-30T00:00:00Z"
	}`

	var issue Issue
	if err := json.Unmarshal([]byte(oldJSON), &issue); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if issue.Ephemeral {
		t.Error("existing JSON without ephemeral field should deserialize to Ephemeral=false")
	}
}
