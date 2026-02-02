package idgen

import (
	"strings"
	"testing"
)

func TestRandomID_CorrectPrefix(t *testing.T) {
	id, err := RandomID("bd-", 4)
	if err != nil {
		t.Fatalf("RandomID returned error: %v", err)
	}
	if !strings.HasPrefix(id, "bd-") {
		t.Errorf("expected prefix 'bd-', got %q", id)
	}
}

func TestRandomID_CorrectLength(t *testing.T) {
	for length := MinLength; length <= MaxLength; length++ {
		id, err := RandomID("bd-", length)
		if err != nil {
			t.Fatalf("RandomID(length=%d) returned error: %v", length, err)
		}
		// Total length = prefix length + random part length
		wantLen := len("bd-") + length
		if len(id) != wantLen {
			t.Errorf("RandomID(length=%d) = %q (len %d), want len %d", length, id, len(id), wantLen)
		}
	}
}

func TestRandomID_ValidBase36(t *testing.T) {
	id, err := RandomID("bd-", 5)
	if err != nil {
		t.Fatalf("RandomID returned error: %v", err)
	}
	suffix := id[len("bd-"):]
	for _, c := range suffix {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
			t.Errorf("invalid base36 character %q in ID %q", string(c), id)
		}
	}
}

func TestRandomID_DifferentIDs(t *testing.T) {
	id1, err := RandomID("bd-", 5)
	if err != nil {
		t.Fatalf("RandomID returned error: %v", err)
	}
	id2, err := RandomID("bd-", 5)
	if err != nil {
		t.Fatalf("RandomID returned error: %v", err)
	}
	if id1 == id2 {
		t.Errorf("two RandomID calls produced the same ID: %q", id1)
	}
}

func TestRandomID_CustomPrefix(t *testing.T) {
	id, err := RandomID("test-", 4)
	if err != nil {
		t.Fatalf("RandomID returned error: %v", err)
	}
	if !strings.HasPrefix(id, "test-") {
		t.Errorf("expected prefix 'test-', got %q", id)
	}
}

func TestRandomID_BoundsError(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"too short", MinLength - 1},
		{"zero", 0},
		{"negative", -1},
		{"too long", MaxLength + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RandomID("bd-", tt.length)
			if err == nil {
				t.Errorf("RandomID(length=%d) expected error, got nil", tt.length)
			}
		})
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
