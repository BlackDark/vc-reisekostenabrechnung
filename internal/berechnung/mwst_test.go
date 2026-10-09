package berechnung

import (
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
)

func TestSplitHelpers(t *testing.T) {
	food := SplitGastronomie(10000)
	if food[0].Brutto != 7000 || food[0].Satz != 700 || food[1].Brutto != 3000 || food[1].Satz != 1900 {
		t.Fatalf("%+v", food)
	}
	hotel := SplitHotel(10000, 2026)
	if hotel[0].Brutto != 1500 || hotel[1].Brutto != 8500 {
		t.Fatalf("2026 %+v", hotel)
	}
	old := SplitHotel(10000, 2025)
	if old[0].Brutto != 2000 || old[1].Brutto != 8000 {
		t.Fatalf("2025 %+v", old)
	}
	eur, err := EURFromKurs(4850, "1.1650")
	if err != nil || eur != 4163 {
		t.Fatalf("eur %d %v", eur, err)
	}
	if ImplicitKurs(4850, 4237) != "1.144678" {
		t.Fatal(ImplicitKurs(4850, 4237))
	}
}

func TestWarningCodes(t *testing.T) {
	cat, err := satz.Load()
	if err != nil {
		t.Fatal(err)
	}
	base := Reise{
		ID: "r", Anlass: "x", ArbeitgeberName: "ACME",
		Beginn: Zeitpunkt{Lokal: "2026-09-12T08:00:00", Zone: "Europe/Berlin"},
		Ende:   Zeitpunkt{Lokal: "2026-09-12T18:00:00", Zone: "Europe/Berlin"},
		Ortswechsel: []Ortswechsel{{
			Ankunft:        Zeitpunkt{Lokal: "2026-09-12T09:00:00", Zone: "Europe/Berlin"},
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	}
	in := func() *Eingabe { return &Eingabe{Jahre: cat.Years} }
	line := Ausgabe{
		ID: "a", Kostenart: "fahrtkosten", Datum: "2026-09-12", Waehrung: "EUR", Betrag: 1000,
		Rechnungsart: "rechnung", PruefeBeleg: true, Anteile: []SteuerInput{{Satz: 1900, Steuerland: "DE", Brutto: 1000}},
	}
	_, warns, _ := convertAusgabe(in(), base, line)
	if !containsStr(warns, "W01") {
		t.Fatalf("W01 %v", warns)
	}
	line.BelegAnzahl = 1
	line.BelegOffen = true
	_, warns, _ = convertAusgabe(in(), base, line)
	if !containsStr(warns, "B01") || containsStr(warns, "W01") {
		t.Fatalf("B01 %v", warns)
	}
	line.BelegOffen = false
	line.Eigenbeleg = true
	_, warns, _ = convertAusgabe(in(), base, line)
	if !containsStr(warns, "W05") {
		t.Fatalf("W05 %v", warns)
	}
	big := line
	big.Eigenbeleg = false
	big.BelegAnzahl = 1
	big.PruefeBeleg = false
	big.Betrag = 30000
	big.Anteile = []SteuerInput{{Satz: 1900, Steuerland: "DE", Brutto: 30000}}
	big.Empfaenger = "Jemand"
	_, warns, _ = convertAusgabe(in(), base, big)
	if !containsStr(warns, "W02") {
		t.Fatalf("W02 %v", warns)
	}
	fxLine := Ausgabe{
		ID: "fx", Kostenart: "fahrtkosten", Datum: "2026-09-12", Waehrung: "USD", Betrag: 4850,
		Monatskurs: "1.170200", Anteile: []SteuerInput{{Satz: 1900, Steuerland: "DE", Brutto: 4850}},
	}
	rates := in()
	rates.Kurse = []Kurs{{Waehrung: "USD", Datum: "2026-09-11", Kurs: "1.1650"}}
	ae, warns, block := convertAusgabe(rates, base, fxLine)
	if len(block) > 0 || ae.BetragEUR != 4163 || !containsStr(warns, "H-UST-KURS") || ae.Monatskurs == "" {
		t.Fatalf("fx %+v %v %v", ae, warns, block)
	}
	bew := Ausgabe{
		ID: "b", Kostenart: "bewirtung", Datum: "2026-09-12", Waehrung: "EUR", Betrag: 5000,
		Bewirtung: &Bewirtung{}, Anteile: []SteuerInput{{Satz: 1900, Steuerland: "DE", Brutto: 5000}},
	}
	_, warns, block = convertAusgabe(in(), base, bew)
	if len(block) > 0 || !containsStr(warns, "B03") || !containsStr(warns, "W06") {
		t.Fatalf("bew %v %v", warns, block)
	}
	bew.TSE = true
	bew.Bewirtung = &Bewirtung{Anlass: "Essen", Teilnehmer: 1, Ort: "Berlin", Bestaetigt: true}
	_, warns, _ = convertAusgabe(in(), base, bew)
	if containsStr(warns, "B03") || containsStr(warns, "W06") {
		t.Fatalf("complete %v", warns)
	}
	meal := base
	meal.Tage = map[string]TagEingabe{"2026-09-12": {Mittag: true}}
	meal.Beginn = Zeitpunkt{Lokal: "2026-03-10T08:00:00", Zone: "Europe/Berlin"}
	meal.Ende = Zeitpunkt{Lokal: "2026-03-10T15:45:00", Zone: "Europe/Berlin"}
	meal.Ortswechsel = []Ortswechsel{{
		Ankunft:        Zeitpunkt{Lokal: "2026-03-10T09:00:00", Zone: "Europe/Berlin"},
		Verkehrsmittel: "bahn", LandISO: "DE",
	}}
	meal.Tage = map[string]TagEingabe{"2026-03-10": {Mittag: true}}
	got, err := Berechne(Eingabe{Jahre: cat.Years, Reisen: []Reise{meal}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(got.Reisen[0].Warnungen, "W07") {
		t.Fatalf("W07 %v", got.Reisen[0].Warnungen)
	}
}
