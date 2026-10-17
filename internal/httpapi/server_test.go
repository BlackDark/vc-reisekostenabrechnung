package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

func newTestApp(t *testing.T, tweak func(*config.Config)) *App {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		AppBaseURL:          "http://127.0.0.1:9",
		ListenAddr:          ":9",
		DataDir:             dir,
		DBPath:              filepath.Join(dir, "t.db"),
		LogLevel:            "error",
		LogFormat:           "text",
		DefaultLocale:       "de",
		CookieSecure:        false,
		SessionIdle:         time.Hour,
		SessionLifetime:     24 * time.Hour,
		AuthPasswordEnabled: true,
		Argon2MemoryKiB:     8,
		Argon2Time:          1,
		Argon2Threads:       1,
		StorageBackend:      "local",
		TypstPath:           os.Args[0],
		RateLogin:           config.Rate{Count: 5, Per: time.Minute},
		RateLoginAccount:    config.Rate{Count: 20, Per: time.Hour},
		RateAPI:             config.Rate{Count: 300, Per: time.Minute},
		Header: config.Header{
			UserHeader:   "Remote-User",
			EmailHeader:  "Remote-Email",
			NameHeader:   "Remote-Name",
			GroupsHeader: "Remote-Groups",
		},
	}
	if tweak != nil {
		tweak(&cfg)
	}
	st, err := store.Open(t.Context(), cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	pw, err := auth.NewPasswords(auth.Params{Memory: cfg.Argon2MemoryKiB, Time: cfg.Argon2Time, Threads: cfg.Argon2Threads})
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(cfg, st, pw, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.passwords = pw
	return app
}

func seedUser(t *testing.T, app *App, name, password string, admin bool) {
	t.Helper()
	hash, err := auth.Hash(password, app.passwords.Params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.store.CreateNutzer(t.Context(), store.NewNutzer{
		Anzeigename: name, Benutzername: name, PasswortHash: &hash,
		IstAdminLokal: admin, Sprache: "de", Aktiv: true,
	}, store.Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
}

func doJSON(t *testing.T, h http.Handler, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = "192.0.2.10:1234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPasswordLoginAndCSRF(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()

	bad := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"wrong-password"}`, nil)
	if bad.Code != http.StatusUnauthorized || !strings.Contains(bad.Body.String(), "ungueltige_anmeldung") {
		t.Fatalf("bad login %d %s", bad.Code, bad.Body)
	}

	ok := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"ist_admin":true`) {
		t.Fatalf("login %d %s", ok.Code, ok.Body)
	}
	cookie := ok.Result().Cookies()
	if len(cookie) == 0 || cookie[0].Name != "rk_session" || !cookie[0].HttpOnly {
		t.Fatalf("cookie %+v", cookie)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	me.RemoteAddr = "192.0.2.10:1234"
	me.AddCookie(cookie[0])
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, me)
	if rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body)
	}

	csrf := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, map[string]string{
		"Origin":         "https://evil.example",
		"Sec-Fetch-Site": "cross-site",
	})
	if csrf.Code != http.StatusForbidden {
		t.Fatalf("csrf %d %s", csrf.Code, csrf.Body)
	}
}

func TestLoginRateLimit(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.RateLogin = config.Rate{Count: 2, Per: time.Minute}
	})
	h := app.Handler()
	var last int
	for i := 0; i < 3; i++ {
		res := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"wrong-password"}`, nil)
		last = res.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("status %d", last)
	}
}

func TestLoginAccountRateLimit(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.RateLogin = config.Rate{Count: 100, Per: time.Minute}
		cfg.RateLoginAccount = config.Rate{Count: 2, Per: time.Hour}
	})
	h := app.Handler()
	var last int
	for i := 0; i < 3; i++ {
		res := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"wrong-password"}`, nil)
		last = res.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("status %d", last)
	}
}

