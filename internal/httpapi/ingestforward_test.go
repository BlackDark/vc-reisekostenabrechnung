package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
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
	if ingest.Scope != "mail:ingest" {
		t.Fatalf("ingest token scope wrong: %s", rec.Body)
	}
	// The address travels through mail servers, mailboxes and logs, so it carries
	// the hash the database stores: whoever reads it can send mail into the
	// account, but it is not a bearer token for the API.
	want := "belege+" + auth.HashBearer(ingest.Token) + "@example.com"
	if ingest.ForwardingAddress != want {
		t.Fatalf("ingest token forwarding address %q, want %q", ingest.ForwardingAddress, want)
	}
	if strings.Contains(ingest.ForwardingAddress, ingest.Token) {
		t.Fatalf("the address carries the token: %q", ingest.ForwardingAddress)
	}
	// The address still selects its owner, and only its own domain does.
	nutzerID, ok := app.resolveMailUser(t.Context(), envelopeFor(ingest.ForwardingAddress))
	if !ok || nutzerID == "" {
		t.Fatalf("address %q found no owner", ingest.ForwardingAddress)
	}
	if _, ok := app.resolveMailUser(t.Context(), envelopeFor("belege+"+
		auth.HashBearer(ingest.Token)+"@andere.example")); ok {
		t.Fatalf("a foreign domain resolved to an owner")
	}

	up := doCookie(t, h, http.MethodPost, "/api/v1/me/tokens", `{"name":"up","scope":"belege:upload"}`, cookie)
	if up.Code != http.StatusCreated {
		t.Fatalf("create upload token %d", up.Code)
	}
	if strings.Contains(up.Body.String(), "forwarding_address") {
		t.Fatalf("upload token should not carry a forwarding address: %s", up.Body)
	}
}

// envelopeFor returns the envelope of a message addressed to one recipient.
func envelopeFor(address string) *imap.Envelope {
	at := strings.LastIndex(address, "@")
	return &imap.Envelope{To: []imap.Address{{Mailbox: address[:at], Host: address[at+1:]}}}
}
