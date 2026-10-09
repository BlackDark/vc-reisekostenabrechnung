package store

import (
	"encoding/json"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
)

// CalcReise maps a stored Reise onto the calculator input.
func CalcReise(b ReiseBundle) berechnung.Reise {
	trip := berechnung.Reise{
		ID: b.Reise.ID, Anlass: b.Reise.Anlass, ArbeitgeberID: b.Reise.ArbeitgeberID,
		Konstellation: "arbeitgebererstattung", Status: b.Reise.Status,
		Beginn: berechnung.Zeitpunkt{Lokal: formatLocal(b.Reise.Beginn, b.Reise.BeginnZone), Zone: b.Reise.BeginnZone},
		Ende:   berechnung.Zeitpunkt{Lokal: formatLocal(b.Reise.Ende, b.Reise.EndeZone), Zone: b.Reise.EndeZone},
		Tage:   map[string]berechnung.TagEingabe{},
	}
	for _, leg := range b.Legs {
		o := berechnung.Ortswechsel{
			Ankunft:        berechnung.Zeitpunkt{Lokal: formatLocal(leg.Ankunft, leg.AnkunftZone), Zone: leg.AnkunftZone},
			Verkehrsmittel: leg.Verkehrsmittel, LandISO: leg.LandIso, Satzort: leg.Satzort, Ort: leg.Ort,
			ZwischenlandungMitUebernachtung: leg.ZwischenlandungMitUebernachtung,
		}
		if leg.Abfahrt != nil && leg.AbfahrtZone != nil {
			o.Abfahrt = &berechnung.Zeitpunkt{Lokal: formatLocal(*leg.Abfahrt, *leg.AbfahrtZone), Zone: *leg.AbfahrtZone}
		}
		if leg.TaetigkeitsstaetteID != nil {
			o.TaetigkeitsstaetteID = *leg.TaetigkeitsstaetteID
			o.Taetigkeit = true
		}
		trip.Ortswechsel = append(trip.Ortswechsel, o)
	}
	for _, day := range b.Tage {
		in := berechnung.TagEingabe{
			Fruehstueck: day.FruehstueckGestellt, Mittag: day.MittagGestellt, Abend: day.AbendGestellt,
			ZuzahlungFruehstueck: day.ZuzahlungFruehstueck, ZuzahlungMittag: day.ZuzahlungMittag, ZuzahlungAbend: day.ZuzahlungAbend,
			Unterkunft: day.Unterkunft, VerpflegungAusgeschlossen: day.VerpflegungAusgeschlossen,
		}
		if day.LandManuell != nil {
			in.LandManuell = *day.LandManuell
		}
		if day.SatzortManuell != nil {
			in.SatzortManuell = *day.SatzortManuell
		}
		if day.LandBegruendung != nil {
			in.Begruendung = *day.LandBegruendung
		}
		trip.Tage[day.Datum] = in
	}
	for _, f := range b.Fahrten {
		trip.Fahrten = append(trip.Fahrten, berechnung.Fahrt{
			ID: f.ID, Datum: f.Datum, Fahrzeugart: f.Fahrzeugart, Km: f.Km, HinUndZurueck: f.HinUndZurueck,
		})
	}
	for _, item := range b.Ausgaben {
		trip.Ausgaben = append(trip.Ausgaben, ausgabeToCalc(item))
	}
	return trip
}

func ausgabeToCalc(item AusgabeBundle) berechnung.Ausgabe {
	row := item.Row
	a := berechnung.Ausgabe{
		ID: row.ID, Kostenart: row.Kostenart, Datum: row.Datum, Leistender: row.Leistender,
		Waehrung: row.Waehrung, Betrag: row.Betrag, BetragEUR: row.BetragEur,
		Rechnungsart: row.Rechnungsart, RechnungAufArbeitgeber: row.RechnungAufArbeitgeber,
		FruehstueckEnthalten: row.FruehstueckEnthalten, TSE: row.TseBeleg,
		BelegAnzahl: len(item.Belege), Eigenbeleg: item.Eigen != nil || row.Rechnungsart == "eigenbeleg",
		PruefeBeleg: true,
	}
	if row.Kurs != nil {
		a.Kurs = *row.Kurs
	}
	if row.KursQuelle != nil {
		a.KursQuelle = *row.KursQuelle
	}
	if row.KursDatum != nil {
		a.KursDatum = *row.KursDatum
	}
	if row.Empfaenger != nil {
		a.Empfaenger = *row.Empfaenger
	}
	if row.Verkehrsmittel != nil {
		a.Verkehrsmittel = *row.Verkehrsmittel
	}
	if row.Mahlzeit != nil {
		a.Mahlzeit = *row.Mahlzeit
	}
	_ = json.Unmarshal([]byte(row.UebernachtungNaechte), &a.Naechte)
	for _, beleg := range item.Belege {
		if belegOffen(beleg.Status) {
			a.BelegOffen = true
		}
	}
	for _, sh := range item.Anteile {
		netto, steuer := sh.Netto, sh.Steuer
		a.Anteile = append(a.Anteile, berechnung.SteuerInput{
			Satz: sh.Satz, Steuerland: sh.Steuerland, Brutto: sh.Brutto, Netto: &netto, Steuer: &steuer,
		})
	}
	if doc := decodeBewirtung(row.Bewirtung); doc != nil {
		n := 0
		for _, p := range doc.Teilnehmer {
			if p.Name != "" {
				n++
			}
		}
		a.Bewirtung = &berechnung.Bewirtung{
			Anlass: doc.Anlass, Teilnehmer: n, Ort: doc.Ort, Bewirtender: doc.Bewirtender, Bestaetigt: doc.BestaetigtAm != "",
		}
	}
	return a
}

func belegOffen(status string) bool {
	switch status {
	case "hochgeladen", "in_aufbereitung", "zur_bestaetigung", "fehlgeschlagen":
		return true
	default:
		return false
	}
}
