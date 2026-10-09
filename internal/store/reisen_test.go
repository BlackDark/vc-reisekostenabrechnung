package store

import (
	"context"
	"errors"
	"testing"
)

func TestReiseDaysAndTenant(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	owner := Actor{Art: "nutzer"}
	ada, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Ada", Benutzername: "ada", Sprache: "de", Aktiv: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Bob", Benutzername: "bob", Sprache: "de", Aktiv: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	owner.NutzerID = &ada.ID
	ag, err := s.CreateArbeitgeber(ctx, ada.ID, ArbeitgeberInput{Name: "ACME", Anschrift: "Weg 1"}, owner)
	if err != nil {
		t.Fatal(err)
	}
	in := ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "Kundentermin", Projekt: "Alpha",
		BeginnLokal: "2026-09-07T20:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-09-10T18:00:00", EndeZone: "Europe/Berlin",
		UnterkunftDefault: "gestellt",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-09-08T02:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "FR", Satzort: "FR-PARIS", Ort: "Paris",
		}},
	}
	bundle, err := s.CreateReise(ctx, ada.ID, in, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Tage) != 4 {
		t.Fatalf("days %d", len(bundle.Tage))
	}
	for i, day := range bundle.Tage {
		if i < 3 && day.Unterkunft != "gestellt" {
			t.Fatalf("unterkunft %s", day.Unterkunft)
		}
		if i == 3 && day.Unterkunft != "keine" {
			t.Fatalf("last %s", day.Unterkunft)
		}
	}
	if _, err := s.GetReiseBundle(ctx, bob.ID, bundle.Reise.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	in.EndeLokal = "2026-09-11T18:00:00"
	updated, err := s.UpdateReise(ctx, ada.ID, bundle.Reise.ID, bundle.Reise.Version, in, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Tage) != 5 || updated.Tage[0].Datum != "2026-09-07" || updated.Tage[4].Datum != "2026-09-11" {
		t.Fatalf("%+v", updated.Tage)
	}
	_, err = s.PatchReisetag(ctx, ada.ID, bundle.Reise.ID, "2026-09-08", updated.Reise.Version, ReisetagInput{
		LandISO: "AT", Unterkunft: "gestellt",
	}, owner)
	var code *CodeError
	if !errors.As(err, &code) || code.Code != "begruendung_fehlt" {
		t.Fatal(err)
	}
	patched, err := s.PatchReisetag(ctx, ada.ID, bundle.Reise.ID, "2026-09-08", updated.Reise.Version, ReisetagInput{
		LandISO: "AT", Begruendung: "Grenzort", Unterkunft: "gestellt", Mittag: true,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, day := range patched.Tage {
		if day.Datum == "2026-09-08" && day.LandManuell != nil && *day.LandManuell == "AT" && day.MittagGestellt {
			found = true
		}
	}
	if !found {
		t.Fatal("override missing")
	}
	ev, err := s.AuditAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	logged := false
	for _, e := range ev {
		if e.Aktion == "reisetag.geaendert" && e.Grund != nil && *e.Grund == "Grenzort" {
			logged = true
		}
	}
	if !logged {
		t.Fatal("override was not audited")
	}
	fahrt, err := s.CreateFahrt(ctx, ada.ID, bundle.Reise.ID, FahrtInput{
		Datum: "2026-09-08", Start: "Wohnung", Ziel: "Paris", Fahrzeugart: "kraftwagen", Km: 10,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateFahrt(ctx, bob.ID, fahrt.ID, fahrt.Version, FahrtInput{
		Datum: "2026-09-08", Start: "X", Ziel: "Y", Fahrzeugart: "kraftwagen", Km: 10,
	}, Actor{Art: "nutzer", NutzerID: &bob.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	names, err := s.ListProjekte(ctx, ada.ID, "alp", 10)
	if err != nil || len(names) != 1 || names[0] != "Alpha" {
		t.Fatalf("%v %v", names, err)
	}
}
