package store

import (
	"context"
	"testing"
	"time"
)

// A load must not wait for the write connection. The write pool holds one
// connection, and an upload or an export keeps it busy for whole seconds while
// the browser keeps reading trips. Holding that connection open therefore must
// not stop a load; before the loaders moved to the read pool, every call below
// waited for the connection and gave up on the deadline instead.
func TestLoadsRunWhileAWriteIsOpen(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	ada := mustNutzer(t, s, "ada")
	actor := Actor{Art: "nutzer", NutzerID: &ada.ID}
	ag, err := s.CreateArbeitgeber(ctx, ada.ID, ArbeitgeberInput{Name: "ACME", Anschrift: "Weg 1"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	reise := mustReise(t, s, ada.ID, actor, ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "Termin",
		BeginnLokal: "2026-04-13T18:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-04-14T10:00:00", EndeZone: "Europe/Berlin",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-04-13T20:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	})
	abrechnung, err := s.CreateAbrechnung(ctx, ada.ID, AbrechnungInput{
		ArbeitgeberID: ag.ID, ZeitraumArt: "monat", Von: "2026-04-01", Bis: "2026-04-30",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}

	held, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Rollback() })

	loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.GetReiseBundle(loadCtx, ada.ID, reise.Reise.ID); err != nil {
		t.Fatalf("load a Reise while a write is open: %v", err)
	}
	if _, err := s.ListReisenAll(loadCtx, ada.ID); err != nil {
		t.Fatalf("list the Reisen while a write is open: %v", err)
	}
	if _, err := s.ListFahrten(loadCtx, ada.ID, reise.Reise.ID); err != nil {
		t.Fatalf("list the Fahrten while a write is open: %v", err)
	}
	if _, err := s.GetAbrechnungBundle(loadCtx, ada.ID, abrechnung.Row.ID); err != nil {
		t.Fatalf("load an Abrechnung while a write is open: %v", err)
	}
	if _, err := s.EffectiveYear(loadCtx, 2026); err != nil {
		t.Fatalf("load a Satz year while a write is open: %v", err)
	}
}
