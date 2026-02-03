package config

import (
	"strings"
	"testing"
)

func TestDefaultValues(t *testing.T) {
	defaults := DefaultValues()

	expected := map[string]string{
		"create.require-description": "false",
		"defaults.priority":          "medium",
		"defaults.type":              "task",
		"issue_prefix":                  "bd",
		"actor":                      "${USER}",
		"project.name":               "issues",
		"hierarchy.max_depth":        "3",
	}

	if len(defaults) != len(expected) {
		t.Fatalf("DefaultValues() has %d entries, want %d", len(defaults), len(expected))
	}

	for k, want := range expected {
		got, ok := defaults[k]
		if !ok {
			t.Errorf("DefaultValues() missing key %q", k)
			continue
		}
		if got != want {
			t.Errorf("DefaultValues()[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	s := &memStore{data: map[string]string{
		"actor": "alice",
	}}

	ApplyDefaults(s)

	// Pre-existing key should not be overwritten
	if v, _ := s.Get("actor"); v != "alice" {
		t.Errorf("actor = %q, want %q (should not be overwritten)", v, "alice")
	}

	// Missing keys should be filled from defaults
	if v, ok := s.Get("defaults.priority"); !ok || v != "medium" {
		t.Errorf("defaults.priority = %q, %v; want %q, true", v, ok, "medium")
	}
	if v, ok := s.Get("issue_prefix"); !ok || v != "bd" {
		t.Errorf("issue_prefix = %q, %v; want %q, true", v, ok, "bd")
	}
	if v, ok := s.Get("project.name"); !ok || v != "issues" {
		t.Errorf("project.name = %q, %v; want %q, true", v, ok, "issues")
	}
}

func TestApplyDefaults_AllPresent(t *testing.T) {
	s := &memStore{data: map[string]string{
		"defaults.priority": "high",
		"defaults.type":     "bug",
		"issue_prefix":         "x-",
		"actor":             "bob",
		"project.name":      "work",
	}}

	ApplyDefaults(s)

	// No values should change
	if v, _ := s.Get("defaults.priority"); v != "high" {
		t.Errorf("defaults.priority = %q, want %q", v, "high")
	}
	if v, _ := s.Get("actor"); v != "bob" {
		t.Errorf("actor = %q, want %q", v, "bob")
	}
}

func TestSplitCustomValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"whitespace only", "   ", nil},
		{"single value", "widget", []string{"widget"}},
		{"comma separated", "widget,gadget", []string{"widget", "gadget"}},
		{"whitespace trimming", " widget , gadget , doohickey ", []string{"widget", "gadget", "doohickey"}},
		{"trailing comma", "widget,gadget,", []string{"widget", "gadget"}},
		{"empty segments", "widget,,gadget", []string{"widget", "gadget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCustomValues(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("SplitCustomValues(%q) = %v, want %v", tt.input, got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("SplitCustomValues(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestValidate_TypesCustomAccepted(t *testing.T) {
	s := &memStore{data: map[string]string{
		"types.custom": "widget,gadget",
	}}
	if err := Validate(s); err != nil {
		t.Errorf("Validate should accept types.custom: %v", err)
	}
}

func TestValidate_StatusCustomAccepted(t *testing.T) {
	s := &memStore{data: map[string]string{
		"status.custom": "review,qa",
	}}
	if err := Validate(s); err != nil {
		t.Errorf("Validate should accept status.custom: %v", err)
	}
}

func TestValidate_DefaultsTypeWithCustomTypes(t *testing.T) {
	s := &memStore{data: map[string]string{
		"types.custom":  "widget,gadget",
		"defaults.type": "widget",
	}}
	if err := Validate(s); err != nil {
		t.Errorf("Validate should accept custom type as defaults.type: %v", err)
	}
}

func TestValidate_DefaultsTypeInvalidWithCustomTypes(t *testing.T) {
	s := &memStore{data: map[string]string{
		"types.custom":  "widget,gadget",
		"defaults.type": "bogus",
	}}
	err := Validate(s)
	if err == nil {
		t.Fatal("Validate should reject unknown defaults.type even with custom types")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention the invalid value: %v", err)
	}
}

func TestValidate_DefaultsTypeBuiltInStillWorks(t *testing.T) {
	s := &memStore{data: map[string]string{
		"types.custom":  "widget",
		"defaults.type": "task",
	}}
	if err := Validate(s); err != nil {
		t.Errorf("Validate should still accept built-in types: %v", err)
	}
}

// memStore is a simple in-memory Store for testing.
type memStore struct {
	data map[string]string
}

func (m *memStore) Get(key string) (string, bool) {
	v, ok := m.data[key]
	return v, ok
}

func (m *memStore) Set(key, value string) error {
	m.data[key] = value
	return nil
}

func (m *memStore) SetInMemory(key, value string) {
	m.data[key] = value
}

func (m *memStore) Unset(key string) error {
	delete(m.data, key)
	return nil
}

func (m *memStore) All() map[string]string {
	out := make(map[string]string, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out
}
