package store

import (
	"context"
	"testing"
	"time"
)

// TestBelegQuelleUndMail covers the source columns: an ordinary upload defaults
// to "upload", and a mail-filed Beleg keeps its origin mail in the same
// transaction, so a Beleg never claims mail without provenance.
func TestBelegQuelleUndMail(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	n, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Ada", Benutzername: "ada", Sprache: "de", Aktiv: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{Art: "nutzer", NutzerID: &n.ID}

	plain, err := s.CreateBeleg(ctx, n.ID, NewBeleg{
		Typ: "foto", Seiten: 1, SHA256: "aaa", Status: "zur_bestaetigung",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Quelle != "upload" {
		t.Fatalf("default quelle is %q, want upload", plain.Quelle)
	}
	if _, err := s.GetBelegMail(ctx, plain.ID); !errorsIs(err, ErrNotFound) {
		t.Fatalf("an upload should have no origin mail, got %v", err)
	}

	filed, err := s.CreateBeleg(ctx, n.ID, NewBeleg{
		Typ: "foto", Seiten: 1, SHA256: "bbb", Status: "zur_bestaetigung",
		Quelle: "mail",
		Mail: &BelegMailInput{
			Absender: "sender@example.com", Betreff: "Your receipt",
			MailDatum: "2026-10-14T09:30:00Z", EmlSchluessel: "mail/ada/x.eml",
		},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if filed.Quelle != "mail" {
		t.Fatalf("quelle is %q, want mail", filed.Quelle)
	}
	mail, err := s.GetBelegMail(ctx, filed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if mail.Absender != "sender@example.com" || mail.Betreff != "Your receipt" ||
		mail.EmlSchluessel != "mail/ada/x.eml" {
		t.Fatalf("origin mail wrong: %+v", mail)
	}
	got, perr := time.Parse(time.RFC3339, "2026-10-14T09:30:00Z")
	if mail.MailDatum == nil || perr != nil || *mail.MailDatum != got.Format(time.RFC3339) {
		t.Fatalf("mail date wrong: %v", mail.MailDatum)
	}

	// A page of Belege resolves every origin mail in one query.
	batch, err := s.ListBelegMailByIDs(ctx, []string{plain.ID, filed.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != 1 || batch[0].BelegID != filed.ID {
		t.Fatalf("batch lookup returned %+v", batch)
	}
}
