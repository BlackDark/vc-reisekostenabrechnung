// Package ki calls an OpenAI-compatible chat endpoint and validates receipt suggestions.
// Prompts, images, and API keys are never written to logs by this package.
package ki

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// PromptMarker is part of the prompt so tests can prove it never reaches logs.
const PromptMarker = "belegfelder-v1"

const prompt = PromptMarker + ` Extract the receipt. Amounts are integer cents. satz is hundredths of a percent (1900 = 19). Return JSON with leistender, datum (YYYY-MM-DD), waehrung, betrag_brutto, steueranteile as objects with satz, netto, steuer, brutto, plus rechnungsart, kostenart, empfaenger_name, rechnungsnummer, ust_id_leistender, trinkgeld, volltext, konfidenz.`

// Settings configures one client. An empty BaseURL or Model means the caller must not dial.
type Settings struct {
	BaseURL string
	APIKey  string
	Model   string
	Format  string
	Timeout time.Duration
}

// Anteil is one VAT share. Amounts are cents. Satz is hundredths of a percent.
type Anteil struct {
	Satz   int   `json:"satz"`
	Netto  int64 `json:"netto"`
	Steuer int64 `json:"steuer"`
	Brutto int64 `json:"brutto"`
}

// Fields is a validated suggestion. Empty amounts mean the model reply was unusable.
type Fields struct {
	Leistender      string             `json:"leistender"`
	Datum           string             `json:"datum"`
	Waehrung        string             `json:"waehrung"`
	BetragBrutto    int64              `json:"betrag_brutto"`
	Steueranteile   []Anteil           `json:"steueranteile"`
	Rechnungsart    string             `json:"rechnungsart"`
	Kostenart       string             `json:"kostenart"`
	EmpfaengerName  string             `json:"empfaenger_name"`
	Rechnungsnummer string             `json:"rechnungsnummer"`
	UstIDLeistender string             `json:"ust_id_leistender"`
	Trinkgeld       int64              `json:"trinkgeld"`
	Volltext        string             `json:"volltext"`
	Konfidenz       map[string]float64 `json:"konfidenz"`
}

// Outcome is a suggestion or an intentionally empty one. A non-nil error means the endpoint was unreachable.
type Outcome struct {
	Fields Fields
	Empty  bool
	Mode   string
}

// Client talks to POST {BaseURL}/chat/completions.
type Client struct {
	Settings
	HTTP      *http.Client
	Now       func() time.Time
	RetryWait time.Duration
}

// NewClient builds a client. Timeout defaults to 60s.
func NewClient(s Settings) *Client {
	if s.Timeout <= 0 {
		s.Timeout = 60 * time.Second
	}
	if s.Format == "" {
		s.Format = "auto"
	}
	return &Client{
		Settings:  s,
		HTTP:      &http.Client{Timeout: s.Timeout},
		Now:       time.Now,
		RetryWait: 100 * time.Millisecond,
	}
}

// Gate limits how many extractions run at once.
type Gate struct {
	sem chan struct{}
}

// NewGate allows n concurrent extractions.
func NewGate(n int) *Gate {
	if n < 1 {
		n = 1
	}
	return &Gate{sem: make(chan struct{}, n)}
}

// Acquire blocks until a slot is free or ctx ends.
func (g *Gate) Acquire(ctx context.Context) error {
	if g == nil {
		return nil
	}
	select {
	case g.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns a slot.
func (g *Gate) Release() {
	if g == nil {
		return
	}
	select {
	case <-g.sem:
	default:
	}
}

// Extract sends JPEG pages and parses a suggestion. Faulty replies yield an empty outcome.
func (c *Client) Extract(ctx context.Context, images [][]byte) (Outcome, error) {
	if c == nil || c.BaseURL == "" || c.Model == "" {
		return Outcome{}, errors.New("ki: disabled")
	}
	sawOK := false
	var last error
	for _, mode := range c.plan() {
		body, status, err := c.callWithRetry(ctx, images, mode)
		if err != nil {
			last = c.scrub(err)
			if status == http.StatusBadRequest || status == http.StatusUnprocessableEntity || status == http.StatusUnsupportedMediaType {
				continue
			}
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				return Outcome{}, last
			}
			if !sawOK {
				return Outcome{}, last
			}
			continue
		}
		sawOK = true
		fields, ok := parseContent(body)
		if !ok {
			continue
		}
		fields = normalize(fields)
		if !plausible(fields, c.now()) {
			return Outcome{Empty: true, Mode: mode}, nil
		}
		return Outcome{Fields: fields, Mode: mode}, nil
	}
	if !sawOK && last != nil {
		return Outcome{}, last
	}
	return Outcome{Empty: true}, nil
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) plan() []string {
	switch c.Format {
	case "json_schema", "json_object", "text":
		return []string{c.Format}
	default:
		return []string{"json_schema", "json_object", "text"}
	}
}

func (c *Client) callWithRetry(ctx context.Context, images [][]byte, mode string) ([]byte, int, error) {
	var last error
	var status int
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			wait := c.RetryWait
			if wait <= 0 {
				wait = 100 * time.Millisecond
			}
			wait *= time.Duration(attempt)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, 0, ctx.Err()
			case <-timer.C:
			}
		}
		body, code, err := c.call(ctx, images, mode)
		status = code
		if err == nil {
			return body, code, nil
		}
		last = err
		if code == http.StatusBadRequest || code == http.StatusUnprocessableEntity || code == http.StatusUnsupportedMediaType || code == http.StatusUnauthorized || code == http.StatusForbidden {
			return body, code, err
		}
	}
	return nil, status, last
}

