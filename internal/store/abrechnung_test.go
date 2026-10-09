package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func TestAbrechnungPeriodAndLock(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	ada := mustNutzer(t, s, "ada")
	bob := mustNutzer(t, s, "bob")
	actor := Actor{Art: "nutzer", NutzerID: &ada.ID}
	ag, err := s.CreateArbeitgeber(ctx, ada.ID, ArbeitgeberInput{Name: "ACME", Anschrift: "Weg 1"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateArbeitgeber(ctx, ada.ID, ArbeitgeberInput{Name: "Andere", Anschrift: "Weg 2"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	inside := trip(t, s, ada.ID, actor, ag.ID, "2026-10-01T08:00:00", "2026-10-02T18:00:00")
	outside := trip(t, s, ada.ID, actor, ag.ID, "2026-11-01T08:00:00", "2026-11-02T18:00:00")
	foreign := trip(t, s, ada.ID, actor, other.ID, "2026-10-03T08:00:00", "2026-10-04T18:00:00")

	if _, err := s.CreateAbrechnung(ctx, ada.ID, AbrechnungInput{
		ArbeitgeberID: ag.ID, ZeitraumArt: "woche", Von: "2026-10-01", Bis: "2026-10-07",
	}, actor); err == nil || codeOf(err) != "zeitraum" {
		t.Fatalf("week %v", err)
	}
	bundle, err := s.CreateAbrechnung(ctx, ada.ID, AbrechnungInput{
		ArbeitgeberID: ag.ID, ZeitraumArt: "monat", Von: "2026-10-01", Bis: "2026-10-31", Sprache: "de",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Row.Titel != "Reisekosten Oktober 2026" {
		t.Fatalf("titel %s", bundle.Row.Titel)
	}
	if len(bundle.ReiseIDs) != 1 || bundle.ReiseIDs[0] != inside.Reise.ID {
		t.Fatalf("selection %+v", bundle.ReiseIDs)
	}
	if contains(bundle.VorschlaegeReise, outside.Reise.ID) || contains(bundle.VorschlaegeReise, foreign.Reise.ID) {
		t.Fatalf("suggestion %+v", bundle.VorschlaegeReise)
	}
	if _, err := s.GetAbrechnungBundle(ctx, bob.ID, bundle.Row.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	locked, err := s.GetReiseBundle(ctx, ada.ID, inside.Reise.ID)
	if err != nil || locked.Reise.Status != "in_entwurf" {
		t.Fatalf("status %+v %v", locked.Reise.Status, err)
	}

	adv, err := s.CreateVorschuss(ctx, ada.ID, VorschussInput{
		ArbeitgeberID: ag.ID, Datum: "2026-10-05", Betrag: 30000,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	big, err := s.CreateVorschuss(ctx, ada.ID, VorschussInput{
		ArbeitgeberID: ag.ID, Datum: "2026-10-06", Betrag: 50000,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err = s.SetAbrechnungVorschuesse(ctx, ada.ID, bundle.Row.ID, bundle.Row.Version, []string{adv.ID}, actor)
	if err != nil {
		t.Fatal(err)
	}
	sum, pay := berechnung.Auszahlung(41240, []int64{30000})
	if sum != 30000 || pay != 11240 {
		t.Fatalf("g19 %d %d", sum, pay)
	}
	bundle, err = s.SetAbrechnungVorschuesse(ctx, ada.ID, bundle.Row.ID, bundle.Row.Version, []string{big.ID}, actor)
	if err != nil {
		t.Fatal(err)
	}
	_, pay = berechnung.Auszahlung(41240, []int64{50000})
	if pay != -8760 {
		t.Fatalf("g19 repayment %d", pay)
	}
	bundle, err = s.SetAbrechnungVorschuesse(ctx, ada.ID, bundle.Row.ID, bundle.Row.Version, []string{adv.ID}, actor)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Einreichen(ctx, ada.ID, bundle.Row.ID, EinreichenInput{
		Version: bundle.Row.Version, Blocker: []string{"B01"}, Snapshot: `{}`,
	}, actor); codeOf(err) != "blocker" {
		t.Fatal(err)
	}
	if _, err := s.Einreichen(ctx, ada.ID, bundle.Row.ID, EinreichenInput{
		Version: bundle.Row.Version, Warnungen: []string{WarnKey("W01", "x")}, Snapshot: `{"schema_version":"1"}`,
	}, actor); codeOf(err) != "warnung_offen" {
		t.Fatal(err)
	}
	running, err := s.Einreichen(ctx, ada.ID, bundle.Row.ID, EinreichenInput{
		Version: bundle.Row.Version, Snapshot: `{"schema_version":"1"}`,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if running.Row.Status != "entwurf" || !running.Row.EinreichungLaeuft || running.Row.Abrechnungsnummer == nil {
		t.Fatalf("submit %+v", running.Row)
	}
	if *running.Row.Abrechnungsnummer != "RK-2026-001" {
		t.Fatalf("nummer %s", *running.Row.Abrechnungsnummer)
	}
	if _, err := s.UpdateReise(ctx, ada.ID, inside.Reise.ID, locked.Reise.Version, ReiseInput{
		ArbeitgeberID: ag.ID, Anlass: "anders",
		BeginnLokal: "2026-10-01T08:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-10-02T18:00:00", EndeZone: "Europe/Berlin",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: "2026-10-01T10:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	}, actor); codeOf(err) != "reise_gesperrt" {
		t.Fatal(err)
	}

	exports, err := s.ListExporte(ctx, ada.ID, running.Row.ID)
	if err != nil || len(exports) != 1 || exports[0].Status != "in_erstellung" {
		t.Fatalf("exports %+v %v", exports, err)
	}
	job := sqlitedb.Job{ID: "job-1", Art: "export", Payload: &exports[0].ID}
	if err := s.FailExportJob(ctx, job, errors.New("typst failed")); err != nil {
		t.Fatal(err)
	}
	failed, err := s.GetAbrechnungBundle(ctx, ada.ID, running.Row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Row.Status != "entwurf" || failed.Row.EinreichungLaeuft || failed.Row.EinreichungFehler == nil {
		t.Fatalf("rollback %+v", failed.Row)
	}
	openAgain, err := s.GetReiseBundle(ctx, ada.ID, inside.Reise.ID)
	if err != nil || openAgain.Reise.Status != "in_entwurf" {
		t.Fatalf("unlocked %s %v", openAgain.Reise.Status, err)
	}

	again, err := s.Einreichen(ctx, ada.ID, failed.Row.ID, EinreichenInput{
		Version: failed.Row.Version, Snapshot: `{"schema_version":"1"}`,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	exports, err = s.ListExporte(ctx, ada.ID, again.Row.ID)
	if err != nil || len(exports) != 2 {
		t.Fatalf("versions %+v %v", exports, err)
	}
	var fresh sqlitedb.Export
	for _, exp := range exports {
		if exp.Status == "in_erstellung" {
			fresh = exp
		}
	}
	if err := s.CompleteExport(ctx, "job-2", fresh.ID, "pdf", "aa", "zip", "bb", ""); err != nil {
		t.Fatal(err)
	}
	done, err := s.GetAbrechnungBundle(ctx, ada.ID, again.Row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Row.Status != "eingereicht" || done.Row.AktuelleExportVersion != fresh.Version {
		t.Fatalf("done %+v", done.Row)
	}
	if _, err := s.Entsperren(ctx, ada.ID, done.Row.ID, done.Row.Version, "kurz", actor); codeOf(err) != "grund" {
		t.Fatal(err)
	}
	paid, err := s.MarkBezahlt(ctx, ada.ID, done.Row.ID, done.Row.Version, time.Now().In(berlin()).Format("2006-01-02"), "überwiesen", actor)
	if err != nil || paid.Row.Status != "bezahlt" {
		t.Fatalf("paid %+v %v", paid.Row.Status, err)
	}
	if _, err := s.Entsperren(ctx, ada.ID, paid.Row.ID, paid.Row.Version, "Korrektur der Belege", actor); codeOf(err) != "uebergang" {
		t.Fatal(err)
	}
	back, err := s.BezahltZuruecknehmen(ctx, ada.ID, paid.Row.ID, paid.Row.Version, "Zahlung war falsch zugeordnet", actor)
	if err != nil || back.Row.Status != "eingereicht" {
		t.Fatalf("withdraw %+v %v", back.Row.Status, err)
	}
	draft, err := s.Entsperren(ctx, ada.ID, back.Row.ID, back.Row.Version, "Beleg nachgereicht", actor)
	if err != nil || draft.Row.Status != "entwurf" {
		t.Fatalf("unlock %+v %v", draft.Row.Status, err)
	}
	second, err := s.Einreichen(ctx, ada.ID, draft.Row.ID, EinreichenInput{
		Version: draft.Row.Version, Snapshot: `{"schema_version":"1","v":2}`,
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	exports, err = s.ListExporte(ctx, ada.ID, second.Row.ID)
	if err != nil {
		t.Fatal(err)
	}
	var next sqlitedb.Export
	for _, exp := range exports {
		if exp.Status == "in_erstellung" {
			next = exp
		}
	}
	if next.Anlass != "einreichung_nach_entsperrung" || next.Version != fresh.Version+1 {
		t.Fatalf("v2 %+v", next)
	}
	if err := s.CompleteExport(ctx, "job-3", next.ID, "pdf2", "cc", "zip2", "dd", `{"schema_version":"1","v":2}`); err != nil {
		t.Fatal(err)
	}
	final, err := s.ListExporte(ctx, ada.ID, second.Row.ID)
	if err != nil {
		t.Fatal(err)
	}
	var kept bool
	for _, exp := range final {
		if exp.Version == fresh.Version {
			kept = exp.Status == "fertig" && exp.ErsetztDurchVersion != nil && *exp.ErsetztDurchVersion == next.Version
		}
	}
	if !kept {
		t.Fatalf("replaced %+v", final)
	}
	if err := s.DeleteAbrechnung(ctx, ada.ID, second.Row.ID, actor); codeOf(err) != "uebergang" {
		t.Fatal(err)
	}
}

func TestAbrechnungDeleteDraft(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	ada := mustNutzer(t, s, "ada")
	actor := Actor{Art: "nutzer", NutzerID: &ada.ID}
	ag, err := s.CreateArbeitgeber(ctx, ada.ID, ArbeitgeberInput{Name: "ACME", Anschrift: "Weg 1"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	row := trip(t, s, ada.ID, actor, ag.ID, "2026-10-01T08:00:00", "2026-10-01T12:00:00")
	bundle, err := s.CreateAbrechnung(ctx, ada.ID, AbrechnungInput{
		ArbeitgeberID: ag.ID, ZeitraumArt: "tag", Von: "2026-10-01", Bis: "2026-10-01",
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAbrechnung(ctx, ada.ID, bundle.Row.ID, actor); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetReiseBundle(ctx, ada.ID, row.Reise.ID)
	if err != nil || got.Reise.Status != "offen" {
		t.Fatalf("%s %v", got.Reise.Status, err)
	}
}

func trip(t *testing.T, s *Store, nutzerID string, actor Actor, agID, beginn, ende string) ReiseBundle {
	t.Helper()
	row, err := s.CreateReise(context.Background(), nutzerID, ReiseInput{
		ArbeitgeberID: agID, Anlass: "Termin",
		BeginnLokal: beginn, BeginnZone: "Europe/Berlin",
		EndeLokal: ende, EndeZone: "Europe/Berlin",
		UnterkunftDefault: "gestellt",
		Ortswechsel: []OrtswechselInput{{
			AnkunftLokal: beginn, AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func codeOf(err error) string {
	var code *CodeError
	if errors.As(err, &code) {
		return code.Code
	}
	return ""
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
