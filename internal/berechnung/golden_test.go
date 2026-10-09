package berechnung

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
)

func TestGolden(t *testing.T) {
	cat, err := satz.Load()
	if err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob("testdata/golden/G??-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 20 {
		t.Fatalf("golden files %d", len(matches))
	}
	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var doc goldenDoc
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Auszahlung) > 0 {
				checkAuszahlung(t, doc)
				return
			}
			if len(doc.Dreimonat) > 0 {
				checkFrist(t, doc)
				return
			}
			in := Eingabe{Jahre: cat.Years}
			for _, k := range doc.Kurse {
				in.Kurse = append(in.Kurse, Kurs(k))
			}
			for _, src := range doc.Reisen {
				in.Reisen = append(in.Reisen, src.reise())
			}
			got, err := Berechne(in)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Reisen) != len(doc.Expect.Reisen) {
				t.Fatalf("reisen %d", len(got.Reisen))
			}
			for i, exp := range doc.Expect.Reisen {
				re := got.Reisen[i]
				if re.ID != exp.ID {
					t.Fatalf("id %s", re.ID)
				}
				if exp.Verpflegung != nil && re.Verpflegung != *exp.Verpflegung {
					t.Fatalf("verpflegung %d want %d", re.Verpflegung, *exp.Verpflegung)
				}
				if exp.Uebernachtung != nil && re.Uebernachtung != *exp.Uebernachtung {
					t.Fatalf("uebernachtung %d want %d", re.Uebernachtung, *exp.Uebernachtung)
				}
				if exp.Fahrtkosten != nil && re.Fahrtkosten != *exp.Fahrtkosten {
					t.Fatalf("fahrtkosten %d want %d", re.Fahrtkosten, *exp.Fahrtkosten)
				}
				if exp.Summe != nil && re.Summe != *exp.Summe {
					t.Fatalf("summe %d want %d", re.Summe, *exp.Summe)
				}
				if exp.Neben != nil && re.Reisenebenkosten != *exp.Neben {
					t.Fatalf("neben %d", re.Reisenebenkosten)
				}
				if exp.Bewirtung != nil && re.Bewirtung != *exp.Bewirtung {
					t.Fatalf("bewirtung %d", re.Bewirtung)
				}
				if exp.BewirtungB != nil && re.BewirtungB != *exp.BewirtungB {
					t.Fatalf("bewirtung B %d", re.BewirtungB)
				}
				if exp.Abziehbar != nil && re.BewirtungAbziehbar != *exp.Abziehbar {
					t.Fatalf("abziehbar %d", re.BewirtungAbziehbar)
				}
				if exp.Nicht != nil && re.BewirtungNichtAbziehbar != *exp.Nicht {
					t.Fatalf("nicht %d", re.BewirtungNichtAbziehbar)
				}
				if exp.Vorsteuer != nil && re.Vorsteuer != *exp.Vorsteuer {
					t.Fatalf("vorsteuer %d", re.Vorsteuer)
				}
				for _, code := range exp.Warnungen {
					if !containsStr(re.Warnungen, code) {
						t.Fatalf("missing %s in %v", code, re.Warnungen)
					}
				}
				if exp.Anteile != nil {
					var gotShares []SteuerErgebnis
					for _, a := range re.Ausgaben {
						gotShares = append(gotShares, a.Anteile...)
					}
					if len(gotShares) != len(exp.Anteile) {
						t.Fatalf("anteile %d", len(gotShares))
					}
					for i, sh := range exp.Anteile {
						if gotShares[i].Netto != sh.Netto || gotShares[i].Steuer != sh.Steuer {
							t.Fatalf("anteil %d %+v", i, gotShares[i])
						}
					}
				}
				for _, aexp := range exp.Ausgaben {
					var found *AusgabeErgebnis
					for i := range re.Ausgaben {
						if re.Ausgaben[i].ID == aexp.ID {
							found = &re.Ausgaben[i]
						}
					}
					if found == nil {
						t.Fatalf("ausgabe %s", aexp.ID)
					}
					if aexp.BetragEUR != nil && found.BetragEUR != *aexp.BetragEUR {
						t.Fatalf("betrag_eur %d", found.BetragEUR)
					}
					if aexp.Kurs != "" && found.Kurs != aexp.Kurs {
						t.Fatalf("kurs %s", found.Kurs)
					}
					if aexp.KursDatum != "" && found.KursDatum != aexp.KursDatum {
						t.Fatalf("kursdatum %s", found.KursDatum)
					}
				}
				dates, err := Dates(in.Reisen[i])
				if err != nil {
					t.Fatal(err)
				}
				if len(re.Tage) != len(dates) {
					t.Fatalf("I4 %d days want %d", len(re.Tage), len(dates))
				}
				for d, datum := range dates {
					if re.Tage[d].Datum != datum {
						t.Fatalf("gap %s", re.Tage[d].Datum)
					}
				}
				for _, te := range exp.Tage {
					day := findDay(re, te.Datum)
					if day == nil {
						t.Fatalf("tag %s missing in %d days", te.Datum, len(re.Tage))
					}
					if te.Tagesart != "" && day.Tagesart != te.Tagesart {
						t.Fatalf("%s art %s", te.Datum, day.Tagesart)
					}
					if te.Land != "" && day.LandISO != te.Land {
						t.Fatalf("%s land %s", te.Datum, day.LandISO)
					}
					if te.Satzort != nil && day.Satzort != *te.Satzort {
						t.Fatalf("%s satzort %s", te.Datum, day.Satzort)
					}
					if te.Ergebnis != nil && day.Ergebnis != *te.Ergebnis {
						t.Fatalf("%s ergebnis %d want %d", te.Datum, day.Ergebnis, *te.Ergebnis)
					}
					if te.Pauschale != nil && day.Pauschale != *te.Pauschale {
						t.Fatalf("%s pauschale %d", te.Datum, day.Pauschale)
					}
					if te.Kuerzung != nil {
						var sum int64
						for _, k := range day.Kuerzungen {
							sum += k.Betrag
						}
						if sum != *te.Kuerzung {
							t.Fatalf("%s kuerzung %d want %d (%v)", te.Datum, sum, *te.Kuerzung, day.Kuerzungen)
						}
					}
					if te.Abwesenheit != nil && day.AbwesenheitMin != *te.Abwesenheit {
						t.Fatalf("%s min %d", te.Datum, day.AbwesenheitMin)
					}
					for _, code := range te.Hinweise {
						if !hasHinweis(*day, code) {
							t.Fatalf("%s missing %s in %v", te.Datum, code, day.Hinweise)
						}
					}
					if te.Sachbezug != nil {
						ok := false
						for _, h := range day.Hinweise {
							if h.Code == "H-SACHBEZUG" && h.Betrag == *te.Sachbezug {
								ok = true
							}
						}
						if !ok {
							t.Fatalf("sachbezug %v", day.Hinweise)
						}
					}
					for _, regel := range te.Regeln {
						if !containsStr(day.RegelIDs, regel) {
							t.Fatalf("%s missing %s in %v", te.Datum, regel, day.RegelIDs)
						}
					}
					if day.Ergebnis < 0 || day.Uebernachtung < 0 {
						t.Fatalf("negative %+v", day)
					}
				}
			}
			for _, v := range doc.Expect.Vorschlaege {
				found := false
				for _, r := range in.Reisen {
					for _, s := range SuggestMeals(r) {
						if s.Datum == v.Datum && s.Mahlzeit == v.Mahlzeit {
							found = true
						}
					}
				}
				if !found {
					t.Fatalf("vorschlag %s %s", v.Datum, v.Mahlzeit)
				}
			}
		})
	}
}

