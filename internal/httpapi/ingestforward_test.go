package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
)

func TestIngestTokenForwardingAddress(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) { cfg.Mail.ForwardDomain = "example.com" })
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()
	cookie := loginCookie(t, h)

	rec := doCookie(t, h, http.MethodPost, "/api/v1/me/tokens", `{"name":"inbox","scope":"mail:ingest"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ingest token %d %s", rec.Code, rec.Body)
	}
	var ingest struct {
		Token             string `json:"token"`
		Scope             string `json:"scope"`
		ForwardingAddress string `json:"forwarding_address"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ingest); err != nil {
		t.Fatal(err)
	}
	if ingest.Scope != "mail:ingest" || ingest.ForwardingAddress != "belege+"+ingest.Token+"@example.com" {
		t.Fatalf("ingest token forwarding address wrong: %s", rec.Body)
	}

	up := doCookie(t, h, http.MethodPost, "/api/v1/me/tokens", `{"name":"up","scope":"belege:upload"}`, cookie)
	if up.Code != http.StatusCreated {
		t.Fatalf("create upload token %d", up.Code)
	}
	if strings.Contains(up.Body.String(), "forwarding_address") {
		t.Fatalf("upload token should not carry a forwarding address: %s", up.Body)
	}
}