func (c *Client) call(ctx context.Context, images [][]byte, mode string) ([]byte, int, error) {
	payload, err := c.payload(images, mode)
	if err != nil {
		return nil, 0, err
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, 0, errors.New("ki: timeout")
		}
		return nil, 0, fmt.Errorf("ki: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, res.StatusCode, errors.New("ki: response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return raw, res.StatusCode, fmt.Errorf("ki: HTTP %d", res.StatusCode)
	}
	return raw, res.StatusCode, nil
}

func (c *Client) payload(images [][]byte, mode string) ([]byte, error) {
	content := make([]map[string]any, 0, 1+len(images))
	content = append(content, map[string]any{"type": "text", "text": prompt})
	for _, img := range images {
		if len(img) == 0 {
			continue
		}
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)},
		})
	}
	body := map[string]any{
		"model": c.Model,
		"messages": []map[string]any{
			{"role": "user", "content": content},
		},
	}
	switch mode {
	case "json_schema":
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "beleg",
				"strict": false,
				"schema": schema(),
			},
		}
	case "json_object":
		body["response_format"] = map[string]any{"type": "json_object"}
	}
	return json.Marshal(body)
}

func (c *Client) scrub(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if c.APIKey != "" {
		msg = strings.ReplaceAll(msg, c.APIKey, "[redacted]")
	}
	msg = strings.ReplaceAll(msg, PromptMarker, "[redacted]")
	msg = strings.ReplaceAll(msg, "data:image", "[redacted]")
	return errors.New(msg)
}

func schema() map[string]any {
	share := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"satz", "netto", "steuer", "brutto"},
		"properties": map[string]any{
			"satz":   map[string]any{"type": "integer"},
			"netto":  map[string]any{"type": "integer"},
			"steuer": map[string]any{"type": "integer"},
			"brutto": map[string]any{"type": "integer"},
		},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"leistender", "datum", "waehrung", "betrag_brutto", "steueranteile",
			"rechnungsart", "kostenart", "empfaenger_name", "rechnungsnummer",
			"ust_id_leistender", "trinkgeld", "volltext", "konfidenz",
		},
		"properties": map[string]any{
			"leistender":        map[string]any{"type": "string"},
			"datum":             map[string]any{"type": "string"},
			"waehrung":          map[string]any{"type": "string"},
			"betrag_brutto":     map[string]any{"type": "integer"},
			"steueranteile":     map[string]any{"type": "array", "items": share},
			"rechnungsart":      map[string]any{"type": "string"},
			"kostenart":         map[string]any{"type": "string"},
			"empfaenger_name":   map[string]any{"type": "string"},
			"rechnungsnummer":   map[string]any{"type": "string"},
			"ust_id_leistender": map[string]any{"type": "string"},
			"trinkgeld":         map[string]any{"type": "integer"},
			"volltext":          map[string]any{"type": "string"},
			"konfidenz":         map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "number"}},
		},
	}
}

func parseContent(raw []byte) (Fields, bool) {
	var envelope struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Choices) == 0 {
		return Fields{}, false
	}
	text := contentText(envelope.Choices[0].Message.Content)
	blob := extractJSON(text)
	if blob == nil {
		return Fields{}, false
	}
	var parsed rawFields
	if err := json.Unmarshal(blob, &parsed); err != nil {
		return Fields{}, false
	}
	return parsed.fields(), true
}

