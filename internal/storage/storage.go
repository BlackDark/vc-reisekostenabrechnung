// Package storage stores immutable blobs on a local volume or on S3.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrExists means the key is already stored with different bytes.
var ErrExists = errors.New("object exists")

// ErrChecksum means the bytes do not match the expected SHA-256.
var ErrChecksum = errors.New("checksum mismatch")

// ErrNotFound means the key is absent.
var ErrNotFound = errors.New("not found")

// Info describes a stored object.
type Info struct {
	Size   int64
	SHA256 string
}

// Store is the file storage contract (SPEC 12.1).
type Store interface {
	// PutIfAbsent writes key once. A second write of the same bytes succeeds.
	// A second write of different bytes returns ErrExists. sha256 is hex.
	PutIfAbsent(ctx context.Context, key string, r io.Reader, sha256hex string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Stat(ctx context.Context, key string) (Info, error)
	// DeleteForRetention removes a key. Callers must not use it for an Archivbeleg
	// outside the retention process. auditRef is recorded by the caller, not here.
	DeleteForRetention(ctx context.Context, key, auditRef string) error
	Ready(ctx context.Context) error
}
