package store

import (
	"context"
	"testing"
	"time"
)

func TestMailAllowPrecedence(t *testing.T) {
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
	now := time.Now().UTC().Format(time.RFC3339)
	sa := Actor{Art: "nutzer", NutzerID: &a.ID}
	sb := Actor{Art: "nutzer", NutzerID: &b.ID}
	if err := s.AddMailErlaubt(ctx, a.ID, "x@example.com", now, sa); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMailErlaubt(ctx, b.ID, "@example.com", now, sb); err != nil {
		t.Fatal(err)
	}
	if owner, ok, _ := s.MailOwnerForAbsender(ctx, "x@example.com"); !ok || owner != a.ID {
		t.Fatalf("exact address must win, got owner=%v ok=%v want a.ID=%s", owner, ok, a.ID)
	}
	if owner, ok, _ := s.MailOwnerForAbsender(ctx, "someone@example.com"); !ok || owner != b.ID {
		t.Fatalf("domain fallback, got owner=%v ok=%v want b.ID=%s", owner, ok, b.ID)
	}
}