func TestHeaderAuthTrustedOnly(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.Header.Enabled = true
		cfg.Header.AdminGroup = "admins"
		cfg.Header.AllowedGroup = "users"
		cfg.TrustedProxies = []*net.IPNet{n}
	})
	h := app.Handler()

	trusted := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	trusted.RemoteAddr = "10.1.2.3:99"
	trusted.Header.Set("Remote-User", "Ada Lovelace")
	trusted.Header.Set("Remote-Groups", "admins")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, trusted)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ist_admin":true`) {
		t.Fatalf("trusted %d %s", rec.Code, rec.Body)
	}

	untrusted := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	untrusted.RemoteAddr = "192.0.2.8:99"
	untrusted.Header.Set("Remote-User", "Ada Lovelace")
	untrusted.Header.Set("Remote-Groups", "admins")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, untrusted)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("untrusted %d %s", rec.Code, rec.Body)
	}
}

// dropAuditTable removes the table every failed and successful login writes to,
// which is how these tests reach the audit writes' failure path without a store
// double.
func dropAuditTable(t *testing.T, app *App) {
	t.Helper()
	db, err := sql.Open("sqlite", app.cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("DROP TABLE audit_ereignis"); err != nil {
		t.Fatal(err)
	}
}

// A login writes an audit row. Where that write fails, the request must not be
// answered as if the attempt were on record, and a correct password must not hand
// out a session either.
func TestLoginFailsWhenTheAttemptCannotBeRecorded(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()

	ok := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if ok.Code != http.StatusOK {
		t.Fatalf("login %d %s", ok.Code, ok.Body)
	}
	dropAuditTable(t, app)

	wrong := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"wrong-password"}`, nil)
	if wrong.Code != http.StatusInternalServerError {
		t.Fatalf("refused attempt %d %s, want 500", wrong.Code, wrong.Body)
	}
	right := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if right.Code != http.StatusInternalServerError {
		t.Fatalf("accepted attempt %d %s, want 500", right.Code, right.Body)
	}
}

// The same for a header-authenticated request: the login cannot be recorded, so
// the request is refused rather than served under a principal that was never
// written down.
func TestHeaderAuthRefusesWhenTheLoginCannotBeRecorded(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.Header.Enabled = true
		cfg.Header.AdminGroup = "admins"
		cfg.Header.AllowedGroup = "users"
		cfg.TrustedProxies = []*net.IPNet{n}
	})
	h := app.Handler()
	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.RemoteAddr = "10.1.2.3:99"
		req.Header.Set("Remote-User", "Ada Lovelace")
		req.Header.Set("Remote-Groups", "admins")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := call(); rec.Code != http.StatusOK {
		t.Fatalf("first request %d %s", rec.Code, rec.Body)
	}
	dropAuditTable(t, app)
	if rec := call(); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unrecorded login served with %d %s, want 401", rec.Code, rec.Body)
	}
}

func TestSetupToken(t *testing.T) {
	app := newTestApp(t, nil)
	if err := Bootstrap(t.Context(), app.store, app.cfg, app.passwords, nil); err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(filepath.Join(app.cfg.DataDir, "setup.token"))
	if err != nil || len(token) < 8 {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"token": string(token), "benutzername": "ada", "anzeigename": "Ada", "passwort": "correct-horse-1",
	})
	res := doJSON(t, app.Handler(), http.MethodPost, "/api/v1/auth/setup", string(body), nil)
	if res.Code != http.StatusCreated {
		t.Fatalf("setup %d %s", res.Code, res.Body)
	}
	again := doJSON(t, app.Handler(), http.MethodPost, "/api/v1/auth/setup", string(body), nil)
	if again.Code != http.StatusConflict {
		t.Fatalf("again %d", again.Code)
	}
}

func TestLastAdminAndReady(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	login := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatal(login.Body)
	}
	var n struct {
		Version int64  `json:"version"`
		ID      string `json:"id"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &n); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/nutzer/"+n.ID, strings.NewReader(`{"aktiv":false}`))
	req.RemoteAddr = "192.0.2.10:1"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", login.Header().Get("ETag"))
	req.AddCookie(login.Result().Cookies()[0])
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "letzter_admin") {
		t.Fatalf("last admin %d %s", rec.Code, rec.Body)
	}

	ready := doJSON(t, h, http.MethodGet, "/readyz", "", nil)
	if ready.Code != http.StatusOK {
		t.Fatalf("ready %d %s", ready.Code, ready.Body)
	}
	page := doJSON(t, h, http.MethodGet, "/login", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `name="rk-app"`) {
		t.Fatalf("spa %d %s", page.Code, page.Body)
	}
}

