package config

import (
	"testing"
)

func TestDefaultValues(t *testing.T) {
	defaults := DefaultValues()

	expected := map[string]string{
		"defaults.priority": "medium",
		"defaults.type":     "task",
		"id.prefix":         "bd-",
		"id.length":         "4",
		"actor":             "${USER}",
		"project.name":      "issues",
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

	if err := ApplyDefaults(s); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}

	// Pre-existing key should not be overwritten
	if v, _ := s.Get("actor"); v != "alice" {
		t.Errorf("actor = %q, want %q (should not be overwritten)", v, "alice")
	}

	// Missing keys should be filled from defaults
	if v, ok := s.Get("defaults.priority"); !ok || v != "medium" {
		t.Errorf("defaults.priority = %q, %v; want %q, true", v, ok, "medium")
	}
	if v, ok := s.Get("id.prefix"); !ok || v != "bd-" {
		t.Errorf("id.prefix = %q, %v; want %q, true", v, ok, "bd-")
	}
	if v, ok := s.Get("project.name"); !ok || v != "issues" {
		t.Errorf("project.name = %q, %v; want %q, true", v, ok, "issues")
	}
}

func TestApplyDefaults_AllPresent(t *testing.T) {
	s := &memStore{data: map[string]string{
		"defaults.priority": "high",
		"defaults.type":     "bug",
		"id.prefix":         "x-",
		"id.length":         "8",
		"actor":             "bob",
		"project.name":      "work",
	}}

	if err := ApplyDefaults(s); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}

	// No values should change
	if v, _ := s.Get("defaults.priority"); v != "high" {
		t.Errorf("defaults.priority = %q, want %q", v, "high")
	}
	if v, _ := s.Get("actor"); v != "bob" {
		t.Errorf("actor = %q, want %q", v, "bob")
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