type flexInt int64

func (n *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*n = 0
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		b = []byte(strings.TrimSpace(s))
		if len(b) == 0 {
			*n = 0
			return nil
		}
	}
	var i int64
	if err := json.Unmarshal(b, &i); err == nil {
		*n = flexInt(i)
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if math.Abs(f-math.Round(f)) > 0.001 {
		return errors.New("not integer cents")
	}
	*n = flexInt(math.Round(f))
	return nil
}

type rawAnteil struct {
	Satz   flexInt `json:"satz"`
	Netto  flexInt `json:"netto"`
	Steuer flexInt `json:"steuer"`
	Brutto flexInt `json:"brutto"`
}

type rawFields struct {
	Leistender      string             `json:"leistender"`
	Datum           string             `json:"datum"`
	Waehrung        string             `json:"waehrung"`
	BetragBrutto    flexInt            `json:"betrag_brutto"`
	Steueranteile   []rawAnteil        `json:"steueranteile"`
	Rechnungsart    string             `json:"rechnungsart"`
	Kostenart       string             `json:"kostenart"`
	EmpfaengerName  string             `json:"empfaenger_name"`
	Rechnungsnummer string             `json:"rechnungsnummer"`
	UstIDLeistender string             `json:"ust_id_leistender"`
	Trinkgeld       flexInt            `json:"trinkgeld"`
	Volltext        string             `json:"volltext"`
	Konfidenz       map[string]float64 `json:"konfidenz"`
}

func (r rawFields) fields() Fields {
	out := Fields{
		Leistender: r.Leistender, Datum: r.Datum, Waehrung: r.Waehrung,
		BetragBrutto: int64(r.BetragBrutto), Rechnungsart: r.Rechnungsart, Kostenart: r.Kostenart,
		EmpfaengerName: r.EmpfaengerName, Rechnungsnummer: r.Rechnungsnummer,
		UstIDLeistender: r.UstIDLeistender, Trinkgeld: int64(r.Trinkgeld), Volltext: r.Volltext,
		Konfidenz: r.Konfidenz,
	}
	for _, sh := range r.Steueranteile {
		satz := int64(sh.Satz)
		if satz < 0 || satz > 3000 {
			satz = -1
		}
		out.Steueranteile = append(out.Steueranteile, Anteil{
			Satz: int(satz), Netto: int64(sh.Netto), Steuer: int64(sh.Steuer), Brutto: int64(sh.Brutto),
		})
	}
	return out
}

func contentText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return ""
		}
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return string(raw)
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Text != "" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func extractJSON(s string) []byte {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil
	}
	return []byte(s[start : end+1])
}

func normalize(f Fields) Fields {
	f.Leistender = strings.TrimSpace(f.Leistender)
	f.Datum = strings.TrimSpace(f.Datum)
	f.EmpfaengerName = strings.TrimSpace(f.EmpfaengerName)
	f.Waehrung = strings.ToUpper(strings.TrimSpace(f.Waehrung))
	if len(f.Waehrung) != 3 {
		f.Waehrung = "EUR"
	}
	switch f.Kostenart {
	case "fahrtkosten", "verpflegung", "uebernachtung", "reisenebenkosten", "bewirtung":
	default:
		f.Kostenart = ""
	}
	switch f.Rechnungsart {
	case "rechnung", "kleinbetragsrechnung", "fahrausweis", "eigenbeleg":
	default:
		f.Rechnungsart = ""
	}
	if f.Konfidenz == nil {
		f.Konfidenz = map[string]float64{}
	}
	for k, v := range f.Konfidenz {
		if math.IsNaN(v) || v < 0 || v > 1 {
			delete(f.Konfidenz, k)
		}
	}
	return f
}

func plausible(f Fields, now time.Time) bool {
	if f.BetragBrutto <= 0 || !dateOK(f.Datum, now) {
		return false
	}
	if len(f.Steueranteile) == 0 {
		return true
	}
	var sum int64
	for _, sh := range f.Steueranteile {
		if sh.Satz < 0 || sh.Satz > 3000 || sh.Brutto < 0 {
			return false
		}
		sum += sh.Brutto
	}
	return sum == f.BetragBrutto
}

func dateOK(value string, now time.Time) bool {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return false
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	got := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return !got.Before(day.AddDate(-1, 0, 0)) && !got.After(day.AddDate(1, 0, 0))
}