func TestClientIP(t *testing.T) {
	_, trusted, _ := net.ParseCIDR("10.0.0.0/8")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.1.1:40"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.9.9.9")
	if got := ClientIP(req, []*net.IPNet{trusted}); got != "203.0.113.9" {
		t.Fatal(got)
	}
	req.RemoteAddr = "192.0.2.4:40"
	if got := ClientIP(req, []*net.IPNet{trusted}); got != "192.0.2.4" {
		t.Fatal(got)
	}
}

func TestMatchVersion(t *testing.T) {
	app := &App{}
	missing := httptest.NewRecorder()
	if _, ok := app.matchVersion(missing, nil); ok || missing.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing %d", missing.Code)
	}
	bad := "bogus"
	invalid := httptest.NewRecorder()
	if _, ok := app.matchVersion(invalid, &bad); ok || invalid.Code != http.StatusPreconditionFailed {
		t.Fatalf("invalid %d", invalid.Code)
	}
	good := "4"
	rec := httptest.NewRecorder()
	version, ok := app.matchVersion(rec, &good)
	if !ok || version != 4 || rec.Code != http.StatusOK {
		t.Fatalf("valid version=%d code=%d ok=%v", version, rec.Code, ok)
	}
}

// A caller that holds an older version of the row must not be able to write it,
// and that answer is the store's own conflict answer rather than a second copy of
// it. The version the caller does hold still goes through.
func TestStaleVersionIsRefused(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()
	login := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if login.Code != http.StatusOK || len(login.Result().Cookies()) == 0 {
		t.Fatalf("login %d %s", login.Code, login.Body)
	}
	var me struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &me); err != nil || me.Version == 0 {
		t.Fatalf("me %s (%v)", login.Body, err)
	}
	cookie := login.Result().Cookies()[0]
	body := `{"sprache":"en"}`
	stale := doJSON(t, h, http.MethodPatch, "/api/v1/me", body,
		ifMatch(cookie, me.Version+1))
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale version %d %s", stale.Code, stale.Body)
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(stale.Body.Bytes(), &problem); err != nil || problem.Code != "version" {
		t.Fatalf("stale version code %q (%v)", problem.Code, err)
	}
	// The version is settled before the input is looked at: a caller that writes
	// from a stale row is refused for that, and not for what the write says.
	both := doJSON(t, h, http.MethodPatch, "/api/v1/me", `{"sprache":"fr"}`, ifMatch(cookie, me.Version+1))
	if both.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale version with an invalid body %d %s", both.Code, both.Body)
	}
	current := doJSON(t, h, http.MethodPatch, "/api/v1/me", body, ifMatch(cookie, me.Version))
	if current.Code != http.StatusOK {
		t.Fatalf("current version %d %s", current.Code, current.Body)
	}
}

func ifMatch(cookie *http.Cookie, version int64) map[string]string {
	return map[string]string{
		"Cookie":   cookie.Name + "=" + cookie.Value,
		"If-Match": strconv.FormatInt(version, 10),
	}
}

func TestMissingIfMatch(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()
	login := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", bytes.NewBufferString(`{"sprache":"en"}`))
	req.RemoteAddr = "192.0.2.10:1"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(login.Result().Cookies()[0])
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

// The readiness probe writes temporary files and queries the database, so an
// unauthenticated caller must not be able to drive that work per request, and the
// cause of a failed probe belongs in the log instead of the response.
func TestReadyzCachesItsVerdictAndHidesTheCause(t *testing.T) {
	app := newTestApp(t, func(c *config.Config) {
		c.TypstPath = filepath.Join(t.TempDir(), "kein-typst")
	})
	h := app.Handler()

	first := doJSON(t, h, http.MethodGet, "/readyz", "", nil)
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("first probe %d %s", first.Code, first.Body)
	}
	var problem struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "typst" {
		t.Fatalf("code %q", problem.Code)
	}
	if problem.Detail != "" {
		t.Fatalf("the response names the cause: %q", problem.Detail)
	}

	// The path is fine now, but the verdict of the first probe still stands.
	app.cfg.TypstPath = os.Args[0]
	second := doJSON(t, h, http.MethodGet, "/readyz", "", nil)
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second probe inside the window %d %s", second.Code, second.Body)
	}
}
