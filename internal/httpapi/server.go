package httpapi

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/webui"
)

const sessionNutzerKey = "nutzer_id"

// App is the HTTP application.
type App struct {
	cfg       config.Config
	store     *store.Store
	passwords *auth.Passwords
	sessions  *scs.SessionManager
	log       *slog.Logger
	limits    limiter
	oidcMu    sync.Mutex
	oidcKey   []byte
	oidcP     *oidc.Provider
	Version   string
	Commit    string
}

// New wires sessions, CSRF and the embedded SPA.
func New(cfg config.Config, st *store.Store, pw *auth.Passwords, log *slog.Logger) (*App, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	sm := scs.New()
	sm.Store = store.NewSessionStore(st)
	sm.Lifetime = cfg.SessionLifetime
	sm.IdleTimeout = cfg.SessionIdle
	sm.Cookie.Name = cfg.CookieName("rk_session")
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = cfg.CookieSecure
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Path = "/"
	sm.Cookie.Persist = true
	if log == nil {
		log = slog.Default()
	}
	return &App{cfg: cfg, store: st, passwords: pw, sessions: sm, log: log, oidcKey: key}, nil
}

// Handler is the root HTTP handler.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	api.HandlerWithOptions(a, api.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, http.StatusBadRequest, "ungueltige_anfrage", "Bad request", err.Error())
		},
	})
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("GET /readyz", a.readyz)
	mux.HandleFunc("GET /version", a.version)
	dist, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		panic(err)
	}
	spa := spaHandler(dist)
	mux.Handle("GET /{$}", spa)
	mux.Handle("GET /{path...}", spa)

	csrf := http.NewCrossOriginProtection()
	if origin := a.cfg.OriginURL(); origin != "" {
		if err := csrf.AddTrustedOrigin(origin); err != nil {
			a.log.Error("csrf origin", "err", err)
		}
	}
	var h http.Handler = mux
	h = a.withSession(h)
	h = csrf.Handler(h)
	h = a.withSecurity(h)
	return h
}

func (a *App) withSession(next http.Handler) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meta := &store.SessionMeta{UserAgent: r.UserAgent(), IP: ClientIP(r, a.cfg.TrustedProxies)}
		ctx := store.WithSessionMeta(r.Context(), meta)
		r = r.WithContext(ctx)
		a.stripUntrustedHeaders(r)
		if n, ok := a.headerPrincipal(r); ok {
			ctx = withPrincipal(r.Context(), n)
			r = r.WithContext(ctx)
		}
		if !a.allowAPI(r) {
			w.Header().Set("Retry-After", "60")
			writeProblem(w, http.StatusTooManyRequests, "rate_limit", "Too many requests", "")
			return
		}
		next.ServeHTTP(w, r)
	})
	return a.sessions.LoadAndSave(inner)
}

func (a *App) allowAPI(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/oidc/start", "/api/v1/auth/setup":
		return true
	}
	key := "api:" + ClientIP(r, a.cfg.TrustedProxies)
	if id := a.currentID(r); id != "" {
		key = "api:" + id
	}
	return a.limits.allow(key, a.cfg.RateAPI.Count, a.cfg.RateAPI.Per)
}

