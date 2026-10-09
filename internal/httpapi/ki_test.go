package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/ki"
)

func TestKISuggestionRoundTrip(t *testing.T) {
	day := time.Now().UTC().Format("2006-01-02")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if !bytes.Contains(body, []byte(ki.PromptMarker)) || !bytes.Contains(body, []byte("data:image/jpeg;base64,")) {
			t.Errorf("missing prompt or image")
		}
		if bytes.Contains(body, []byte("sk-test-secret-value")) {
			t.Errorf("key in body")
		}
		raw, _ := json.Marshal(map[string]any{
			"leistender": "Cafe Roma", "datum": day, "waehrung": "USD", "betrag_brutto": 4850,
			"steueranteile": []map[string]any{{"satz": 1900, "netto": 4076, "steuer": 774, "brutto": 4850}},
			"kostenart":     "verpflegung", "empfaenger_name": "Fremde GmbH", "volltext": "Cafe Roma 48.50",
			"konfidenz":    map[string]float64{"leistender": 0.9},
			"rechnungsart": "kleinbetragsrechnung",
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": string(raw)}}},
		})
	}))
	t.Cleanup(srv.Close)

	app := newKIApp(t, srv.URL+"/v1", 2*time.Second)
	var logs bytes.Buffer
	app.log = slog.New(slog.NewJSONHandler(&logs, nil))
	h := app.Handler()
	cookie := loginCookie(t, h)
	allowKI(t, h, cookie)

	res := uploadBeleg(t, h, cookie, jpegBytesMarked(t, 3), "bon.jpg", "image/jpeg", false)
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload %d %s", res.Code, res.Body)
	}
	var uploaded map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	id, _ := uploaded["id"].(string)
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	queued := jsonAuth(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/ki-auslesen", nil, "")
	if queued.Code != http.StatusAccepted || hits.Load() != 0 {
		t.Fatalf("queue %d hits %d %s", queued.Code, hits.Load(), queued.Body)
	}
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits %d", hits.Load())
	}
	got := jsonAuth(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/ki", nil, "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "Cafe Roma") || !strings.Contains(got.Body.String(), "Fremde GmbH") {
		t.Fatalf("stand %d %s", got.Code, got.Body)
	}
	logText := logs.String()
	if !strings.Contains(logText, `"erfolg":true`) || strings.Contains(logText, "sk-test-secret-value") || strings.Contains(logText, ki.PromptMarker) || strings.Contains(logText, "data:image") {
		t.Fatalf("log %s", logText)
	}
	confirmed := jsonAuth(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/texte", []byte(`{"volltext":"Cafe Roma 48.50","felder":{"leistender":"Cafe Roma","betrag_brutto_cent":4850,"empfaenger_name":"Fremde GmbH"}}`), "")
	if confirmed.Code != http.StatusCreated || !strings.Contains(confirmed.Body.String(), `"quelle":"manuell"`) {
		t.Fatalf("confirm %d %s", confirmed.Code, confirmed.Body)
	}
	texts := jsonAuth(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/texte", nil, "")
	if texts.Code != http.StatusOK || strings.Count(texts.Body.String(), `"quelle"`) < 2 {
		t.Fatalf("texte %s", texts.Body)
	}
}

func TestKIStaysOffWithoutConfigOrOptIn(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.BelegFormat = "webp"
		cfg.BelegWebPQuality = 40
		cfg.BelegJPEGQuality = 70
	})
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	cfg := jsonAuth(t, h, nil, http.MethodGet, "/api/v1/auth/config", nil, "")
	if !strings.Contains(cfg.Body.String(), `"ai_aktiviert":false`) || strings.Contains(cfg.Body.String(), srv.URL) {
		t.Fatalf("config %s", cfg.Body)
	}
	cookie := loginCookie(t, h)
	res := uploadBeleg(t, h, cookie, jpegBytesMarked(t, 4), "bon.jpg", "image/jpeg", false)
	var uploaded map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &uploaded)
	id, _ := uploaded["id"].(string)
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	off := jsonAuth(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/ki-auslesen", nil, "")
	if off.Code != http.StatusConflict || !strings.Contains(off.Body.String(), "ki_deaktiviert") || hits.Load() != 0 {
		t.Fatalf("off %d %s", off.Code, off.Body)
	}

	on := newKIApp(t, srv.URL+"/v1", time.Second)
	h = on.Handler()
	cookie = loginCookie(t, h)
	res = uploadBeleg(t, h, cookie, jpegBytesMarked(t, 5), "bon.jpg", "image/jpeg", false)
	_ = json.Unmarshal(res.Body.Bytes(), &uploaded)
	id, _ = uploaded["id"].(string)
	if _, err := on.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	denied := jsonAuth(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/ki-auslesen", nil, "")
	if denied.Code != http.StatusConflict || !strings.Contains(denied.Body.String(), "ki_nicht_erlaubt") || hits.Load() != 0 {
		t.Fatalf("opt %d %s hits %d", denied.Code, denied.Body, hits.Load())
	}
}

func TestKIEmptyAndUnreachable(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "nope"}}},
		})
	}))
	t.Cleanup(bad.Close)
	app := newKIApp(t, bad.URL+"/v1", time.Second)
	h := app.Handler()
	cookie := loginCookie(t, h)
	allowKI(t, h, cookie)
	id := readyBeleg(t, app, h, cookie, 6)
	if code := jsonAuth(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/ki-auslesen", nil, "").Code; code != http.StatusAccepted {
		t.Fatal(code)
	}
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := jsonAuth(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/ki", nil, "")
	if !strings.Contains(got.Body.String(), `"leer"`) {
		t.Fatalf("%s", got.Body)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	t.Cleanup(slow.Close)
	app = newKIApp(t, slow.URL+"/v1", 40*time.Millisecond)
	h = app.Handler()
	cookie = loginCookie(t, h)
	allowKI(t, h, cookie)
	id = readyBeleg(t, app, h, cookie, 7)
	_ = jsonAuth(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/ki-auslesen", nil, "")
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	got = jsonAuth(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/ki", nil, "")
	if !strings.Contains(got.Body.String(), "nicht_erreichbar") {
		t.Fatalf("%s", got.Body)
	}
}

func newKIApp(t *testing.T, base string, timeout time.Duration) *App {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.BelegFormat = "webp"
		cfg.BelegWebPQuality = 40
		cfg.BelegJPEGQuality = 70
		cfg.AI.Enabled = true
		cfg.AI.BaseURL = base
		cfg.AI.APIKey = "sk-test-secret-value"
		cfg.AI.Model = "fake"
		cfg.AI.Timeout = timeout
		cfg.AI.MaxConcurrency = 2
		cfg.AI.MaxImagePx = 2480
		cfg.AI.ResponseFormat = "json_object"
		cfg.RateAI = config.Rate{Count: 20, Per: time.Minute}
	})
	seedUser(t, app, "ada", "correct-horse-1", true)
	return app
}

func allowKI(t *testing.T, h http.Handler, cookie *http.Cookie) {
	t.Helper()
	me := jsonAuth(t, h, cookie, http.MethodGet, "/api/v1/me", nil, "")
	if me.Code != http.StatusOK {
		t.Fatal(me.Body)
	}
	res := jsonAuth(t, h, cookie, http.MethodPatch, "/api/v1/me", []byte(`{"ki_erlaubt":true}`), me.Header().Get("ETag"))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"ki_erlaubt":true`) {
		t.Fatalf("opt-in %d %s", res.Code, res.Body)
	}
}

func readyBeleg(t *testing.T, app *App, h http.Handler, cookie *http.Cookie, mark int) string {
	t.Helper()
	res := uploadBeleg(t, h, cookie, jpegBytesMarked(t, mark), "bon.jpg", "image/jpeg", false)
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload %d %s", res.Code, res.Body)
	}
	var uploaded map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	id, _ := uploaded["id"].(string)
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	return id
}

func jsonAuth(t *testing.T, h http.Handler, cookie *http.Cookie, method, path string, body []byte, etag string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = "192.0.2.40:9"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
