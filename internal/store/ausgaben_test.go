package store

import (
	"context"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/fx"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
)

func TestPersistGoldenAusgaben(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	cat, err := satz.Load()
	if err != nil {
		t.Fatal(err)
	}
	ada := mustNutzer(t, s, "ada")
	actor := Actor{Art: "nutzer", NutzerID: &ada.ID}
	ag, err := s.CreateArbeitgeber(ctx, ada.ID, ArbeitgeberInput{Name: "ACME", Anschrift: "Weg 1"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	beleg, err := s.CreateBeleg(ctx, ada.ID, NewBeleg{
		Typ: "foto", Seiten: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Status: "bestaetigt",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}

	g15 := mustReise(t, s, ada.ID, actor, ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "Hotelbeleg",
		BeginnLokal: "2026-04-13T18:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-04-14T10:00:00", EndeZone: "Europe/Berlin",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-04-13T20:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	})
	if _, err := s.PatchReisetag(ctx, ada.ID, g15.Reise.ID, "2026-04-13", g15.Reise.Version, ReisetagInput{Unterkunft: "beleg"}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAusgabe(ctx, ada.ID, g15.Reise.ID, AusgabeInput{
		Kostenart: "uebernachtung", Datum: "2026-04-13", Betrag: 12127, Waehrung: "EUR",
		Naechte: []string{"2026-04-13"}, Fruehstueck: true, BelegIDs: []string{beleg.ID},
		Anteile: []AnteilInput{
			{Satz: 700, Steuerland: "DE", Brutto: 11770},
			{Satz: 1900, Steuerland: "DE", Brutto: 357},
		},
	}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAusgabe(ctx, ada.ID, g15.Reise.ID, AusgabeInput{
		Kostenart: "reisenebenkosten", Datum: "2026-04-13", Betrag: 1190, Waehrung: "EUR", BelegIDs: []string{beleg.ID},
		Anteile: []AnteilInput{{Satz: 1900, Steuerland: "DE", Brutto: 1190}},
	}, actor); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyMealSuggestions(ctx, ada.ID, g15.Reise.ID, actor); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.GetReiseBundle(ctx, ada.ID, g15.Reise.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Ausgaben) != 2 {
		t.Fatalf("ausgaben %d", len(reloaded.Ausgaben))
	}
	for _, item := range reloaded.Ausgaben {
		if len(item.Belege) != 1 || item.Belege[0].ID != beleg.ID {
			t.Fatalf("links %+v", item.Belege)
		}
	}
	got := berechneOne(t, cat, CalcReise(reloaded), nil)
	if got.Uebernachtung != 12127 || got.Reisenebenkosten != 1190 || got.Vorsteuer != 1017 {
		t.Fatalf("G15 %+v", got)
	}
	var netto, steuer int64
	for _, line := range got.Ausgaben {
		if line.Kostenart == "uebernachtung" && len(line.Anteile) > 0 {
			netto, steuer = line.Anteile[0].Netto, line.Anteile[0].Steuer
		}
	}
	if netto != 11000 || steuer != 770 {
		t.Fatalf("shares %+v", got.Ausgaben)
	}
	breakfast := false
	for _, day := range reloaded.Tage {
		if day.Datum == "2026-04-14" && day.FruehstueckGestellt {
			breakfast = true
		}
	}
	if !breakfast {
		t.Fatal("breakfast suggestion")
	}

	g16 := mustReise(t, s, ada.ID, actor, ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "Taxi",
		BeginnLokal: "2026-09-12T10:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-09-12T11:00:00", EndeZone: "Europe/Berlin",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-09-12T10:15:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "sonstiges", LandISO: "DE",
		}},
	})
	if _, err := s.CreateAusgabe(ctx, ada.ID, g16.Reise.ID, AusgabeInput{
		Kostenart: "fahrtkosten", Datum: "2026-09-12", Waehrung: "USD", Betrag: 4850,
		Kurs: "1.1650", KursQuelle: "ezb", KursDatum: "2026-09-11", BetragEUR: 4163,
		Anteile: []AnteilInput{{Satz: 0, Steuerland: "DE", Brutto: 4850, BruttoEUR: 4163}},
	}, actor); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.GetReiseBundle(ctx, ada.ID, g16.Reise.ID)
	if err != nil {
		t.Fatal(err)
	}
	var kurse []berechnung.Kurs
	for _, rate := range fx.EmbeddedECB() {
		kurse = append(kurse, berechnung.Kurs{Waehrung: rate.Waehrung, Datum: rate.Datum, Kurs: rate.Kurs})
	}
	got = berechneOne(t, cat, CalcReise(reloaded), kurse)
	if got.Fahrtkosten != 4163 || got.Ausgaben[0].Kurs != "1.1650" || got.Ausgaben[0].KursDatum != "2026-09-11" {
		t.Fatalf("G16 %+v", got)
	}

	charge := mustReise(t, s, ada.ID, actor, ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "Taxi belastet",
		BeginnLokal: "2026-09-13T10:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-09-13T11:00:00", EndeZone: "Europe/Berlin",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-09-13T10:15:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "sonstiges", LandISO: "DE",
		}},
	})
	if _, err := s.CreateAusgabe(ctx, ada.ID, charge.Reise.ID, AusgabeInput{
		Kostenart: "fahrtkosten", Datum: "2026-09-12", Waehrung: "USD", Betrag: 4850,
		KursQuelle: "belastung", Kurs: berechnung.ImplicitKurs(4850, 4237), BetragEUR: 4237,
		Anteile: []AnteilInput{{Satz: 0, Steuerland: "DE", Brutto: 4850, BruttoEUR: 4237}},
	}, actor); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.GetReiseBundle(ctx, ada.ID, charge.Reise.ID)
	if err != nil {
		t.Fatal(err)
	}
	got = berechneOne(t, cat, CalcReise(reloaded), nil)
	if got.Fahrtkosten != 4237 || got.Ausgaben[0].Kurs != "1.144678" {
		t.Fatalf("G16 belastung %+v", got)
	}

	g17 := mustReise(t, s, ada.ID, actor, ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "Bewirtung",
		BeginnLokal: "2026-03-11T08:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-03-13T18:00:00", EndeZone: "Europe/Berlin",
		UnterkunftDefault: "gestellt",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-03-11T10:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	})
	patched, err := s.PatchReisetag(ctx, ada.ID, g17.Reise.ID, "2026-03-12", g17.Reise.Version, ReisetagInput{
		Unterkunft: "gestellt", Abend: true,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	_ = patched
	if _, err := s.CreateAusgabe(ctx, ada.ID, g17.Reise.ID, AusgabeInput{
		Kostenart: "bewirtung", Datum: "2026-03-12", Waehrung: "EUR", Betrag: 23800, Mahlzeit: "abend",
		Anteile: []AnteilInput{
			{Satz: 700, Steuerland: "DE", Brutto: 16050},
			{Satz: 1900, Steuerland: "DE", Brutto: 5950},
			{Satz: 0, Steuerland: "DE", Brutto: 1800},
		},
	}, actor); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.GetReiseBundle(ctx, ada.ID, g17.Reise.ID)
	if err != nil {
		t.Fatal(err)
	}
	got = berechneOne(t, cat, CalcReise(reloaded), nil)
	if got.Verpflegung != 4480 || got.Bewirtung != 23800 || got.BewirtungB != 21800 || got.BewirtungAbziehbar != 15260 || got.BewirtungNichtAbziehbar != 6540 || got.Vorsteuer != 2000 {
		t.Fatalf("G17 %+v", got)
	}
	var dayOK bool
	for _, day := range got.Tage {
		if day.Datum == "2026-03-12" && day.Tagesart == "zwischentag" && day.Ergebnis == 1680 {
			dayOK = true
		}
	}
	if !dayOK {
		t.Fatalf("day %+v", got.Tage)
	}

	if _, err := s.CreateVorschuss(ctx, ada.ID, VorschussInput{ArbeitgeberID: ag.ID, Datum: "2026-03-01", Betrag: 30000}, actor); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListVorschuesse(ctx, ada.ID)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	sum, pay := berechnung.Auszahlung(41240, []int64{rows[0].Betrag})
	if sum != 30000 || pay != 11240 {
		t.Fatalf("G19 %d %d", sum, pay)
	}
	if _, err := s.UpdateVorschuss(ctx, ada.ID, rows[0].ID, rows[0].Version, VorschussInput{Datum: "2026-03-01", Betrag: 50000}, actor); err != nil {
		t.Fatal(err)
	}
	rows, err = s.ListVorschuesse(ctx, ada.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, pay = berechnung.Auszahlung(41240, []int64{rows[0].Betrag})
	if pay != -8760 {
		t.Fatalf("repay %d", pay)
	}
}

func mustReise(t *testing.T, s *Store, nutzerID string, actor Actor, in ReiseInput) ReiseBundle {
	t.Helper()
	b, err := s.CreateReise(context.Background(), nutzerID, in, actor)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func berechneOne(t *testing.T, cat *satz.Catalog, trip berechnung.Reise, kurse []berechnung.Kurs) berechnung.ReiseErgebnis {
	t.Helper()
	got, err := berechnung.Berechne(berechnung.Eingabe{Jahre: cat.Years, Reisen: []berechnung.Reise{trip}, Kurse: kurse})
	if err != nil {
		t.Fatal(err)
	}
	return got.Reisen[0]
}
