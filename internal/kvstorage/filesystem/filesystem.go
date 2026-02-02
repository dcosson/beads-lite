// Package filesystem implements kvstorage.KVStore using the local filesystem.
// Each key is stored as a JSON file in a named table directory under .beads/.
package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beads-lite/internal/kvstorage"
)

// Store implements kvstorage.KVStore using filesystem-backed JSON files.
// Each table is a directory, and each key is a .json file within it.
type Store struct {
	dir string // absolute path to the table directory
}

// New creates a new filesystem KV store for the given table.
// root is the .beads/<project> directory; table is the table name.
// Returns an error if the table name is reserved or invalid.
func New(root, table string) (*Store, error) {
	if err := kvstorage.ValidateTableName(table); err != nil {
		return nil, err
	}
	return &Store{dir: filepath.Join(root, table)}, nil
}

// Init creates the table directory if it doesn't exist.
func (s *Store) Init(ctx context.Context) error {
	return os.MkdirAll(s.dir, 0755)
}

// Set stores a value for the given key.
func (s *Store) Set(ctx context.Context, key string, value []byte, opts kvstorage.SetOptions) error {
	if err := validateKey(key); err != nil {
		return err
	}
	path := s.keyPath(key)
	if opts.FailIfExists {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("key %q: %w", key, kvstorage.ErrAlreadyExists)
		}
	}
	return atomicWrite(path, value)
}

// Get retrieves the value for the given key.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.keyPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key %q: %w", key, kvstorage.ErrKeyNotFound)
		}
		return nil, err
	}
	return data, nil
}

// Update replaces the value for an existing key.
func (s *Store) Update(ctx context.Context, key string, value []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}
	path := s.keyPath(key)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("key %q: %w", key, kvstorage.ErrKeyNotFound)
	}
	return atomicWrite(path, value)
}

// Delete removes a key and its value.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	path := s.keyPath(key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key %q: %w", key, kvstorage.ErrKeyNotFound)
		}
		return err
	}
	return nil
}

// List returns all keys in the table.
func (s *Store) List(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		keys = append(keys, strings.TrimSuffix(name, ".json"))
	}
	return keys, nil
}

// keyPath returns the filesystem path for a key.
func (s *Store) keyPath(key string) string {
	return filepath.Join(s.dir, key+".json")
}

// validateKey checks that a key is non-empty and doesn't contain path separators.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if strings.ContainsAny(key, "/\\") {
		return fmt.Errorf("key %q contains path separator", key)
	}
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
