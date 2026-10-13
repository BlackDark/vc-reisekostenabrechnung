package store

import (
	"context"
	"testing"
	"time"
)

func TestMailAllowlist(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a, err := s.CreateNutzer(ctx, NewNutzer{Anzeigename: "A", Benutzername: "ada", Sprache: "de", Aktiv: true}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateNutzer(ctx, NewNutzer{Anzeigename: "B", Benutzername: "bob", Sprache: "de", Aktiv: true}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{Art: "nutzer", NutzerID: &a.ID}
	now := time.Now().UTC().Format(time.RFC3339)

	if w, ok := NormalizeMailAllow(" Hotel@Example.COM "); !ok || w != "hotel@example.com" {
		t.Fatalf("normalize address: %q %v", w, ok)
	}
	if w, ok := NormalizeMailAllow("@Gmail.com"); !ok || w != "@gmail.com" {
		t.Fatalf("normalize domain: %q %v", w, ok)
	}
	if _, ok := NormalizeMailAllow("not-an-email"); ok {
		t.Fatal("invalid entry accepted")
	}

	if err := s.AddMailErlaubt(ctx, a.ID, "hotel@example.com", now, actor); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMailErlaubt(ctx, a.ID, "@booking.com", now, actor); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListMailErlaubt(ctx, a.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("list a: %d %v", len(rows), err)
	}
	if rows, _ := s.ListMailErlaubt(ctx, b.ID); len(rows) != 0 {
		t.Fatalf("list b not empty: %d", len(rows))
	}
	if owner, ok, _ := s.MailOwnerForAbsender(ctx, "hotel@example.com"); !ok || owner != a.ID {
		t.Fatalf("exact match: %q %v", owner, ok)
	}
	if owner, ok, _ := s.MailOwnerForAbsender(ctx, "receipt@booking.com"); !ok || owner != a.ID {
		t.Fatalf("domain match: %q %v", owner, ok)
	}
	if _, ok, _ := s.MailOwnerForAbsender(ctx, "someone@unknown.test"); ok {
		t.Fatal("unexpected match")
	}

	done, err := s.RemoveMailErlaubt(ctx, a.ID, "hotel@example.com", actor)
	if err != nil || !done {
		t.Fatalf("remove: %v %v", done, err)
	}
	if _, ok, _ := s.MailOwnerForAbsender(ctx, "hotel@example.com"); ok {
		t.Fatal("removed address still matches")
	}
	if done, _ := s.RemoveMailErlaubt(ctx, a.ID, "hotel@example.com", actor); done {
		t.Fatal("second remove should be a no-op")
	}
	if err := s.VerifyAudit(ctx); err != nil {
		t.Fatal(err)
	}
}
