package kvstorage

import "errors"

var (
	// ErrKeyNotFound is returned when a key does not exist.
	ErrKeyNotFound = errors.New("key not found")

	// ErrAlreadyExists is returned when Set is called with FailIfExists
	// and the key already exists.
	ErrAlreadyExists = errors.New("key already exists")

	// ErrReservedTable is returned when a reserved table name is used.
	ErrReservedTable = errors.New("reserved table name")
)
