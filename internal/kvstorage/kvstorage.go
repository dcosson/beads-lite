// Package kvstorage defines a generic key-value storage interface for
// filesystem-backed persistence without issue lifecycle semantics.
// It is used for slots and other simple record types.
package kvstorage

import (
	"context"
	"fmt"

	"beads-lite/internal/issuestorage"
)

// KVStore defines the interface for generic key-value persistence.
// Each store operates on a single "table" (a named directory under .beads/).
type KVStore interface {
	// Set stores a value for the given key.
	// Behavior depends on opts.Exists:
	//   SetAlways (default): create or overwrite.
	//   FailIfExists: return ErrAlreadyExists if the key is already present.
	//   FailIfNotExists: return ErrKeyNotFound if the key doesn't exist.
	Set(ctx context.Context, key string, value []byte, opts SetOptions) error

	// Get retrieves the value for the given key.
	// Returns ErrKeyNotFound if the key doesn't exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key and its value.
	// Returns ErrKeyNotFound if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// List returns all keys in the table.
	List(ctx context.Context) ([]string, error)
}

// ExistsBehavior controls how Set handles pre-existing keys.
type ExistsBehavior int

const (
	SetAlways       ExistsBehavior = iota // create or overwrite (default)
	FailIfExists                          // create semantics: error if key already exists
	FailIfNotExists                       // update semantics: error if key doesn't exist
)

// SetOptions controls Set behavior.
type SetOptions struct {
	Exists ExistsBehavior
}

// ReservedTableNames are directory names used by issue storage.
// These cannot be used as KV table names.
var ReservedTableNames = issuestorage.ReservedDirs

// ValidateTableName checks that a table name is not reserved and is non-empty.
func ValidateTableName(name string) error {
	if name == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	for _, reserved := range ReservedTableNames {
		if name == reserved {
			return fmt.Errorf("table name %q is reserved by issue storage: %w", name, ErrReservedTable)
		}
	}
	return nil
}
