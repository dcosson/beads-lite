package idgen

import (
	"errors"
	"fmt"
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

func TestIsHierarchicalID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		// Hierarchical IDs (suffix after last dot is numeric)
		{"bd-a3f8.1", true},
		{"bd-a3f8.12", true},
		{"bd-a3f8.1.2", true},
		{"prefix.0", true},

		// Non-hierarchical IDs
		{"bd-a3f8", false},
		{"my.project-abc", false},
		{"some.thing.name", false},
		{"bd-a3f8.", false},
		{"", false},
		{"no-dots", false},
		{"trailing.1x", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := IsHierarchicalID(tt.id)
			if got != tt.want {
				t.Errorf("IsHierarchicalID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestRootParentID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		// Hierarchical IDs — return root before first dot
		{"bd-a3f8.1", "bd-a3f8"},
		{"bd-a3f8.1.2", "bd-a3f8"},
		{"prefix.0", "prefix"},

		// Non-hierarchical IDs — return full ID
		{"bd-a3f8", "bd-a3f8"},
		{"no-dots", "no-dots"},
		{"", ""},

		// Dotted but non-numeric suffix — still splits at first dot
		{"my.project-abc", "my"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := RootParentID(tt.id)
			if got != tt.want {
				t.Errorf("RootParentID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestParseHierarchicalID(t *testing.T) {
	tests := []struct {
		id        string
		wantPar   string
		wantChild int
		wantOK    bool
	}{
		{"bd-a3f8.1", "bd-a3f8", 1, true},
		{"bd-a3f8.12", "bd-a3f8", 12, true},
		{"bd-a3f8.1.2", "bd-a3f8.1", 2, true},
		{"prefix.0", "prefix", 0, true},

		// Non-hierarchical — ok should be false
		{"bd-a3f8", "", 0, false},
		{"my.project-abc", "", 0, false},
		{"", "", 0, false},
		{"no-dots", "", 0, false},
		{"bd-a3f8.", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			parent, childNum, ok := ParseHierarchicalID(tt.id)
			if ok != tt.wantOK {
				t.Errorf("ParseHierarchicalID(%q) ok = %v, want %v", tt.id, ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}
			if parent != tt.wantPar {
				t.Errorf("ParseHierarchicalID(%q) parent = %q, want %q", tt.id, parent, tt.wantPar)
			}
			if childNum != tt.wantChild {
				t.Errorf("ParseHierarchicalID(%q) childNum = %d, want %d", tt.id, childNum, tt.wantChild)
			}
		})
	}
}

func TestHierarchyDepth(t *testing.T) {
	tests := []struct {
		id   string
		want int
	}{
		// Root IDs — depth 0
		{"bd-a3f8", 0},
		{"no-dots", 0},
		{"", 0},

		// Single level — depth 1
		{"bd-a3f8.1", 1},
		{"prefix.0", 1},

		// Multi-level
		{"bd-a3f8.1.2", 2},
		{"bd-a3f8.1.2.3", 3},
		{"a.b.c.d.e", 4},

		// Non-numeric suffix still counted (dots are counted)
		{"my.project-abc", 1},
		{"some.thing.name", 2},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := HierarchyDepth(tt.id)
			if got != tt.want {
				t.Errorf("HierarchyDepth(%q) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}

func TestChildID(t *testing.T) {
	tests := []struct {
		parentID string
		childNum int
		want     string
	}{
		{"bd-a3f8", 1, "bd-a3f8.1"},
		{"bd-a3f8", 12, "bd-a3f8.12"},
		{"bd-a3f8.1", 2, "bd-a3f8.1.2"},
		{"bd-a3f8.1.2", 3, "bd-a3f8.1.2.3"},
		{"prefix", 0, "prefix.0"},
	}

	for _, tt := range tests {
		name := tt.parentID + "." + fmt.Sprintf("%d", tt.childNum)
		t.Run(name, func(t *testing.T) {
			got := ChildID(tt.parentID, tt.childNum)
			if got != tt.want {
				t.Errorf("ChildID(%q, %d) = %q, want %q", tt.parentID, tt.childNum, got, tt.want)
			}
		})
	}
}

func TestBuildPrefix(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		addition string
		want     string
	}{
		// No addition — returns base with trailing dash
		{"no_addition", "bd-", "", "bd-"},
		{"no_addition_no_dash", "bd", "", "bd-"},

		// Standard additions
		{"mol", "bd-", "mol", "bd-mol-"},
		{"wisp", "bd-", "wisp", "bd-wisp-"},

		// Base without trailing dash
		{"base_no_dash", "bd", "mol", "bd-mol-"},

		// Addition with leading dash
		{"addition_leading_dash", "bd-", "-mol", "bd-mol-"},
		// Addition with trailing dash
		{"addition_trailing_dash", "bd-", "mol-", "bd-mol-"},
		// Addition with both leading and trailing dashes
		{"addition_both_dashes", "bd-", "-mol-", "bd-mol-"},

		// Base with multiple trailing dashes
		{"base_multi_dash", "bd--", "mol", "bd-mol-"},

		// Different prefix
		{"alt_prefix", "bl-", "wisp", "bl-wisp-"},
		{"alt_prefix_no_dash", "bl", "wisp", "bl-wisp-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPrefix(tt.base, tt.addition)
			if got != tt.want {
				t.Errorf("BuildPrefix(%q, %q) = %q, want %q", tt.base, tt.addition, got, tt.want)
			}
		})
	}
}

func TestCheckHierarchyDepth(t *testing.T) {
	tests := []struct {
		name     string
		parentID string
		maxDepth int
		wantErr  bool
	}{
		// Root parent (depth 0) — always allowed
		{"root_depth0_max3", "bd-a3f8", 3, false},
		// Depth 1 parent — allowed at max 3
		{"depth1_max3", "bd-a3f8.1", 3, false},
		// Depth 2 parent — allowed at max 3
		{"depth2_max3", "bd-a3f8.1.2", 3, false},
		// Depth 3 parent — rejected at max 3 (child would be depth 4)
		{"depth3_max3_rejected", "bd-a3f8.1.2.3", 3, true},
		// Depth 4 parent — rejected at max 3
		{"depth4_max3_rejected", "bd-a3f8.1.2.3.4", 3, true},
		// Custom max depth of 1
		{"depth0_max1", "bd-a3f8", 1, false},
		{"depth1_max1_rejected", "bd-a3f8.1", 1, true},
		// Custom max depth of 5
		{"depth4_max5", "bd-a3f8.1.2.3.4", 5, false},
		{"depth5_max5_rejected", "bd-a3f8.1.2.3.4.5", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckHierarchyDepth(tt.parentID, tt.maxDepth)
			if tt.wantErr {
				if err == nil {
					t.Errorf("CheckHierarchyDepth(%q, %d) = nil, want error", tt.parentID, tt.maxDepth)
				} else if !errors.Is(err, ErrMaxDepthExceeded) {
					t.Errorf("CheckHierarchyDepth(%q, %d) = %v, want ErrMaxDepthExceeded", tt.parentID, tt.maxDepth, err)
				}
			} else {
				if err != nil {
					t.Errorf("CheckHierarchyDepth(%q, %d) = %v, want nil", tt.parentID, tt.maxDepth, err)
				}
			}
		})
	}
}
