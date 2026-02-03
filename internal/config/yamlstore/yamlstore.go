// Package yamlstore implements config.Store backed by a flat YAML file.
//
// The file format is flat key-value pairs where dotted keys (e.g.
// "defaults.priority") are literal strings, not nested paths.
// yaml.Marshal on map[string]string produces alphabetical key ordering,
// making the output deterministic and diff-friendly.
package yamlstore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

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
	return s.withLock(func() {
		s.data[key] = value
	})
}

// SetInMemory writes key=value to the in-memory store without persisting.
func (s *YAMLStore) SetInMemory(key, value string) {
	s.data[key] = value
}

// Unset removes key and persists to disk.
func (s *YAMLStore) Unset(key string) error {
	return s.withLock(func() {
		delete(s.data, key)
	})
}

// All returns a copy of all key-value pairs.
func (s *YAMLStore) All() map[string]string {
	out := make(map[string]string, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// lockPath returns the path to the lock file used for flock-based coordination.
func (s *YAMLStore) lockPath() string {
	return s.path + ".lock"
}

// withLock acquires an exclusive file lock, re-reads the config from disk
// (picking up writes from other processes), calls fn to mutate s.data,
// then atomically writes s.data back to disk.
func (s *YAMLStore) withLock(fn func()) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening config lock: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring config lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Re-read from disk to pick up changes from other processes.
	if err := s.readFromDisk(); err != nil {
		return err
	}

	fn()

	raw, err := yaml.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return atomicWrite(s.path, raw)
}

// readFromDisk reloads s.data from the config file on disk.
func (s *YAMLStore) readFromDisk() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]string)
			return nil
		}
		return fmt.Errorf("reading config file: %w", err)
	}

	if len(raw) == 0 {
		s.data = make(map[string]string)
		return nil
	}

	fresh := make(map[string]string)
	if err := yaml.Unmarshal(raw, &fresh); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}
	if fresh == nil {
		fresh = make(map[string]string)
	}
	s.data = fresh
	return nil
}

// atomicWrite writes data to a file atomically via a temporary file and rename.
func atomicWrite(path string, data []byte) error {
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("generating random suffix: %w", err)
	}
	tmp := path + ".tmp." + hex.EncodeToString(randBytes)

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) // best effort cleanup
		return err
	}
	return nil
}

// Compile-time check that YAMLStore implements config.Store.
var _ config.Store = (*YAMLStore)(nil)
