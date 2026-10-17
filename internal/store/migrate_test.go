package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/audit"
	_ "modernc.org/sqlite"
)

func TestMigrateEmptyAndIdempotent(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	v, err := s.DBVersion(ctx)
	if err != nil || v != 16 {
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
	ev := allAudit(t, s)
	if len(ev) != 1 {
		t.Fatalf("events %d", len(ev))
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

// allAudit collects the whole chain, which the store reads in pages so that
// verifying it does not have to hold it all at once.
func allAudit(t *testing.T, s *Store) []audit.Event {
	t.Helper()
	var out []audit.Event
	err := s.WalkAudit(context.Background(), func(e audit.Event) error {
		out = append(out, e)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The real chain is read pages of five hundred at a time, so the walk has to
// continue from the last event of one page and stop after a short one. Five
// events over pages of two cross that boundary twice, and the checker has to
// carry the previous hash over it, or the chain would look broken there.
func TestWalkAuditReadsTheChainInPages(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	n := mustNutzer(t, s, "ada")
	actor := Actor{Art: "nutzer", NutzerID: &n.ID}
	for i := range 5 {
		if err := s.RecordAudit(ctx, actor, fmt.Sprintf("test.eintrag.%d", i), "nutzer", n.ID); err != nil {
			t.Fatal(err)
		}
	}
	want := allAudit(t, s)
	if len(want) != 6 {
		t.Fatalf("chain has %d events, want the account plus five", len(want))
	}

	var got []audit.Event
	var checker audit.Checker
	err := s.walkAudit(ctx, 2, func(e audit.Event) error {
		got = append(got, e)
		return checker.Add(e)
	})
	if err != nil {
		t.Fatalf("paged walk of an intact chain: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("paged walk saw %d events, the chain has %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Hash != want[i].Hash {
			t.Fatalf("event %d is %s, want %s", i, got[i].Hash, want[i].Hash)
		}
	}
	if _, err := s.write.ExecContext(ctx,
		"UPDATE audit_ereignis SET aktion = 'geaendert' WHERE nr = 4"); err != nil {
		t.Fatal(err)
	}
	var after audit.Checker
	if err := s.walkAudit(ctx, 2, after.Add); err == nil {
		t.Fatal("a rewritten event in a later page was not caught")
	}
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
