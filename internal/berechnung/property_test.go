package berechnung

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
)

func TestOrderIndependentAndOnePauschale(t *testing.T) {
	cat, err := satz.Load()
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 40; n++ {
		in := randomEingabe(rng, cat)
		a, err := Berechne(cloneEingabe(in))
		if err != nil {
			t.Fatal(err)
		}
		perm := append([]Reise{}, in.Reisen...)
		rng.Shuffle(len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
		in.Reisen = perm
		b, err := Berechne(in)
		if err != nil {
			t.Fatal(err)
		}
		if !sameAssignment(a, b) {
			t.Fatalf("order changed assignment\n%v\n%v", amounts(a), amounts(b))
		}
		perDay := map[string]int{}
		for _, re := range a.Reisen {
			if re.Summe < 0 {
				t.Fatal("negative sum")
			}
			for _, day := range re.Tage {
				if day.Ergebnis < 0 {
					t.Fatal("negative day")
				}
				if day.Ergebnis > 0 {
					perDay[day.Datum]++
				}
			}
		}
		for datum, c := range perDay {
			if c > 1 {
				t.Fatalf("%s has %d pauschalen", datum, c)
			}
		}
	}
}

func TestKonstellationAndBlocker(t *testing.T) {
	cat, err := satz.Load()
	if err != nil {
		t.Fatal(err)
	}
	in := Eingabe{Jahre: cat.Years, Reisen: []Reise{{
		ID: "x", Konstellation: "werbungskosten",
		Beginn: Zeitpunkt{Lokal: "2026-03-10T08:00:00", Zone: "Europe/Berlin"},
		Ende:   Zeitpunkt{Lokal: "2026-03-10T18:00:00", Zone: "Europe/Berlin"},
	}}}
	if _, err := Berechne(in); err == nil || err.Error() != "konstellation_nicht_unterstuetzt" {
		t.Fatal(err)
	}
	in.Reisen[0].Konstellation = "arbeitgebererstattung"
	in.Jahre = map[int]*satz.Year{}
	got, err := Berechne(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reisen) != 1 || !containsStr(got.Reisen[0].Blocker, "B05") {
		t.Fatalf("%v", got.Reisen[0].Blocker)
	}
	in.Jahre = cat.Years
	in.Reisen[0].Ende = Zeitpunkt{Lokal: "2026-03-12T18:00:00", Zone: "Europe/Berlin"}
	got, err = Berechne(in)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(got.Reisen[0].Blocker, "B06") {
		t.Fatalf("%v", got.Reisen[0].Blocker)
	}
}

func TestSchiffUndManuell(t *testing.T) {
	cat, err := satz.Load()
	if err != nil {
		t.Fatal(err)
	}
	r := Reise{
		ID: "schiff", Konstellation: "arbeitgebererstattung",
		Beginn: Zeitpunkt{Lokal: "2026-06-01T08:00:00", Zone: "Europe/Berlin"},
		Ende:   Zeitpunkt{Lokal: "2026-06-04T18:00:00", Zone: "Europe/Berlin"},
		Ortswechsel: []Ortswechsel{{
			Abfahrt:        &Zeitpunkt{Lokal: "2026-06-01T18:00:00", Zone: "Europe/Berlin"},
			Ankunft:        Zeitpunkt{Lokal: "2026-06-04T09:00:00", Zone: "Europe/Berlin"},
			Verkehrsmittel: "schiff", LandISO: "FR", Ort: "Le Havre",
		}},
		Tage: map[string]TagEingabe{
			"2026-06-01": {Unterkunft: "verkehrsmittel"},
			"2026-06-02": {Unterkunft: "verkehrsmittel"},
			"2026-06-03": {Unterkunft: "verkehrsmittel", LandManuell: "DE", Begruendung: "Korrektur"},
		},
	}
	got, err := Berechne(Eingabe{Jahre: cat.Years, Reisen: []Reise{r}})
	if err != nil {
		t.Fatal(err)
	}
	days := map[string]TagErgebnis{}
	for _, d := range got.Reisen[0].Tage {
		days[d.Datum] = d
	}
	if days["2026-06-02"].LandISO != "LU" || !containsStr(days["2026-06-02"].RegelIDs, "LAND-SCHIFF") {
		t.Fatalf("%+v", days["2026-06-02"])
	}
	if days["2026-06-03"].LandISO != "DE" || !containsStr(days["2026-06-03"].RegelIDs, "LAND-MANUELL") {
		t.Fatalf("%+v", days["2026-06-03"])
	}
	if days["2026-06-01"].LandISO != "DE" {
		t.Fatalf("embark %s", days["2026-06-01"].LandISO)
	}
}

func randomEingabe(rng *rand.Rand, cat *satz.Catalog) Eingabe {
	n := 2 + rng.Intn(3)
	base := 10 + rng.Intn(10)
	in := Eingabe{Jahre: cat.Years}
	for i := 0; i < n; i++ {
		startH := 6 + rng.Intn(10)
		hours := 2 + rng.Intn(12)
		day := base
		if rng.Intn(2) == 0 {
			day = base
		}
		id := string(rune('A' + i))
		r := Reise{
			ID: id, Anlass: id, Konstellation: "arbeitgebererstattung",
			Beginn: Zeitpunkt{Lokal: clock(2026, 6, day, startH, 0), Zone: "Europe/Berlin"},
			Ende:   Zeitpunkt{Lokal: clock(2026, 6, day, startH, 0), Zone: "Europe/Berlin"},
			Ortswechsel: []Ortswechsel{{
				Ankunft:        Zeitpunkt{Lokal: clock(2026, 6, day, startH, 30), Zone: "Europe/Berlin"},
				Verkehrsmittel: "bahn", LandISO: "DE",
			}},
		}
		endH := startH + hours
		endDay := day
		if endH >= 24 {
			endH -= 24
			endDay++
			r.Tage = map[string]TagEingabe{}
		}
		r.Ende = Zeitpunkt{Lokal: clock(2026, 6, endDay, endH, 0), Zone: "Europe/Berlin"}
		in.Reisen = append(in.Reisen, r)
	}
	return in
}

func clock(y, m, d, h, min int) string {
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:00", y, m, d, h, min)
}

func cloneEingabe(in Eingabe) Eingabe {
	out := in
	out.Reisen = append([]Reise{}, in.Reisen...)
	return out
}

func sameAssignment(a, b Ergebnis) bool {
	return amounts(a) == amounts(b)
}

func amounts(e Ergebnis) string {
	byID := map[string]string{}
	var ids []string
	for _, re := range e.Reisen {
		s := ""
		for _, d := range re.Tage {
			s += d.Datum + "=" + itoa(d.Ergebnis) + ","
		}
		byID[re.ID] = s
		ids = append(ids, re.ID)
	}
	sortStrings(ids)
	out := ""
	for _, id := range ids {
		out += id + ":" + byID[id] + ";"
	}
	return out
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
