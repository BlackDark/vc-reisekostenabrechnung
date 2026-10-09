package ki

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExtractModes(t *testing.T) {
	day := time.Now().UTC().Format("2006-01-02")
	good := fieldsJSON(t, day, 4850, 4850)
	cases := []struct {
		name   string
		format string
		handle func(w http.ResponseWriter, r *http.Request, body []byte)
		want   string
		empty  bool
	}{
		{
			name:   "json_schema",
			format: "json_schema",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				if !strings.Contains(string(body), `"json_schema"`) {
					http.Error(w, "mode", http.StatusBadRequest)
					return
				}
				writeChoice(w, good)
			},
			want: "json_schema",
		},
		{
			name:   "json_object",
			format: "json_object",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				if !strings.Contains(string(body), `"json_object"`) {
					http.Error(w, "mode", http.StatusBadRequest)
					return
				}
				writeChoice(w, good)
			},
			want: "json_object",
		},
		{
			name:   "text",
			format: "text",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				if strings.Contains(string(body), "response_format") {
					http.Error(w, "mode", http.StatusBadRequest)
					return
				}
				writeChoice(w, "```json\n"+good+"\n```")
			},
			want: "text",
		},
		{
			name:   "auto falls through",
			format: "auto",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				raw := string(body)
				if strings.Contains(raw, `"json_schema"`) || strings.Contains(raw, `"json_object"`) {
					http.Error(w, "unsupported", http.StatusBadRequest)
					return
				}
				writeChoice(w, good)
			},
			want: "text",
		},
		{
			name:   "implausible sum is empty",
			format: "json_object",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				writeChoice(w, fieldsJSON(t, day, 4850, 100))
			},
			empty: true,
		},
		{
			name:   "date outside a year is empty",
			format: "json_object",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				writeChoice(w, fieldsJSON(t, "2001-01-01", 4850, 4850))
			},
			empty: true,
		},
		{
			name:   "garbage is empty",
			format: "text",
			handle: func(w http.ResponseWriter, r *http.Request, body []byte) {
				writeChoice(w, "not a receipt")
			},
			empty: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sawImage atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" {
					http.NotFound(w, r)
					return
				}
				if r.Header.Get("Authorization") != "Bearer sk-test-secret-value" {
					http.Error(w, "auth", http.StatusUnauthorized)
					return
				}
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), PromptMarker) || !strings.Contains(string(body), "data:image/jpeg;base64,") {
					t.Errorf("request missing prompt or image")
				}
				if strings.Contains(string(body), "sk-test-secret-value") {
					t.Errorf("api key leaked into the body")
				}
				sawImage.Store(true)
				tc.handle(w, r, body)
			}))
			t.Cleanup(srv.Close)
			c := NewClient(Settings{
				BaseURL: srv.URL + "/v1", APIKey: "sk-test-secret-value", Model: "fake",
				Format: tc.format, Timeout: 2 * time.Second,
			})
			c.RetryWait = time.Millisecond
			out, err := c.Extract(context.Background(), [][]byte{tinyJPEG(t)})
			if err != nil {
				t.Fatal(err)
			}
			if !sawImage.Load() {
				t.Fatal("endpoint was not called")
			}
			if tc.empty {
				if !out.Empty || out.Fields.BetragBrutto != 0 {
					t.Fatalf("%+v", out)
				}
				return
			}
			if out.Empty || out.Mode != tc.want || out.Fields.Leistender != "Cafe Roma" || out.Fields.BetragBrutto != 4850 {
				t.Fatalf("%+v", out)
			}
			if out.Fields.EmpfaengerName != "Fremde GmbH" || out.Fields.Waehrung != "USD" {
				t.Fatalf("%+v", out.Fields)
			}
		})
	}
}

func TestExtractRetriesThenTimesOut(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			http.Error(w, "busy", http.StatusBadGateway)
			return
		}
		writeChoice(w, fieldsJSON(t, time.Now().UTC().Format("2006-01-02"), 100, 100))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(Settings{BaseURL: srv.URL + "/v1", Model: "fake", Format: "json_object", Timeout: 2 * time.Second})
	c.RetryWait = time.Millisecond
	out, err := c.Extract(context.Background(), [][]byte{tinyJPEG(t)})
	if err != nil || out.Empty || hits.Load() != 3 {
		t.Fatalf("out %+v err %v hits %d", out, err, hits.Load())
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)
	c = NewClient(Settings{BaseURL: slow.URL + "/v1", Model: "fake", Format: "text", Timeout: 50 * time.Millisecond, APIKey: "sk-test-secret-value"})
	c.RetryWait = time.Millisecond
	_, err = c.Extract(context.Background(), [][]byte{tinyJPEG(t)})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatal(err)
	}
	if strings.Contains(err.Error(), "sk-test-secret-value") || strings.Contains(err.Error(), PromptMarker) {
		t.Fatal(err)
	}
}

func TestGateBlocks(t *testing.T) {
	g := NewGate(1)
	ctx := context.Background()
	if err := g.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if err := g.Acquire(ctx); err == nil {
		t.Fatal("expected block")
	}
	g.Release()
	if err := g.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFitJPEGShrinks(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	out, err := FitJPEG(b.Bytes(), 20)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 20 || got.Bounds().Dy() != 10 {
		t.Fatalf("%v", got.Bounds())
	}
}

func fieldsJSON(t *testing.T, day string, gross, share int64) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"leistender": "Cafe Roma", "datum": day, "waehrung": "usd", "betrag_brutto": gross,
		"steueranteile": []map[string]any{{"satz": 1900, "netto": share - 774, "steuer": 774, "brutto": share}},
		"rechnungsart":  "kleinbetragsrechnung", "kostenart": "verpflegung",
		"empfaenger_name": "Fremde GmbH", "rechnungsnummer": "R-1",
		"ust_id_leistender": "DE123", "trinkgeld": 0, "volltext": "Cafe Roma",
		"konfidenz": map[string]float64{"leistender": 0.9},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func writeChoice(w http.ResponseWriter, content string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"role": "assistant", "content": content}},
		},
	})
}

func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
