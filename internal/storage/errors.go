package storage

import "errors"

var (
	ErrNotFound         = errors.New("issue not found")
	ErrAlreadyExists    = errors.New("issue already exists")
	ErrLockTimeout      = errors.New("could not acquire lock")
	ErrInvalidID        = errors.New("invalid issue ID")
	ErrCycle            = errors.New("operation would create a cycle")
	ErrMaxDepthExceeded = errors.New("maximum hierarchy depth exceeded")
)
