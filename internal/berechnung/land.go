package berechnung

import (
	"strings"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
)

type ort struct {
	Land    string
	Satzort string
	Regel   string
}

func (o ort) inland() bool {
	return o.Land == "" || o.Land == "DE"
}

func (leg Ortswechsel) activity() bool {
	return leg.Taetigkeit || leg.TaetigkeitsstaetteID != ""
}

// massgeblichesLand applies SPEC 4.6 in priority order.
func massgeblichesLand(r Reise, day civil, art string, year *satz.Year) ort {
	in := tagInput(r, day.String())
	if strings.TrimSpace(in.LandManuell) != "" {
		return ort{Land: strings.ToUpper(in.LandManuell), Satzort: in.SatzortManuell, Regel: "LAND-MANUELL"}
	}
	if o, ok := flightIntermediate(r, day, year); ok {
		return o
	}
	if o, ok := shipIntermediate(r, day, year); ok {
		return o
	}
	base := ortRegel4(r, day)
	if base.inland() && needsRueck(r, day, art) {
		if foreign, ok := lastForeignActivity(r, day); ok {
			foreign.Regel = "LAND-RUECK"
			return foreign
		}
	}
	if base.Land == "" {
		base = ort{Land: "DE", Regel: "LAND-GRUND"}
	}
	if base.Regel == "" {
		base.Regel = "LAND-GRUND"
	}
	return base
}

func needsRueck(r Reise, day civil, art string) bool {
	if art == "abreisetag" || art == "eintaegig" {
		return hasForeignActivity(r, day)
	}
	return foreignActivityOn(r, day)
}

func hasForeignActivity(r Reise, day civil) bool {
	_, ok := lastForeignActivity(r, day)
	return ok
}

func foreignActivityOn(r Reise, day civil) bool {
	for _, leg := range r.Ortswechsel {
		if !leg.activity() || leg.LandISO == "" || leg.LandISO == "DE" {
			continue
		}
		arrived, err := leg.Ankunft.civil()
		if err != nil {
			continue
		}
		if arrived.equal(day) {
			return true
		}
	}
	return false
}

func lastForeignActivity(r Reise, day civil) (ort, bool) {
	var found ort
	ok := false
	for _, leg := range r.Ortswechsel {
		if !leg.activity() || leg.LandISO == "" || leg.LandISO == "DE" {
			continue
		}
		if !arrivedOnOrBefore(leg, day) {
			continue
		}
		found = ort{Land: leg.LandISO, Satzort: leg.Satzort}
		ok = true
	}
	return found, ok
}

func flightIntermediate(r Reise, day civil, year *satz.Year) (ort, bool) {
	if flaggedStop(r, day) {
		return ort{}, false
	}
	land := "AT"
	if year != nil && year.Inland.Flug != "" {
		land = year.Inland.Flug
	}
	for _, leg := range r.Ortswechsel {
		if leg.Verkehrsmittel != "flug" || leg.Abfahrt == nil {
			continue
		}
		a, errA := leg.Abfahrt.civil()
		l, errL := leg.Ankunft.civil()
		if errA != nil || errL != nil {
			continue
		}
		if a.daysUntil(l) >= 2 && a.before(day) && day.before(l) {
			return ort{Land: land, Regel: "LAND-FLUG-AT"}, true
		}
	}
	return ort{}, false
}

func flaggedStop(r Reise, day civil) bool {
	for _, leg := range r.Ortswechsel {
		if leg.Verkehrsmittel != "flug" || !leg.ZwischenlandungMitUebernachtung {
			continue
		}
		arrived, err := leg.Ankunft.civil()
		if err == nil && arrived.equal(day) {
			return true
		}
	}
	return false
}

func shipIntermediate(r Reise, day civil, year *satz.Year) (ort, bool) {
	land := "LU"
	if year != nil && year.Inland.Schiff != "" {
		land = year.Inland.Schiff
	}
	for _, leg := range r.Ortswechsel {
		if leg.Verkehrsmittel != "schiff" || leg.Abfahrt == nil {
			continue
		}
		a, errA := leg.Abfahrt.civil()
		l, errL := leg.Ankunft.civil()
		if errA != nil || errL != nil {
			continue
		}
		if a.before(day) && day.before(l) {
			return ort{Land: land, Regel: "LAND-SCHIFF"}, true
		}
	}
	return ort{}, false
}

// ortRegel4 is the last arrival strictly before the end of day (SPEC 4.6 rule 4).
func ortRegel4(r Reise, day civil) ort {
	var found ort
	seen := false
	for _, leg := range r.Ortswechsel {
		if !arrivedOnOrBefore(leg, day) {
			continue
		}
		land := leg.LandISO
		if land == "" {
			land = "DE"
		}
		found = ort{Land: land, Satzort: leg.Satzort, Regel: "LAND-GRUND"}
		seen = true
	}
	if !seen {
		return ort{Land: "DE", Regel: "LAND-GRUND"}
	}
	return found
}

func arrivedOnOrBefore(leg Ortswechsel, day civil) bool {
	arrived, err := leg.Ankunft.civil()
	if err != nil {
		return false
	}
	return !arrived.after(day)
}

func lookup(year *satz.Year, o ort) (satz.Hit, error) {
	if year == nil {
		return satz.Hit{}, errNoYear
	}
	return year.Satz(o.Land, o.Satzort)
}
