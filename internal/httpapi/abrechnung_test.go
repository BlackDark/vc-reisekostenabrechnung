package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

func TestAbrechnungSubmitAndExport(t *testing.T) {
	typst := os.Getenv("TYPST_PATH")
	app := newTestApp(t, func(cfg *config.Config) {
		if filepath.Base(typst) == "typst" || filepath.Base(typst) == "typst.exe" {
			cfg.TypstPath = typst
		}
	})
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	login := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if login.Code != http.StatusOK || len(login.Result().Cookies()) == 0 {
		t.Fatalf("login %d %s", login.Code, login.Body)
	}
	cookie := login.Result().Cookies()[0]
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &me); err != nil || me.ID == "" {
		t.Fatalf("me %s", login.Body)
	}
	hdr := map[string]string{"Cookie": cookie.Name + "=" + cookie.Value}
	ag := doJSON(t, h, http.MethodPost, "/api/v1/arbeitgeber", `{"name":"Alpha","anschrift":"Weg 1"}`, hdr)
	if ag.Code != http.StatusCreated {
		t.Fatalf("ag %d %s", ag.Code, ag.Body)
	}
	var employer struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ag.Body.Bytes(), &employer); err != nil {
		t.Fatal(err)
	}
	actor := store.Actor{Art: "nutzer", NutzerID: &me.ID}
	trip, err := app.store.CreateReise(t.Context(), me.ID, store.ReiseInput{
		ArbeitgeberID: employer.ID, Anlass: "Termin",
		BeginnLokal: "2026-10-01T08:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-10-02T18:00:00", EndeZone: "Europe/Berlin",
		UnterkunftDefault: "gestellt",
		Ortswechsel: []store.OrtswechselInput{{
			AnkunftLokal: "2026-10-01T08:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"arbeitgeber_id":"` + employer.ID + `","zeitraum_art":"monat","von":"2026-10-01","bis":"2026-10-31","sprache":"de"}`
	created := doJSON(t, h, http.MethodPost, "/api/v1/abrechnungen", body, hdr)
	if created.Code != http.StatusCreated {
		t.Fatalf("create %d %s", created.Code, created.Body)
	}
	var claim struct {
		ID      string   `json:"id"`
		Version int64    `json:"version"`
		Reisen  []string `json:"reise_ids"`
		Titel   string   `json:"titel"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	if claim.Titel != "Reisekosten Oktober 2026" || len(claim.Reisen) != 1 || claim.Reisen[0] != trip.Reise.ID {
		t.Fatalf("claim %+v", claim)
	}
	check := doJSON(t, h, http.MethodGet, "/api/v1/abrechnungen/"+claim.ID+"/pruefung", "", hdr)
	if check.Code != http.StatusOK || strings.Contains(check.Body.String(), `"code":"B0`) {
		t.Fatalf("pruefung %d %s", check.Code, check.Body)
	}
	var probe struct {
		Warnungen []struct {
			Code     string `json:"code"`
			ObjektID string `json:"objekt_id"`
		} `json:"warnungen"`
		Auszahlung int64 `json:"auszahlung_cent"`
	}
	if err := json.Unmarshal(check.Body.Bytes(), &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Auszahlung <= 0 {
		t.Fatalf("auszahlung %s", check.Body)
	}
	var quitt []string
	for _, w := range probe.Warnungen {
		quitt = append(quitt, `{"code":"`+w.Code+`","objekt_id":"`+w.ObjektID+`"}`)
	}
	submitHdr := map[string]string{
		"Cookie": hdr["Cookie"], "If-Match": "1",
	}
	if claim.Version != 1 {
		submitHdr["If-Match"] = jsonNumber(claim.Version)
	}
	submit := doJSON(t, h, http.MethodPost, "/api/v1/abrechnungen/"+claim.ID+"/einreichen", `{"quittierte_warnungen":[`+strings.Join(quitt, ",")+`]}`, submitHdr)
	if submit.Code != http.StatusAccepted {
		t.Fatalf("submit %d %s", submit.Code, submit.Body)
	}
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := doJSON(t, h, http.MethodGet, "/api/v1/abrechnungen/"+claim.ID, "", hdr)
	if got.Code != http.StatusOK {
		t.Fatalf("get %d %s", got.Code, got.Body)
	}
	ready := app.typstBin() != ""
	if ready {
		if !strings.Contains(got.Body.String(), `"status":"eingereicht"`) {
			t.Fatalf("expected eingereicht %s", got.Body)
		}
		files := doJSON(t, h, http.MethodGet, "/api/v1/abrechnungen/"+claim.ID+"/exporte", "", hdr)
		var list struct {
			Items []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal(files.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0].Status != "fertig" {
			t.Fatalf("exporte %s", files.Body)
		}
		pdf := doJSON(t, h, http.MethodGet, "/api/v1/exporte/"+list.Items[0].ID+"/pdf", "", hdr)
		if pdf.Code != http.StatusOK || !strings.HasPrefix(pdf.Body.String(), "%PDF") {
			t.Fatalf("pdf %d %s", pdf.Code, pdf.Body.String()[:min(80, pdf.Body.Len())])
		}
		return
	}
	if !strings.Contains(got.Body.String(), `"status":"entwurf"`) || !strings.Contains(got.Body.String(), "typst missing") {
		t.Fatalf("rollback %s", got.Body)
	}
	again, err := app.store.GetReiseBundle(t.Context(), me.ID, trip.Reise.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Reise.Status != "in_entwurf" {
		t.Fatalf("status %s", again.Reise.Status)
	}
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
