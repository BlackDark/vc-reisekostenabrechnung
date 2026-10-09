package store

import (
	"testing"
)

func TestResumeRunningJob(t *testing.T) {
	ctx := t.Context()
	s := openTest(t)
	n, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Ada", Benutzername: "ada", Sprache: "de", Aktiv: true, IstAdminLokal: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBeleg(ctx, n.ID, NewBeleg{
		Typ: "foto", Seiten: 1, SHA256: "abc", Status: "in_aufbereitung", JobArt: "aufbereiten",
	}, Actor{Art: "nutzer", NutzerID: &n.ID}); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimJob(ctx)
	if err != nil || job.Status != "running" || job.Versuche != 1 {
		t.Fatalf("%+v %v", job, err)
	}
	if err := s.ResumeJobs(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := s.ClaimJob(ctx)
	if err != nil || again.ID != job.ID || again.Versuche != 2 {
		t.Fatalf("%+v %v", again, err)
	}
	if err := s.FailBelegJob(ctx, again, errString("later")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimJob(ctx); err != ErrNotFound {
		t.Fatalf("due early %v", err)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
