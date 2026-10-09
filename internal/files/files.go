// Package files stores blobs on the local disk, keyed by a relative path.
package files

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Write creates key under root, replacing any existing file.
func Write(root, key string, body []byte) error {
	path, err := safe(root, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// Read returns the bytes stored at key.
func Read(root, key string) ([]byte, error) {
	path, err := safe(root, key)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	return b, err
}

// Remove deletes key. A missing file is not an error.
func Remove(root, key string) error {
	path, err := safe(root, key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
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
