// Package maintain is the backup and database check used by the CLI.
package maintain

import (
	"archive/tar"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// QuickCheck opens the database and runs PRAGMA quick_check.
func QuickCheck(ctx context.Context, dbPath string) error {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var msgs []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return err
		}
		if msg != "ok" {
			msgs = append(msgs, msg)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(msgs) > 0 {
		return fmt.Errorf("quick_check: %s", strings.Join(msgs, "; "))
	}
	return nil
}

// Backup writes a tar with a consistent SQLite snapshot (VACUUM INTO) and the local file tree.
func Backup(ctx context.Context, dbPath, filesRoot, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "rk-backup-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	snap := filepath.Join(tmp, "reisekosten.db")
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if strings.Contains(snap, "'") {
		return fmt.Errorf("snapshot path")
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+snap+"'"); err != nil {
		return err
	}
	if err := QuickCheck(ctx, snap); err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	tw := tar.NewWriter(out)
	when := time.Now().UTC()
	if err := addFile(tw, snap, "reisekosten.db", when); err != nil {
		return err
	}
	var listed []string
	if filesRoot != "" {
		err = filepath.WalkDir(filesRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(filesRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			listed = append(listed, rel)
			return addFile(tw, path, "files/"+rel, when)
		})
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	manifest := strings.Join(listed, "\n")
	if manifest != "" {
		manifest += "\n"
	}
	hdr := &tar.Header{Name: "files.txt", Mode: 0o644, Size: int64(len(manifest)), ModTime: when, Format: tar.FormatPAX}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := io.WriteString(tw, manifest); err != nil {
		return err
	}
	return tw.Close()
}

func addFile(tw *tar.Writer, path, name string, when time.Time) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: st.Size(), ModTime: when, Format: tar.FormatPAX}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}
