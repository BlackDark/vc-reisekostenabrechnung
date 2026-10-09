package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"testing"
)

func TestLocalContract(t *testing.T) {
	s, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exercise(t, s)
}

func TestS3Contract(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT is not set")
	}
	ctx := t.Context()
	s, err := OpenS3(S3Options{
		Endpoint:     endpoint,
		Region:       envDefault("S3_TEST_REGION", "us-east-1"),
		Bucket:       envDefault("S3_TEST_BUCKET", "rk-test"),
		AccessKey:    envDefault("S3_TEST_ACCESS_KEY", "minioadmin"),
		SecretKey:    envDefault("S3_TEST_SECRET_KEY", "minioadmin"),
		UsePathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	exercise(t, s)
}

func exercise(t *testing.T, s Store) {
	t.Helper()
	ctx := t.Context()
	key := "nutzer/test/belege/one/original-1.bin"
	body := []byte("receipt-bytes")
	sum := sha256.Sum256(body)
	hexSum := hex.EncodeToString(sum[:])
	if err := s.PutIfAbsent(ctx, key, bytes.NewReader(body), hexSum); err != nil {
		t.Fatal(err)
	}
	if err := s.PutIfAbsent(ctx, key, bytes.NewReader(body), hexSum); err != nil {
		t.Fatal("same bytes", err)
	}
	if err := s.PutIfAbsent(ctx, key, bytes.NewReader([]byte("other")), ""); !errors.Is(err, ErrExists) {
		t.Fatalf("overwrite: %v", err)
	}
	if err := s.PutIfAbsent(ctx, key+"-bad", bytes.NewReader(body), "00"); !errors.Is(err, ErrChecksum) {
		t.Fatalf("checksum: %v", err)
	}
	info, err := s.Stat(ctx, key)
	if err != nil || info.Size != int64(len(body)) || !bytes.Equal([]byte(hexSum), []byte(info.SHA256)) {
		t.Fatalf("stat %+v %v", info, err)
	}
	rc, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readAll(rc)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("get %q %v", got, err)
	}
	if _, err := s.Get(ctx, key+"-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := s.DeleteForRetention(ctx, key, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if err := s.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func readAll(r io.ReadCloser) ([]byte, error) {
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
