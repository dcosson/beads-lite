package issuestorage

import "testing"

func TestValidateMolType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"swarm", true},
		{"patrol", true},
		{"work", true},
		{"invalid", false},
		{"SWARM", false},   // case-sensitive
		{"Patrol", false},  // case-sensitive
		{"Work", false},    // case-sensitive
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ValidateMolType(tt.input)
			if got != tt.want {
				t.Errorf("ValidateMolType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