func (a *App) withSecurity(next http.Handler) http.Handler {
	csp := "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; style-src-attr 'unsafe-inline'; img-src 'self' blob: data:; connect-src 'self'; worker-src 'self' blob:; font-src 'self'; manifest-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"
	if a.cfg.OIDC.Enabled && a.cfg.OIDC.IssuerURL != "" {
		if origin := issuerOrigin(a.cfg.OIDC.IssuerURL); origin != "" {
			csp += " " + origin
		}
	}
	https := strings.HasPrefix(a.cfg.AppBaseURL, "https://")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(self), geolocation=(), microphone=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if https {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func issuerOrigin(raw string) string {
	if i := strings.Index(raw, "://"); i > 0 {
		rest := raw[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			rest = rest[:slash]
		}
		if rest != "" {
			return raw[:i+3] + rest
		}
	}
	return ""
}

func (a *App) stripUntrustedHeaders(r *http.Request) {
	if !a.cfg.Header.Enabled {
		return
	}
	if configTrusted(r, a.cfg) {
		return
	}
	names := []string{a.cfg.Header.UserHeader, a.cfg.Header.EmailHeader, a.cfg.Header.NameHeader, a.cfg.Header.GroupsHeader}
	present := false
	for _, name := range names {
		if r.Header.Get(name) != "" {
			present = true
			r.Header.Del(name)
		}
	}
	if present {
		a.log.Warn("ignoring auth headers from untrusted peer", "peer", r.RemoteAddr)
	}
}

func configTrusted(r *http.Request, cfg config.Config) bool {
	return config.IPTrusted(cfg.TrustedProxies, peerIP(r.RemoteAddr))
}

type ctxKey int

const principalKey ctxKey = 1

func withPrincipal(ctx context.Context, n sqlitedb.Nutzer) context.Context {
	return context.WithValue(ctx, principalKey, n)
}

func principalFrom(ctx context.Context) (sqlitedb.Nutzer, bool) {
	n, ok := ctx.Value(principalKey).(sqlitedb.Nutzer)
	return n, ok
}

func (a *App) headerPrincipal(r *http.Request) (sqlitedb.Nutzer, bool) {
	if !a.cfg.Header.Enabled || !configTrusted(r, a.cfg) {
		return sqlitedb.Nutzer{}, false
	}
	user := strings.TrimSpace(r.Header.Get(a.cfg.Header.UserHeader))
	if user == "" {
		return sqlitedb.Nutzer{}, false
	}
	groups, _ := auth.ParseGroups([]byte(r.Header.Get(a.cfg.Header.GroupsHeader)))
	n, err := a.resolveExternal(r.Context(), externalLogin{
		Art:          "header",
		Issuer:       "header",
		Subject:      user,
		Username:     user,
		Name:         r.Header.Get(a.cfg.Header.NameHeader),
		Email:        r.Header.Get(a.cfg.Header.EmailHeader),
		Groups:       groups,
		AdminGroup:   a.cfg.Header.AdminGroup,
		AllowedGroup: a.cfg.Header.AllowedGroup,
		Sprache:      a.cfg.DefaultLocale,
		IP:           ClientIP(r, a.cfg.TrustedProxies),
	})
	if err != nil {
		a.log.Warn("header auth rejected", "err", err)
		return sqlitedb.Nutzer{}, false
	}
	return n, true
}

func (a *App) current(r *http.Request) (sqlitedb.Nutzer, bool) {
	if n, ok := principalFrom(r.Context()); ok && n.Aktiv {
		return n, true
	}
	id := a.sessions.GetString(r.Context(), sessionNutzerKey)
	if id == "" {
		return sqlitedb.Nutzer{}, false
	}
	n, err := a.store.GetNutzer(r.Context(), id)
	if err != nil || !n.Aktiv {
		return sqlitedb.Nutzer{}, false
	}
	return n, true
}

func (a *App) currentID(r *http.Request) string {
	if n, ok := a.current(r); ok {
		return n.ID
	}
	return ""
}

func (a *App) requireUser(w http.ResponseWriter, r *http.Request) (sqlitedb.Nutzer, bool) {
	n, ok := a.current(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "nicht_angemeldet", "Not signed in", "")
		return sqlitedb.Nutzer{}, false
	}
	return n, true
}

func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request) (sqlitedb.Nutzer, bool) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return sqlitedb.Nutzer{}, false
	}
	if !n.IstAdminLokal && !n.AdminUeberGruppe {
		writeProblem(w, http.StatusForbidden, "kein_admin", "Admin only", "")
		return sqlitedb.Nutzer{}, false
	}
	return n, true
}

func (a *App) actor(r *http.Request, n sqlitedb.Nutzer) store.Actor {
	id := n.ID
	return store.Actor{NutzerID: &id, Art: "nutzer", IP: ClientIP(r, a.cfg.TrustedProxies)}
}

func (a *App) beginSession(r *http.Request, n sqlitedb.Nutzer) error {
	if meta := store.SessionMetaFrom(r.Context()); meta != nil {
		meta.NutzerID = n.ID
	}
	a.sessions.Put(r.Context(), sessionNutzerKey, n.ID)
	return a.sessions.RenewToken(r.Context())
}

func (a *App) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (a *App) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": a.Version, "commit": a.Commit})
}

func (a *App) readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := a.store.Ping(ctx); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "db", "Database unavailable", err.Error())
		return
	}
	v, err := a.store.DBVersion(ctx)
	if err != nil || v < 1 {
		writeProblem(w, http.StatusServiceUnavailable, "migration", "Migrations not applied", "")
		return
	}
	if err := writable(a.cfg.DataDir); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "data_dir", "DATA_DIR is not writable", err.Error())
		return
	}
	if err := writable(os.TempDir()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "tmp", "/tmp is not writable", err.Error())
		return
	}
	if err := typstOK(a.cfg.TypstPath); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "typst", "typst is not executable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writable(dir string) error {
	f, err := os.CreateTemp(dir, ".ready-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func typstOK(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() || st.Mode()&0o111 == 0 {
		return fmt.Errorf("%s is not executable", path)
	}
	return nil
}
