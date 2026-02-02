package idgen

import (
	"testing"
	"time"
)

func TestHashID_TestVectors(t *testing.T) {
	// Test vectors from beads reference implementation.
	// prefix=bd, title="Fix login", description="Details", creator="jira-import",
	// timestamp=2024-01-02T03:04:05.006Z, nonce=0
	ts, err := time.Parse(time.RFC3339Nano, "2024-01-02T03:04:05.006Z")
	if err != nil {
		t.Fatalf("failed to parse timestamp: %v", err)
	}

	tests := []struct {
		length int
		want   string
	}{
		{3, "bd-vju"},
		{4, "bd-8d8e"},
		{5, "bd-bi3tk"},
		{6, "bd-8bi3tk"},
		{7, "bd-r5sr6bm"},
		{8, "bd-8r5sr6bm"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := HashID("bd-", "Fix login", "Details", "jira-import", ts, 0, tt.length)
			if got != tt.want {
				t.Errorf("HashID(length=%d) = %q, want %q", tt.length, got, tt.want)
			}
		})
	}
}

func TestHashID_Deterministic(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	id1 := HashID("bd-", "Test", "Desc", "user", ts, 0, 5)
	id2 := HashID("bd-", "Test", "Desc", "user", ts, 0, 5)

	if id1 != id2 {
		t.Errorf("HashID not deterministic: %q != %q", id1, id2)
	}
}

func TestHashID_DifferentNonce(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	id0 := HashID("bd-", "Test", "Desc", "user", ts, 0, 5)
	id1 := HashID("bd-", "Test", "Desc", "user", ts, 1, 5)

	if id0 == id1 {
		t.Errorf("Different nonces should produce different IDs: both %q", id0)
	}
}

func TestHashID_CustomPrefix(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	id := HashID("test-", "Title", "Desc", "user", ts, 0, 4)
	if id[:5] != "test-" {
		t.Errorf("Expected prefix 'test-', got %q", id[:5])
	}
}

func TestAdaptiveLength(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  int
	}{
		{"zero issues", 0, MinLength},
		{"few issues", 10, MinLength},
		{"~160 issues still length 3", 100, MinLength},
		{"many issues need length 4", 200, 4},
		{"~1000 issues need length 4", 900, 4},
		{"~1000 issues need length 5", 1000, 5},
		{"~6000 issues need length 5", 5000, 5},
		{"~6000 issues need length 6", 6000, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AdaptiveLength(tt.count)
			if got != tt.want {
				t.Errorf("AdaptiveLength(%d) = %d, want %d", tt.count, got, tt.want)
			}
		})
	}
}
