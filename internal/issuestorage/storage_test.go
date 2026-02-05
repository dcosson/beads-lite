package issuestorage

import (
	"encoding/json"
	"testing"
)

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
		{"SWARM", false},  // case-sensitive
		{"Patrol", false}, // case-sensitive
		{"Work", false},   // case-sensitive
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

func TestIssueChildren(t *testing.T) {
	issue := &Issue{
		Dependents: []Dependency{
			{ID: "child-1", Type: DepTypeParentChild},
			{ID: "blocker-1", Type: DepTypeBlocks},
			{ID: "child-2", Type: DepTypeParentChild},
			{ID: "related-1", Type: DepTypeRelated},
		},
	}

	children := issue.Children()
	if len(children) != 2 {
		t.Fatalf("Children() returned %d items, want 2", len(children))
	}
	if children[0] != "child-1" || children[1] != "child-2" {
		t.Errorf("Children() = %v, want [child-1, child-2]", children)
	}

	// Empty case
	empty := &Issue{}
	if len(empty.Children()) != 0 {
		t.Error("Children() on issue with no dependents should return empty slice")
	}
}

func TestIssueHasDependency(t *testing.T) {
	issue := &Issue{
		Dependencies: []Dependency{
			{ID: "dep-1", Type: DepTypeBlocks},
			{ID: "dep-2", Type: DepTypeRelated},
		},
	}

	if !issue.HasDependency("dep-1") {
		t.Error("HasDependency(dep-1) should be true")
	}
	if !issue.HasDependency("dep-2") {
		t.Error("HasDependency(dep-2) should be true")
	}
	if issue.HasDependency("dep-3") {
		t.Error("HasDependency(dep-3) should be false")
	}
	if issue.HasDependency("") {
		t.Error("HasDependency('') should be false")
	}
}

func TestIssueHasDependent(t *testing.T) {
	issue := &Issue{
		Dependents: []Dependency{
			{ID: "child-1", Type: DepTypeParentChild},
			{ID: "blocker-1", Type: DepTypeBlocks},
		},
	}

	if !issue.HasDependent("child-1") {
		t.Error("HasDependent(child-1) should be true")
	}
	if !issue.HasDependent("blocker-1") {
		t.Error("HasDependent(blocker-1) should be true")
	}
	if issue.HasDependent("other") {
		t.Error("HasDependent(other) should be false")
	}
}

func TestIssueDependencyIDs(t *testing.T) {
	issue := &Issue{
		Dependencies: []Dependency{
			{ID: "blocks-1", Type: DepTypeBlocks},
			{ID: "related-1", Type: DepTypeRelated},
			{ID: "blocks-2", Type: DepTypeBlocks},
		},
	}

	// No filter - returns all
	all := issue.DependencyIDs(nil)
	if len(all) != 3 {
		t.Errorf("DependencyIDs(nil) returned %d items, want 3", len(all))
	}

	// Filter by type
	blocksType := DepTypeBlocks
	blocks := issue.DependencyIDs(&blocksType)
	if len(blocks) != 2 {
		t.Errorf("DependencyIDs(blocks) returned %d items, want 2", len(blocks))
	}

	relatedType := DepTypeRelated
	related := issue.DependencyIDs(&relatedType)
	if len(related) != 1 || related[0] != "related-1" {
		t.Errorf("DependencyIDs(related) = %v, want [related-1]", related)
	}

	// Filter with no matches
	parentType := DepTypeParentChild
	parents := issue.DependencyIDs(&parentType)
	if len(parents) != 0 {
		t.Errorf("DependencyIDs(parent-child) = %v, want []", parents)
	}
}

