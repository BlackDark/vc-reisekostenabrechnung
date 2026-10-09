package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3 is an S3-compatible bucket (MinIO, Garage, Hetzner, …).
type S3 struct {
	client *minio.Client
	bucket string
	prefix string
	region string
}

// S3Options configures the client. Endpoint may include a scheme.
type S3Options struct {
	Endpoint     string
	Region       string
	Bucket       string
	Prefix       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// OpenS3 builds a client. It does not require the bucket to exist yet.
func OpenS3(opt S3Options) (*S3, error) {
	endpoint, secure, err := splitEndpoint(opt.Endpoint)
	if err != nil {
		return nil, err
	}
	lookup := minio.BucketLookupAuto
	if opt.UsePathStyle {
		lookup = minio.BucketLookupPath
	}
	cl, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(opt.AccessKey, opt.SecretKey, ""),
		Secure:       secure,
		Region:       opt.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, err
	}
	prefix := strings.Trim(opt.Prefix, "/")
	return &S3{client: cl, bucket: opt.Bucket, prefix: prefix, region: opt.Region}, nil
}

// EnsureBucket creates the bucket when it is missing. Contract tests use this.
func (s *S3) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: s.region})
}

func (s *S3) object(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", errors.New("invalid storage key")
	}
	if s.prefix == "" {
		return key, nil
	}
	return s.prefix + "/" + key, nil
}

func (s *S3) PutIfAbsent(ctx context.Context, key string, r io.Reader, sha256hex string) error {
	obj, err := s.object(key)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if sha256hex != "" && !strings.EqualFold(got, sha256hex) {
		return ErrChecksum
	}
	info, err := s.client.StatObject(ctx, s.bucket, obj, minio.StatObjectOptions{})
	if err == nil {
		if same, cmpErr := s.sameBytes(ctx, obj, body, got, info); cmpErr != nil {
			return cmpErr
		} else if same {
			return nil
		}
		return ErrExists
	}
	if minio.ToErrorResponse(err).Code != "NoSuchKey" {
		return err
	}
	_, err = s.client.PutObject(ctx, s.bucket, obj, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{
		UserMetadata: map[string]string{"Sha256": got},
	})
	return err
}

func (s *S3) sameBytes(ctx context.Context, obj string, body []byte, got string, info minio.ObjectInfo) (bool, error) {
	prev := info.UserMetadata["Sha256"]
	if prev == "" {
		prev = info.UserMetadata["sha256"]
	}
	if prev != "" && !strings.EqualFold(prev, got) {
		return false, nil
	}
	if prev == "" && info.Size != int64(len(body)) {
		return false, nil
	}
	rc, err := s.client.GetObject(ctx, s.bucket, obj, minio.GetObjectOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = rc.Close() }()
	existing, err := io.ReadAll(rc)
	if err != nil {
		return false, err
	}
	have := sha256.Sum256(existing)
	sum := sha256.Sum256(body)
	return bytes.Equal(have[:], sum[:]), nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.object(key)
	if err != nil {
		return nil, err
	}
	rc, err := s.client.GetObject(ctx, s.bucket, obj, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := rc.Stat(); err != nil {
		_ = rc.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rc, nil
}

func (s *S3) Stat(ctx context.Context, key string) (Info, error) {
	obj, err := s.object(key)
	if err != nil {
		return Info{}, err
	}
	info, err := s.client.StatObject(ctx, s.bucket, obj, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return Info{}, ErrNotFound
		}
		return Info{}, err
	}
	sum := info.UserMetadata["Sha256"]
	if sum == "" {
		sum = info.UserMetadata["sha256"]
	}
	return Info{Size: info.Size, SHA256: sum}, nil
}

func (s *S3) DeleteForRetention(ctx context.Context, key, _ string) error {
	obj, err := s.object(key)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.bucket, obj, minio.RemoveObjectOptions{})
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return nil
	}
	return err
}

func (s *S3) Ready(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("bucket does not exist")
	}
	return nil
}

func splitEndpoint(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, errors.New("s3 endpoint is empty")
	}
	if !strings.Contains(raw, "://") {
		return raw, false, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false, errors.New("s3 endpoint is invalid")
	}
	return u.Host, u.Scheme == "https", nil
}
