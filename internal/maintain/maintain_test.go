package maintain

import (
	"archive/tar"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reisekosten.db")
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t (v TEXT); INSERT INTO t VALUES ('kept')`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	files := filepath.Join(dir, "files")
	if err := os.MkdirAll(files, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(files, "note.txt"), []byte("beleg"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "backup.tar")
	if err := Backup(ctx, dbPath, files, out); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	tr := tar.NewReader(f)
	foundDB, foundNote := false, false
	var snap []byte
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		switch h.Name {
		case "reisekosten.db":
			foundDB = true
			snap = body
		case "files/note.txt":
			foundNote = string(body) == "beleg"
		}
	}
	if !foundDB || !foundNote {
		t.Fatalf("db %v note %v", foundDB, foundNote)
	}
	restored := filepath.Join(dir, "restored.db")
	if err := os.WriteFile(restored, snap, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := sql.Open("sqlite", "file:"+restored+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = got.Close() }()
	var v string
	if err := got.QueryRow(`SELECT v FROM t`).Scan(&v); err != nil || v != "kept" {
		t.Fatalf("%s %v", v, err)
	}
	if err := QuickCheck(ctx, restored); err != nil {
		t.Fatal(err)
	}
}