func checkAuszahlung(t *testing.T, doc goldenDoc) {
	t.Helper()
	for _, row := range doc.Auszahlung {
		_, got := Auszahlung(row.Erstattung, row.Vorschuesse)
		if got != row.Auszahlung {
			t.Fatalf("auszahlung %d want %d", got, row.Auszahlung)
		}
	}
}

func checkFrist(t *testing.T, doc goldenDoc) {
	t.Helper()
	for _, block := range doc.Dreimonat {
		var visits []Visit
		if len(block.Tage) > 0 {
			for _, d := range block.Tage {
				visits = append(visits, Visit{Staette: block.Staette, Datum: d})
			}
		} else {
			visits = weekdayVisits(block.Staette, block.Von, block.Bis, block.Wochentage)
		}
		warns := Dreimonatsfrist(visits)
		if block.KeineW03 {
			if len(warns) != 0 {
				t.Fatalf("unexpected W03 %v", warns)
			}
			if block.FortsetzungAb != "" {
				bis := block.FortsetzungBis
				if bis == "" {
					bis = "2026-08-31"
				}
				more := weekdayVisits(block.Staette, block.FortsetzungAb, bis, []int{1, 2, 3})
				warns = Dreimonatsfrist(append(visits, more...))
				if block.FortsetzungW03 != "" && !hasWarnOn(warns, block.FortsetzungW03) {
					t.Fatalf("missing W03 %s (%d warns)", block.FortsetzungW03, len(warns))
				}
				for _, w := range warns {
					if w.Datum < block.FortsetzungAb {
						t.Fatalf("old series warned %s", w.Datum)
					}
				}
			}
			continue
		}
		if block.Fristende != "" {
			for _, w := range warns {
				if w.Fristende != block.Fristende {
					t.Fatalf("fristende %s", w.Fristende)
				}
				if w.Code != "W03" {
					t.Fatalf("code %s", w.Code)
				}
			}
		}
		if block.W03Ab != "" && !hasWarnOn(warns, block.W03Ab) {
			t.Fatalf("missing %s", block.W03Ab)
		}
		for _, d := range block.W03Nicht {
			if hasWarnOn(warns, d) {
				t.Fatalf("unexpected %s", d)
			}
		}
	}
}

