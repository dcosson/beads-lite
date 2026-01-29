package storage

import "testing"

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
