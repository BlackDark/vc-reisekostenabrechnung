// Package audit seals and verifies the append-only hash chain.
package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Event is one row of the audit chain. Hash is excluded from the canonical payload.
type Event struct {
	ID             string
	Zeitpunkt      time.Time
	AkteurNutzerID *string
	AkteurArt      string
	Aktion         string
	ObjektTyp      string
	ObjektID       string
	Vorher         json.RawMessage
	Nachher        json.RawMessage
	Grund          *string
	IP             string
	VorgaengerHash string
	Hash           string
}

type canon struct {
	ID             string          `json:"id"`
	Zeitpunkt      string          `json:"zeitpunkt"`
	AkteurNutzerID *string         `json:"akteur_nutzer_id"`
	AkteurArt      string          `json:"akteur_art"`
	Aktion         string          `json:"aktion"`
	ObjektTyp      string          `json:"objekt_typ"`
	ObjektID       string          `json:"objekt_id"`
	Vorher         json.RawMessage `json:"vorher"`
	Nachher        json.RawMessage `json:"nachher"`
	Grund          *string         `json:"grund"`
	IP             string          `json:"ip"`
	VorgaengerHash string          `json:"vorgaenger_hash"`
}

// Canonical JSON of the event, excluding the hash itself.
func Canonical(e Event) ([]byte, error) {
	c := canon{
		ID:             e.ID,
		Zeitpunkt:      e.Zeitpunkt.UTC().Format(time.RFC3339Nano),
		AkteurNutzerID: e.AkteurNutzerID,
		AkteurArt:      e.AkteurArt,
		Aktion:         e.Aktion,
		ObjektTyp:      e.ObjektTyp,
		ObjektID:       e.ObjektID,
		Vorher:         compact(e.Vorher),
		Nachher:        compact(e.Nachher),
		Grund:          e.Grund,
		IP:             e.IP,
		VorgaengerHash: e.VorgaengerHash,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("audit canonical: %w", err)
	}
	return b, nil
}

func compact(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}

// Seal sets VorgaengerHash from prev and returns the SHA-256 hex hash.
func Seal(prev string, e *Event) error {
	e.VorgaengerHash = prev
	payload, err := Canonical(*e)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(append([]byte(prev), append([]byte("\n"), payload...)...))
	e.Hash = hex.EncodeToString(sum[:])
	return nil
}

// Checker verifies a hash chain one event at a time. The chain is as long as the
// installation is old, so the caller can read it in pages and hand over what it
// has rather than holding every event at once.
type Checker struct {
	prev string
	seen int
}

// Add checks that e continues the chain from the events already added, and
// remembers its hash for the next call.
func (c *Checker) Add(e Event) error {
	if e.VorgaengerHash != c.prev {
		return fmt.Errorf("audit chain broken at %s: predecessor mismatch", e.ID)
	}
	want := e
	if err := Seal(c.prev, &want); err != nil {
		return err
	}
	if want.Hash != e.Hash {
		return fmt.Errorf("audit chain broken at %s (index %d): hash mismatch", e.ID, c.seen)
	}
	c.prev = e.Hash
	c.seen++
	return nil
}

// Verify checks that every event links to the previous hash.
func Verify(events []Event) error {
	var c Checker
	for _, e := range events {
		if err := c.Add(e); err != nil {
			return err
		}
	}
	return nil
}