func weekdayVisits(name, from, to string, weekdays []int) []Visit {
	start, _ := parseCivil(from)
	end, _ := parseCivil(to)
	want := map[int]bool{}
	for _, d := range weekdays {
		want[d] = true
	}
	var out []Visit
	for d := start; !d.after(end); d = d.next() {
		wd := isoWeekday(d)
		if want[wd] {
			out = append(out, Visit{Staette: name, Datum: d.String()})
		}
	}
	return out
}

func isoWeekday(c civil) int {
	mon := mondayOf(c)
	return mon.daysUntil(c) + 1
}

func hasWarnOn(w []FristWarnung, datum string) bool {
	for _, x := range w {
		if x.Datum == datum {
			return true
		}
	}
	return false
}

func findDay(re ReiseErgebnis, datum string) *TagErgebnis {
	for i := range re.Tage {
		if re.Tage[i].Datum == datum {
			return &re.Tage[i]
		}
	}
	return nil
}

func hasHinweis(day TagErgebnis, code string) bool {
	for _, h := range day.Hinweise {
		if h.Code == code {
			return true
		}
	}
	return false
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

type goldenDoc struct {
	ID         string        `yaml:"id"`
	Kurse      []kursYAML    `yaml:"kurse"`
	Reisen     []reiseYAML   `yaml:"reisen"`
	Expect     expectYAML    `yaml:"expect"`
	Auszahlung []auszahlYAML `yaml:"auszahlung"`
	Dreimonat  []fristYAML   `yaml:"dreimonat"`
}

type kursYAML struct {
	Waehrung string `yaml:"waehrung"`
	Datum    string `yaml:"datum"`
	Kurs     string `yaml:"kurs"`
}

type auszahlYAML struct {
	Erstattung  int64   `yaml:"erstattung"`
	Vorschuesse []int64 `yaml:"vorschuesse"`
	Auszahlung  int64   `yaml:"auszahlung"`
}

type fristYAML struct {
	Staette        string   `yaml:"staette"`
	Von            string   `yaml:"von"`
	Bis            string   `yaml:"bis"`
	Wochentage     []int    `yaml:"wochentage"`
	Tage           []string `yaml:"tage"`
	Fristende      string   `yaml:"fristende"`
	W03Ab          string   `yaml:"w03_ab"`
	W03Nicht       []string `yaml:"w03_nicht"`
	KeineW03       bool     `yaml:"keine_w03"`
	FortsetzungAb  string   `yaml:"fortsetzung_ab"`
	FortsetzungBis string   `yaml:"fortsetzung_bis"`
	FortsetzungW03 string   `yaml:"fortsetzung_w03_ab"`
}

type expectYAML struct {
	Reisen      []expectReise `yaml:"reisen"`
	Vorschlaege []struct {
		Datum    string `yaml:"datum"`
		Mahlzeit string `yaml:"mahlzeit"`
	} `yaml:"vorschlaege"`
}

type expectReise struct {
	ID            string      `yaml:"id"`
	Verpflegung   *int64      `yaml:"verpflegung"`
	Uebernachtung *int64      `yaml:"uebernachtung"`
	Fahrtkosten   *int64      `yaml:"fahrtkosten"`
	Summe         *int64      `yaml:"summe"`
	Neben         *int64      `yaml:"reisenebenkosten"`
	Bewirtung     *int64      `yaml:"bewirtung"`
	BewirtungB    *int64      `yaml:"bewirtung_b"`
	Abziehbar     *int64      `yaml:"abziehbar"`
	Nicht         *int64      `yaml:"nicht_abziehbar"`
	Vorsteuer     *int64      `yaml:"vorsteuer"`
	Warnungen     []string    `yaml:"warnungen"`
	Tage          []expectTag `yaml:"tage"`
	Anteile       []struct {
		Netto  int64 `yaml:"netto"`
		Steuer int64 `yaml:"steuer"`
	} `yaml:"anteile"`
	Ausgaben []struct {
		ID        string `yaml:"id"`
		BetragEUR *int64 `yaml:"betrag_eur"`
		Kurs      string `yaml:"kurs"`
		KursDatum string `yaml:"kurs_datum"`
	} `yaml:"ausgaben"`
}

type expectTag struct {
	Datum       string   `yaml:"datum"`
	Tagesart    string   `yaml:"tagesart"`
	Land        string   `yaml:"land"`
	Satzort     *string  `yaml:"satzort"`
	Ergebnis    *int64   `yaml:"ergebnis"`
	Pauschale   *int64   `yaml:"pauschale"`
	Kuerzung    *int64   `yaml:"kuerzung"`
	Abwesenheit *int     `yaml:"abwesenheit"`
	Hinweise    []string `yaml:"hinweise"`
	Sachbezug   *int64   `yaml:"sachbezug"`
	Regeln      []string `yaml:"regeln"`
}

type reiseYAML struct {
	ID          string             `yaml:"id"`
	Anlass      string             `yaml:"anlass"`
	Beginn      string             `yaml:"beginn"`
	BeginnZone  string             `yaml:"beginn_zone"`
	Ende        string             `yaml:"ende"`
	EndeZone    string             `yaml:"ende_zone"`
	Ortswechsel []legYAML          `yaml:"ortswechsel"`
	Tage        map[string]tagYAML `yaml:"tage"`
	Ausgaben    []ausgabeYAML      `yaml:"ausgaben"`
	Fahrten     []fahrtYAML        `yaml:"fahrten"`
}

func (src reiseYAML) reise() Reise {
	r := Reise{
		ID: src.ID, Anlass: src.Anlass, Konstellation: "arbeitgebererstattung",
		Beginn: Zeitpunkt{Lokal: src.Beginn, Zone: src.BeginnZone},
		Ende:   Zeitpunkt{Lokal: src.Ende, Zone: src.EndeZone},
		Tage:   map[string]TagEingabe{},
	}
	for _, leg := range src.Ortswechsel {
		o := Ortswechsel{
			Ankunft:        Zeitpunkt{Lokal: leg.Ankunft, Zone: leg.AnkunftZone},
			Verkehrsmittel: leg.Verkehrsmittel, LandISO: leg.LandISO, Satzort: leg.Satzort,
			Ort: leg.Ort, Taetigkeit: leg.Taetigkeit,
		}
		if leg.Abfahrt != "" {
			o.Abfahrt = &Zeitpunkt{Lokal: leg.Abfahrt, Zone: leg.AbfahrtZone}
		}
		r.Ortswechsel = append(r.Ortswechsel, o)
	}
	for datum, tag := range src.Tage {
		r.Tage[datum] = TagEingabe{
			Fruehstueck: tag.Fruehstueck, Mittag: tag.Mittag, Abend: tag.Abend,
			ZuzahlungMittag: tag.ZuzahlungMittag, Unterkunft: tag.Unterkunft,
		}
	}
	for _, a := range src.Ausgaben {
		out := Ausgabe{
			ID: a.ID, Kostenart: a.Kostenart, Datum: a.Datum, Waehrung: a.Waehrung,
			Betrag: a.Betrag, KursQuelle: a.KursQuelle, BetragEUR: a.BetragEUR,
			Naechte: a.Naechte, FruehstueckEnthalten: a.Fruehstueck, Mahlzeit: a.Mahlzeit,
		}
		for _, sh := range a.Anteile {
			out.Anteile = append(out.Anteile, SteuerInput{Satz: sh.Satz, Steuerland: sh.Steuerland, Brutto: sh.Brutto})
		}
		r.Ausgaben = append(r.Ausgaben, out)
	}
	for _, f := range src.Fahrten {
		r.Fahrten = append(r.Fahrten, Fahrt{
			Datum: f.Datum, Fahrzeugart: f.Fahrzeugart, Km: f.Km, HinUndZurueck: f.HinUndZurueck,
		})
	}
	return r
}

type legYAML struct {
	Abfahrt        string `yaml:"abfahrt"`
	AbfahrtZone    string `yaml:"abfahrt_zone"`
	Ankunft        string `yaml:"ankunft"`
	AnkunftZone    string `yaml:"ankunft_zone"`
	Verkehrsmittel string `yaml:"verkehrsmittel"`
	LandISO        string `yaml:"land_iso"`
	Satzort        string `yaml:"satzort"`
	Ort            string `yaml:"ort"`
	Taetigkeit     bool   `yaml:"taetigkeit"`
}

type tagYAML struct {
	Unterkunft      string `yaml:"unterkunft"`
	Fruehstueck     bool   `yaml:"fruehstueck"`
	Mittag          bool   `yaml:"mittag"`
	Abend           bool   `yaml:"abend"`
	ZuzahlungMittag int64  `yaml:"zuzahlung_mittag"`
}

type ausgabeYAML struct {
	ID          string   `yaml:"id"`
	Kostenart   string   `yaml:"kostenart"`
	Datum       string   `yaml:"datum"`
	Waehrung    string   `yaml:"waehrung"`
	Betrag      int64    `yaml:"betrag"`
	KursQuelle  string   `yaml:"kurs_quelle"`
	BetragEUR   int64    `yaml:"betrag_eur"`
	Naechte     []string `yaml:"naechte"`
	Fruehstueck bool     `yaml:"fruehstueck"`
	Mahlzeit    string   `yaml:"mahlzeit"`
	Anteile     []struct {
		Satz       int64  `yaml:"satz"`
		Steuerland string `yaml:"steuerland"`
		Brutto     int64  `yaml:"brutto"`
	} `yaml:"anteile"`
}

type fahrtYAML struct {
	Datum         string `yaml:"datum"`
	Fahrzeugart   string `yaml:"fahrzeugart"`
	Km            int64  `yaml:"km"`
	HinUndZurueck bool   `yaml:"hin_und_zurueck"`
}
