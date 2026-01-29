// Package yamlstore implements config.Store backed by a flat YAML file.
//
// The file format is flat key-value pairs where dotted keys (e.g.
// "defaults.priority") are literal strings, not nested paths.
// yaml.Marshal on map[string]string produces alphabetical key ordering,
// making the output deterministic and diff-friendly.
package yamlstore

import (
	"fmt"
	"os"
	"path/filepath"

	"beads-lite/internal/config"

	"gopkg.in/yaml.v3"
)

// YAMLStore implements config.Store using a YAML file on disk.
type YAMLStore struct {
	path string
	data map[string]string
}

// New creates a YAMLStore that reads from and writes to path.
// If the file exists it is loaded; if it does not exist the store
// starts empty and the file is created on the first Set call.
func New(path string) (*YAMLStore, error) {
	s := &YAMLStore{
		path: path,
		data: make(map[string]string),
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if len(raw) == 0 {
		return s, nil
	}

	if err := yaml.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	if s.data == nil {
		s.data = make(map[string]string)
	}

	return s, nil
}

// Get returns the value for key and whether it was found.
func (s *YAMLStore) Get(key string) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}

// Set writes key=value and persists to disk.
func (s *YAMLStore) Set(key, value string) error {
	s.data[key] = value
	return s.save()
}

// Unset removes key and persists to disk.
func (s *YAMLStore) Unset(key string) error {
	delete(s.data, key)
	return s.save()
}

// All returns a copy of all key-value pairs.
func (s *YAMLStore) All() map[string]string {
	out := make(map[string]string, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// save writes the current data to disk. It creates parent directories
// as needed and uses yaml.Marshal for deterministic alphabetical ordering.
func (s *YAMLStore) save() error {
	raw, err := yaml.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(s.path, raw, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// Compile-time check that YAMLStore implements config.Store.
var _ config.Store = (*YAMLStore)(nil)
