package audit

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSealAndVerify(t *testing.T) {
	when := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	first := Event{
		ID:        "018f0000-0000-7000-8000-000000000001",
		Zeitpunkt: when,
		AkteurArt: "system",
		Aktion:    "nutzer.angelegt",
		ObjektTyp: "nutzer",
		ObjektID:  "018f0000-0000-7000-8000-000000000001",
		Nachher:   json.RawMessage(`{"benutzername":"ada"}`),
		IP:        "127.0.0.1",
	}
	if err := Seal("", &first); err != nil {
		t.Fatal(err)
	}
	if first.Hash == "" || first.VorgaengerHash != "" {
		t.Fatalf("unexpected seal: %+v", first)
	}

	second := Event{
		ID:        "018f0000-0000-7000-8000-000000000002",
		Zeitpunkt: when.Add(time.Second),
		AkteurArt: "nutzer",
		Aktion:    "anmeldung",
		ObjektTyp: "nutzer",
		ObjektID:  first.ObjektID,
		IP:        "127.0.0.1",
	}
	if err := Seal(first.Hash, &second); err != nil {
		t.Fatal(err)
	}
	if err := Verify([]Event{first, second}); err != nil {
		t.Fatal(err)
	}

	tampered := second
	tampered.IP = "10.0.0.9"
	if err := Verify([]Event{first, tampered}); err == nil {
		t.Fatal("expected tamper to fail verification")
	}
}
