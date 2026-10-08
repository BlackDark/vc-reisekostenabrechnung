// Package id generates time-ordered UUIDv7 strings.
package id

import (
	"crypto/rand"
	"fmt"
	"time"
)

// NewV7 returns a UUIDv7 (RFC 9562) encoded as a canonical string.
func NewV7() (string, error) {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("uuid entropy: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Must is for tests and startup paths where entropy failure is fatal.
func Must() string {
	v, err := NewV7()
	if err != nil {
		panic(err)
	}
	return v
}
