package blob

import "errors"

// ErrNotFound indicates the requested object does not exist in the store.
var ErrNotFound = errors.New("blob not found")

// ErrStorage wraps any non-not-found failure from the underlying blob backend
// (e.g. S3 connectivity/permission errors). Callers can use errors.Is to map
// storage failures to a 500 while keeping the original cause via %w.
var ErrStorage = errors.New("blob storage error")