func TestIssueDependentIDs(t *testing.T) {
	issue := &Issue{
		Dependents: []Dependency{
			{ID: "child-1", Type: DepTypeParentChild},
			{ID: "child-2", Type: DepTypeParentChild},
			{ID: "blocker-1", Type: DepTypeBlocks},
		},
	}

	// No filter - returns all
	all := issue.DependentIDs(nil)
	if len(all) != 3 {
		t.Errorf("DependentIDs(nil) returned %d items, want 3", len(all))
	}

	// Filter by type
	parentType := DepTypeParentChild
	children := issue.DependentIDs(&parentType)
	if len(children) != 2 {
		t.Errorf("DependentIDs(parent-child) returned %d items, want 2", len(children))
	}
}

func TestPriorityDisplay(t *testing.T) {
	tests := []struct {
		p    Priority
		want string
	}{
		{PriorityCritical, "P0"},
		{PriorityHigh, "P1"},
		{PriorityMedium, "P2"},
		{PriorityLow, "P3"},
		{PriorityBacklog, "P4"},
	}

	for _, tt := range tests {
		if got := tt.p.Display(); got != tt.want {
			t.Errorf("Priority(%d).Display() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestPriorityJSON(t *testing.T) {
	// Marshal
	for _, p := range []Priority{PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow, PriorityBacklog} {
		data, err := json.Marshal(p)
		if err != nil {
			t.Errorf("Marshal Priority(%d) failed: %v", p, err)
			continue
		}
		// Should marshal as integer
		var n int
		if err := json.Unmarshal(data, &n); err != nil {
			t.Errorf("Priority(%d) did not marshal as int: %s", p, data)
		}
		if n != int(p) {
			t.Errorf("Priority(%d) marshaled as %d", p, n)
		}
	}

	// Unmarshal from int
	var p Priority
	if err := json.Unmarshal([]byte("2"), &p); err != nil {
		t.Errorf("Unmarshal int failed: %v", err)
	}
	if p != PriorityMedium {
		t.Errorf("Unmarshal(2) = %d, want %d", p, PriorityMedium)
	}

	// Unmarshal from legacy string (backward compatibility)
	legacyTests := []struct {
		json string
		want Priority
	}{
		{`"critical"`, PriorityCritical},
		{`"high"`, PriorityHigh},
		{`"medium"`, PriorityMedium},
		{`"low"`, PriorityLow},
		{`"backlog"`, PriorityBacklog},
	}
	for _, tt := range legacyTests {
		var p Priority
		if err := json.Unmarshal([]byte(tt.json), &p); err != nil {
			t.Errorf("Unmarshal(%s) failed: %v", tt.json, err)
			continue
		}
		if p != tt.want {
			t.Errorf("Unmarshal(%s) = %d, want %d", tt.json, p, tt.want)
		}
	}

	// Unmarshal invalid
	var bad Priority
	if err := json.Unmarshal([]byte(`"invalid"`), &bad); err == nil {
		t.Error("Unmarshal(invalid) should fail")
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input   string
		want    Priority
		wantErr bool
	}{
		// Numeric
		{"0", PriorityCritical, false},
		{"1", PriorityHigh, false},
		{"2", PriorityMedium, false},
		{"3", PriorityLow, false},
		{"4", PriorityBacklog, false},
		// P-format
		{"P0", PriorityCritical, false},
		{"P1", PriorityHigh, false},
		{"P2", PriorityMedium, false},
		{"P3", PriorityLow, false},
		{"P4", PriorityBacklog, false},
		{"p2", PriorityMedium, false}, // case-insensitive
		// Legacy words
		{"critical", PriorityCritical, false},
		{"high", PriorityHigh, false},
		{"medium", PriorityMedium, false},
		{"low", PriorityLow, false},
		{"backlog", PriorityBacklog, false},
		// Empty defaults to medium
		{"", PriorityMedium, false},
		// Invalid
		{"5", PriorityMedium, true},
		{"P5", PriorityMedium, true},
		{"urgent", PriorityMedium, true},
		{"invalid", PriorityMedium, true},
	}

	for _, tt := range tests {
		got, err := ParsePriority(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParsePriority(%q) should error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParsePriority(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParsePriority(%q) = %d, want %d", tt.input, got, tt.want)
			}
		}
	}
}
