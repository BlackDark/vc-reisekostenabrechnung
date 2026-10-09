package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Local is a directory tree. Writes land in a temp file, are fsynced, then renamed.
type Local struct {
	root string
}

// OpenLocal creates root if needed.
func OpenLocal(root string) (*Local, error) {
	if root == "" {
		return nil, errors.New("storage root is empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{root: root}, nil
}

func (l *Local) PutIfAbsent(_ context.Context, key string, r io.Reader, sha256hex string) error {
	path, err := safe(l.root, key)
	if err != nil {
		return err
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		got, err := hashFile(path)
		if err != nil {
			return err
		}
		incoming, err := hashReader(r)
		if err != nil {
			return err
		}
		if sha256hex != "" && !strings.EqualFold(incoming, sha256hex) {
			return ErrChecksum
		}
		if strings.EqualFold(got, incoming) {
			return nil
		}
		return ErrExists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".put-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), r); err != nil {
		_ = tmp.Close()
		return err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sha256hex != "" && !strings.EqualFold(sum, sha256hex) {
		_ = tmp.Close()
		return ErrChecksum
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	side := path + ".sha256"
	if err := os.WriteFile(side, []byte(sum), 0o644); err != nil {
		return err
	}
	return nil
}

func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := safe(l.root, key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l *Local) Stat(_ context.Context, key string) (Info, error) {
	path, err := safe(l.root, key)
	if err != nil {
		return Info{}, err
	}
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Info{}, ErrNotFound
	}
	if err != nil {
		return Info{}, err
	}
	sum, err := os.ReadFile(path + ".sha256")
	if err != nil {
		got, herr := hashFile(path)
		if herr != nil {
			return Info{}, herr
		}
		sum = []byte(got)
	}
	return Info{Size: st.Size(), SHA256: strings.TrimSpace(string(sum))}, nil
}

func (l *Local) DeleteForRetention(_ context.Context, key, _ string) error {
	path, err := safe(l.root, key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	_ = os.Remove(path + ".sha256")
	return err
}

func (l *Local) Ready(context.Context) error {
	f, err := os.CreateTemp(l.root, ".ready-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func hashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func safe(root, key string) (string, error) {
	if key == "" || strings.Contains(key, "..") || filepath.IsAbs(key) {
		return "", errors.New("invalid storage key")
	}
	root = filepath.Clean(root)
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(key)))
	if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return "", errors.New("invalid storage key")
	}
	return path, nil
}
