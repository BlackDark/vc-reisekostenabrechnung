package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/fx"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

// The month rate is looked up once per currency and month and then held for the
// rest of the run, because a trip usually carries several lines in the same
// month. Every line of that month has to report the rate, and a line in another
// currency has to report its own rather than whichever was asked for first.
func TestExpenseLinesReportTheirOwnMonthRate(t *testing.T) {
	app := newTestApp(t, nil)
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
	ag := doJSON(t, h, http.MethodPost, "/api/v1/arbeitgeber", `{"name":"Alpha","Anschrift":"Weg 1"}`, hdr)
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
		BeginnLokal: "2026-09-11T08:00:00", BeginnZone: "Europe/Berlin",
		EndeLokal: "2026-09-12T18:00:00", EndeZone: "Europe/Berlin",
		Ortswechsel: []store.OrtswechselInput{{
			AnkunftLokal: "2026-09-11T08:00:00", AnkunftZone: "Europe/Berlin",
			Verkehrsmittel: "bahn", LandISO: "DE",
		}},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	// The dollar month rate comes from the shipped table and the franc one from the
	// stored table, so the two answers cannot be confused for each other. The daily
	// rates keep the lines out of the missing-rate block.
	if err := app.store.RememberRates(t.Context(), []fx.Rate{
		{Waehrung: "USD", Datum: "2026-09-11", Kurs: "1.1650"},
		{Waehrung: "CHF", Datum: "2026-09-11", Kurs: "1.0800"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.store.RememberMonth(t.Context(), 2026, 9, "CHF", "0.950000"); err != nil {
		t.Fatal(err)
	}
	lines := []struct{ datum, waehrung string }{
		{"2026-09-11", "USD"},
		{"2026-09-12", "USD"},
		{"2026-09-12", "CHF"},
	}
	for _, line := range lines {
		if _, err := app.store.CreateAusgabe(t.Context(), me.ID, trip.Reise.ID, store.AusgabeInput{
			Kostenart: "reisenebenkosten", Datum: line.datum, Betrag: 4850, BetragEUR: 4163, Waehrung: line.waehrung,
			Anteile: []store.AnteilInput{{Satz: 1900, Steuerland: "DE", Brutto: 4850}},
		}, actor); err != nil {
			t.Fatal(err)
		}
	}

	want := map[string]string{
		"2026-09-11 USD": "1.170200",
		"2026-09-12 USD": "1.170200",
		"2026-09-12 CHF": "0.950000",
	}
	res := authed(t, h, cookie, http.MethodGet, "/api/v1/reisen/"+trip.Reise.ID+"/ausgaben", nil, "")
	if res.Code != http.StatusOK {
		t.Fatalf("list %d %s", res.Code, res.Body)
	}
	var list struct {
		Items []struct {
			Datum    string  `json:"datum"`
			Waehrung string  `json:"waehrung"`
			UstKurs  *string `json:"ust_kurs"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != len(lines) {
		t.Fatalf("%d lines, want %d", len(list.Items), len(lines))
	}
	for _, item := range list.Items {
		key := item.Datum + " " + item.Waehrung
		rate, ok := want[key]
		if !ok {
			t.Fatalf("unexpected line %s", key)
		}
		if item.UstKurs == nil || *item.UstKurs != rate {
			t.Fatalf("line %s reports month rate %v, want %s", key, item.UstKurs, rate)
		}
	}
}
