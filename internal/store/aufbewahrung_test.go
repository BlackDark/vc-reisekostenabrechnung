package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRetentionBlocksEarlyDelete(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	n := mustNutzer(t, s, "ada")
	actor := Actor{Art: "nutzer", NutzerID: &n.ID}
	row, err := s.CreateBeleg(ctx, n.ID, NewBeleg{
		Typ: "pdf", Seiten: 1, SHA256: strings.Repeat("ab", 32), Status: "zur_bestaetigung",
		Dateien: []DateiIn{{
			Variante: "original", Seite: 1, Key: "nutzer/" + n.ID + "/belege/b/original-1.pdf",
			MIME: "application/pdf", Bytes: 4, SHA256: strings.Repeat("cd", 32),
		}},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := s.ConfirmBeleg(ctx, n.ID, row.ID, row.Version, time.Hour, actor)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Belegnummer == nil || confirmed.AufbewahrenBis == nil {
		t.Fatalf("metadata %+v", confirmed)
	}
	today := "2026-10-09"
	_, err = s.PurgeRetention(ctx, actor, PurgeInput{
		BelegIDs: []string{confirmed.ID}, Grund: "Nutzer hat die Löschung verlangt", Ablaufhemmung: false, Today: today,
	})
	if codeOf(err) != "ablaufhemmung" {
		t.Fatalf("ack %v", err)
	}
	if _, err := s.write.ExecContext(ctx, "UPDATE beleg SET aufbewahren_bis = ? WHERE id = ?", "2099-12-31", confirmed.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.PurgeRetention(ctx, actor, PurgeInput{
		BelegIDs: []string{confirmed.ID}, Grund: "Nutzer hat die Löschung verlangt", Ablaufhemmung: true, Today: today,
	})
	if codeOf(err) != "frist" {
		t.Fatalf("early %v", err)
	}
	if _, err := s.write.ExecContext(ctx, "UPDATE beleg SET aufbewahren_bis = ? WHERE id = ?", "2020-12-31", confirmed.ID); err != nil {
		t.Fatal(err)
	}
	keys, err := s.PurgeRetention(ctx, actor, PurgeInput{
		BelegIDs: []string{confirmed.ID}, Grund: "Nutzer hat die Löschung verlangt", Ablaufhemmung: true, Today: today,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("keys %v", keys)
	}
	files, err := s.ListBelegdateien(ctx, confirmed.ID)
	if err != nil || len(files) != 0 {
		t.Fatalf("files %+v %v", files, err)
	}
	got, err := s.GetBeleg(ctx, n.ID, confirmed.ID)
	if err != nil || got.InhaltGeloeschtAm == nil || got.Belegnummer == nil || got.Sha256Original == "" {
		t.Fatalf("kept metadata %+v %v", got, err)
	}
	report, err := s.RetentionReport(ctx, today)
	if err != nil || len(report) != 1 || !report[0].InhaltGeloescht || !report[0].Abgelaufen {
		t.Fatalf("report %+v %v", report, err)
	}
}
