package satz

import (
	"strings"
	"testing"
)

func TestShippedTables(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]int{2024: 223, 2025: 224, 2026: 214}
	for year, n := range want {
		y := cat.Years[year]
		if y == nil || len(y.Rows) != n {
			t.Fatalf("year %d rows %d", year, len(y.Rows))
		}
		for _, row := range y.Rows {
			if len(row.LandISO) != 2 {
				t.Fatalf("missing ISO %+v", row)
			}
		}
	}
	y := cat.Years[2026]
	paris, err := y.Satz("FR", "FR-PARIS")
	if err != nil || paris.Weg != WegExakt || paris.VMA24h != 5800 {
		t.Fatalf("paris %+v %v", paris, err)
	}
	rest, err := y.Satz("FR", "FR-LYON")
	if err != nil || rest.Weg != WegUebriges || rest.VMA24h != 5300 || rest.OrtName != "im Übrigen" {
		t.Fatalf("uebriges %+v %v", rest, err)
	}
	genf, err := y.Satz("CH", "CH-GENF")
	if err != nil || genf.Weg != WegExakt || genf.OrtName != "Genf" {
		t.Fatalf("genf %+v %v", genf, err)
	}
	nyc, err := y.Satz("US", "US-NEW-YORK-CITY")
	if err != nil || nyc.Weg != WegExakt {
		t.Fatalf("nyc %+v %v", nyc, err)
	}
	unknown, err := y.Satz("ZZ", "")
	if err != nil || unknown.Weg != WegLuxemburg || unknown.LandISO != "LU" || unknown.VMA24h != 6300 {
		t.Fatalf("lu %+v %v", unknown, err)
	}
	ph, err := y.Satz("PH", "")
	if err != nil {
		t.Fatal(err)
	}
	fm, err := y.Satz("FM", "")
	if err != nil || fm.Weg != WegErsatz || fm.LandISO != "PH" || fm.VMA24h != ph.VMA24h || fm.ErsatzVon != "FM" {
		t.Fatalf("fm %+v ph %+v %v", fm, ph, err)
	}
	tt, err := y.Satz("TT", "")
	if err != nil {
		t.Fatal(err)
	}
	ag, err := y.Satz("AG", "")
	if err != nil || ag.Weg != WegErsatz || ag.LandISO != "TT" || ag.Uebernachtung != tt.Uebernachtung {
		t.Fatalf("ag %+v tt %+v %v", ag, tt, err)
	}
	de, err := y.Satz("DE", "")
	if err != nil || de.Weg != WegInland || de.VMA24h != 2800 || de.VMA8h != 1400 || de.Uebernachtung != 2000 {
		t.Fatalf("de %+v %v", de, err)
	}
	if y.Inland.SachbezugFruehstueck != 237 || cat.Years[2024].Inland.SachbezugFruehstueck != 217 {
		t.Fatal("sachbezug")
	}
}

func TestImportRejectsBadCSV(t *testing.T) {
	raw := "jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur,land_iso\n" +
		"2027,Testland,,10,7,xx,TL\n"
	rows, errs := ParseImport(strings.NewReader(raw), 2027)
	if rows != nil || len(errs) == 0 {
		t.Fatalf("rows %v errs %v", rows, errs)
	}
	good := "jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur,land_iso\n" +
		"2027,Testland,,10,7,20,TL\n" +
		"2027,Testland,im Übrigen,11,8,21,TL\n"
	rows, errs = ParseImport(strings.NewReader(good), 2027)
	if rows != nil || len(errs) == 0 || errs[0].Code != "doppelt" {
		t.Fatalf("dup rows %v errs %v", rows, errs)
	}
}
