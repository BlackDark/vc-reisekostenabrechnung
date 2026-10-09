package store

import (
	"context"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateEmptyAndIdempotent(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	v, err := s.DBVersion(ctx)
	if err != nil || v != 4 {
		t.Fatalf("version %d err %v", v, err)
	}
	n, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Ada", Benutzername: "ada", Sprache: "de", IstAdminLokal: true, Aktiv: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetNutzer(ctx, n.ID)
	if err != nil || got.Benutzername != "ada" {
		t.Fatalf("%+v %v", got, err)
	}
	if err := s.VerifyAudit(ctx); err != nil {
		t.Fatal(err)
	}
	ev, err := s.AuditAll(ctx)
	if err != nil || len(ev) != 1 {
		t.Fatalf("events %d %v", len(ev), err)
	}
	ev[0].IP = "tampered"
	// persisted chain must still verify; the copy is local
	if err := s.VerifyAudit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestVersionConflict(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	n, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Ada", Benutzername: "ada", Sprache: "de", Aktiv: true, IstAdminLokal: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProfil(ctx, n.ID, n.Version+9, "Ada", "en", nil, false, Actor{Art: "nutzer", NutzerID: &n.ID}); !errorsIs(err, ErrConflict) {
		t.Fatal(err)
	}
}

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func errorsIs(err, target error) bool {
	return err != nil && (err == target || (target != nil && err.Error() == target.Error()) || wrapIs(err, target))
}

func wrapIs(err, target error) bool {
	type unwrapper interface{ Unwrap() error }
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
