package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func TestArbeitgeberStandardAndTenant(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a := mustNutzer(t, s, "ada")
	b := mustNutzer(t, s, "bea")
	first, err := s.CreateArbeitgeber(ctx, a.ID, ArbeitgeberInput{Name: "A GmbH", Anschrift: "Weg 1", Praefix: "RK"}, Actor{Art: "nutzer", NutzerID: &a.ID})
	if err != nil || !first.IstStandard {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := s.CreateArbeitgeber(ctx, a.ID, ArbeitgeberInput{Name: "B GmbH", Anschrift: "Weg 2", Praefix: "RK", IstStandard: true}, Actor{Art: "nutzer", NutzerID: &a.ID})
	if err != nil || !second.IstStandard {
		t.Fatalf("second %+v %v", second, err)
	}
	again, err := s.GetArbeitgeber(ctx, a.ID, first.ID)
	if err != nil || again.IstStandard {
		t.Fatalf("demoted %+v %v", again, err)
	}
	if _, err := s.GetArbeitgeber(ctx, b.ID, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant get %v", err)
	}
	foreign, err := s.ListArbeitgeber(ctx, b.ID, nil, 50)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("tenant list %+v %v", foreign, err)
	}
	archived, err := s.UpdateArbeitgeber(ctx, a.ID, second.ID, second.Version, ArbeitgeberInput{
		Name: second.Name, Anschrift: second.Anschrift, Praefix: "RK", Archiviert: true,
	}, Actor{Art: "nutzer", NutzerID: &a.ID})
	if err != nil || !archived.Archiviert || archived.IstStandard {
		t.Fatalf("archive %+v %v", archived, err)
	}
	restored, err := s.GetArbeitgeber(ctx, a.ID, first.ID)
	if err != nil || !restored.IstStandard || restored.Archiviert {
		t.Fatalf("promoted %+v %v", restored, err)
	}
}

func TestTaetigkeitTenant(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a := mustNutzer(t, s, "ada")
	b := mustNutzer(t, s, "bea")
	row, err := s.CreateTaetigkeit(ctx, a.ID, TaetigkeitInput{
		Bezeichnung: "Kunde", Anschrift: "Rue 1", LandISO: "FR", Satzort: "FR-PARIS",
	}, Actor{Art: "nutzer", NutzerID: &a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTaetigkeit(ctx, b.ID, row.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	list, err := s.ListTaetigkeiten(ctx, b.ID, nil, 20)
	if err != nil || len(list) != 0 {
		t.Fatalf("%+v %v", list, err)
	}
	if err := s.DeleteTaetigkeit(ctx, b.ID, row.ID, Actor{Art: "nutzer", NutzerID: &b.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.DeleteTaetigkeit(ctx, a.ID, row.ID, Actor{Art: "nutzer", NutzerID: &a.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestSatzOverrideAndImport(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	admin := mustNutzer(t, s, "admin")
	actor := Actor{Art: "admin", NutzerID: &admin.ID}
	hit, err := s.Satz(ctx, 2026, "FR", "FR-PARIS")
	if err != nil || hit.VMA24h != 5800 || hit.Weg != satz.WegExakt {
		t.Fatalf("%+v %v", hit, err)
	}
	year, err := s.GetSatztabelle(ctx, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OverrideSatztabelle(ctx, 2026, year.Version, "FR", "FR-PARIS", "vma_24h", 6100, "Dienstreise Paris", actor); err != nil {
		t.Fatal(err)
	}
	hit, err = s.Satz(ctx, 2026, "FR", "FR-PARIS")
	if err != nil || hit.VMA24h != 6100 {
		t.Fatalf("override %+v %v", hit, err)
	}
	rows, err := s.ListOverrides(ctx, 2026)
	if err != nil || len(rows) != 1 || rows[0].Grund != "Dienstreise Paris" || rows[0].AlterWert != "5800" {
		t.Fatalf("%+v %v", rows, err)
	}
	events, err := s.ListProtokoll(ctx, "satztabelle", "2026", 20)
	if err != nil || len(events) == 0 || events[0].Grund == nil || *events[0].Grund != "Dienstreise Paris" {
		t.Fatalf("protokoll %+v %v", events, err)
	}
	bad, errs := satz.ParseImport(strings.NewReader("jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur,land_iso\n2027,X,,1,1,nope,XX\n"), 2027)
	if bad != nil || len(errs) == 0 {
		t.Fatal("bad csv should not parse")
	}
	if _, err := s.GetSatztabelle(ctx, 2027); !errors.Is(err, ErrNotFound) {
		t.Fatal("bad csv must not create a year")
	}
	good, errs := satz.ParseImport(strings.NewReader("jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur,land_iso\n2027,Testland,,12,8,40,TL\n2027,Testland,Paris,13,9,41,TL\n"), 2027)
	if len(errs) != 0 || len(good) != 2 {
		t.Fatalf("good %+v %v", good, errs)
	}
	imported, err := s.ImportSatztabelle(ctx, 2027, good, actor)
	if err != nil || imported.Status != "entwurf" {
		t.Fatalf("import %+v %v", imported, err)
	}
	active, err := s.ActivateSatztabelle(ctx, 2027, actor)
	if err != nil || active.Status != "aktiv" {
		t.Fatalf("activate %+v %v", active, err)
	}
	got, err := s.Satz(ctx, 2027, "TL", "TL-PARIS")
	if err != nil || got.VMA24h != 1300 || got.Weg != satz.WegExakt {
		t.Fatalf("2027 %+v %v", got, err)
	}
}

func mustNutzer(t *testing.T, s *Store, name string) sqlitedb.Nutzer {
	t.Helper()
	n, err := s.CreateNutzer(context.Background(), NewNutzer{
		Anzeigename: name, Benutzername: name, Sprache: "de", Aktiv: true, IstAdminLokal: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	return n
}
