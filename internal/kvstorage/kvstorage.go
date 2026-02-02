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
	// If opts.FailIfExists is true and the key already exists, returns ErrAlreadyExists.
	// Otherwise, overwrites the existing value.
	Set(ctx context.Context, key string, value []byte, opts SetOptions) error

	// Get retrieves the value for the given key.
	// Returns ErrKeyNotFound if the key doesn't exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Update replaces the value for an existing key.
	// Returns ErrKeyNotFound if the key doesn't exist.
	Update(ctx context.Context, key string, value []byte) error

	// Delete removes a key and its value.
	// Returns ErrKeyNotFound if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// List returns all keys in the table.
	List(ctx context.Context) ([]string, error)
}

// SetOptions controls Set behavior.
type SetOptions struct {
	// FailIfExists causes Set to return ErrAlreadyExists if the key is already present.
	FailIfExists bool
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
