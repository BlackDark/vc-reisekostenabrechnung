package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

func (a *App) GetAuthConfig(w http.ResponseWriter, r *http.Request) {
	n, err := a.store.CountNutzer(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	label := a.cfg.OIDC.ButtonLabel
	auto := a.cfg.OIDC.AutoRedirect
	var logout *string
	if a.cfg.Header.LogoutURL != "" {
		logout = &a.cfg.Header.LogoutURL
	}
	writeJSON(w, http.StatusOK, api.AuthConfig{
		DefaultLocale:     api.AuthConfigDefaultLocale(a.cfg.DefaultLocale),
		Header:            a.cfg.Header.Enabled,
		HeaderLogoutUrl:   logout,
		Oidc:              a.cfg.OIDC.Enabled,
		OidcAutoRedirect:  &auto,
		OidcButtonLabel:   &label,
		Passwort:          a.cfg.AuthPasswordEnabled,
		SetupErforderlich: n == 0,
		AiAktiviert:       a.cfg.AI.Enabled,
		AiBasisUrl:        a.cfg.AI.PublicBaseURL(),
	})
}

func (a *App) PostAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.AuthPasswordEnabled {
		writeProblem(w, http.StatusNotFound, "passwort_deaktiviert", "Password login is disabled", "")
		return
	}
	var body api.LoginRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ip := ClientIP(r, a.cfg.TrustedProxies)
	name := auth.NormName(body.Benutzername)
	if !a.limits.allow("login:"+ip, a.cfg.RateLogin.Count, a.cfg.RateLogin.Per) ||
		!a.limits.allow("loginacct:"+name+"|"+ip, a.cfg.RateLoginAccount.Count, a.cfg.RateLoginAccount.Per) {
		w.Header().Set("Retry-After", "60")
		writeProblem(w, http.StatusTooManyRequests, "rate_limit", "Too many login attempts", "")
		return
	}
	n, err := a.store.GetByBenutzername(r.Context(), name)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	hash := ""
	if n.PasswortHash != nil {
		hash = *n.PasswortHash
	}
	var ok, rehash bool
	if hash == "" {
		a.passwords.VerifyDummy(body.Passwort)
	} else {
		ok, rehash = a.passwords.Verify(hash, body.Passwort)
	}
	if !ok || !n.Aktiv {
		if n.ID != "" {
			_ = a.store.TouchLogin(r.Context(), n.ID, store.Actor{NutzerID: &n.ID, Art: "nutzer", IP: ip}, false)
		} else {
			_ = a.store.RecordFailure(r.Context(), name, ip)
		}
		writeProblem(w, http.StatusUnauthorized, "ungueltige_anmeldung", "Invalid username or password", "")
		return
	}
	if rehash {
		next, err := auth.Hash(body.Passwort, a.passwords.Params)
		if err == nil {
			if updated, err := a.store.UpdatePasswort(r.Context(), n.ID, n.Version, next, a.actor(r, n)); err == nil {
				n = updated
			}
		}
	}
	if err := a.beginSession(r, n); err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	_ = a.store.TouchLogin(r.Context(), n.ID, a.actor(r, n), true)
	writeNutzer(w, http.StatusOK, n)
}

func (a *App) PostAuthLogout(w http.ResponseWriter, r *http.Request) {
	if n, ok := a.current(r); ok {
		_ = a.store.RecordAudit(r.Context(), a.actor(r, n), "abmeldung", "nutzer", n.ID)
	}
	_ = a.sessions.Destroy(r.Context())
	w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetAuthMe(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	writeNutzer(w, http.StatusOK, n)
}

func (a *App) GetMe(w http.ResponseWriter, r *http.Request) {
	a.GetAuthMe(w, r)
}

func (a *App) PatchMe(w http.ResponseWriter, r *http.Request, params api.PatchMeParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	if version != n.Version {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.ProfilPatch
	if !decodeJSON(w, r, &body) {
		return
	}
	name := n.Anzeigename
	if body.Anzeigename != nil {
		name = strings.TrimSpace(*body.Anzeigename)
	}
	if name == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Anzeigename is required", "")
		return
	}
	sprache := n.Sprache
	if body.Sprache != nil {
		sprache = string(*body.Sprache)
	}
	if sprache != "de" && sprache != "en" {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Sprache must be de or en", "")
		return
	}
	pn := n.Personalnummer
	if body.Personalnummer != nil {
		v := strings.TrimSpace(*body.Personalnummer)
		if v == "" {
			pn = nil
		} else {
			pn = &v
		}
	}
	ki := n.KiErlaubt
	if body.KiErlaubt != nil {
		ki = *body.KiErlaubt
	}
	updated, err := a.store.UpdateProfil(r.Context(), n.ID, version, name, sprache, pn, ki, a.actor(r, n))
	if errors.Is(err, store.ErrConflict) {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeNutzer(w, http.StatusOK, updated)
}

func (a *App) PutAuthPasswort(w http.ResponseWriter, r *http.Request, params api.PutAuthPasswortParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	if version != n.Version {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.PasswortRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if !auth.PasswordOK(body.NeuesPasswort) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Password must be 12 to 256 characters", "")
		return
	}
	if n.PasswortHash != nil {
		if body.AktuellesPasswort == nil {
			writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Current password is required", "")
			return
		}
		ok, _ := a.passwords.Verify(*n.PasswortHash, *body.AktuellesPasswort)
		if !ok {
			writeProblem(w, http.StatusUnauthorized, "ungueltige_anmeldung", "Invalid username or password", "")
			return
		}
	}
	hash, err := auth.Hash(body.NeuesPasswort, a.passwords.Params)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	updated, err := a.store.UpdatePasswort(r.Context(), n.ID, version, hash, a.actor(r, n))
	if errors.Is(err, store.ErrConflict) {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	_ = a.sessions.RenewToken(r.Context())
	writeNutzer(w, http.StatusOK, updated)
}

func (a *App) PostAuthSetup(w http.ResponseWriter, r *http.Request) {
	n, err := a.store.CountNutzer(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	if n > 0 {
		writeProblem(w, http.StatusConflict, "setup_geschlossen", "Setup is closed", "")
		return
	}
	var body api.SetupRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	want, err := readSetupToken(a.cfg.DataDir)
	if err != nil || subtle.ConstantTimeCompare([]byte(body.Token), []byte(want)) != 1 {
		writeProblem(w, http.StatusUnauthorized, "ungueltiges_token", "Invalid setup token", "")
		return
	}
	name := auth.NormName(body.Benutzername)
	if !auth.BenutzernameOK(name) || strings.TrimSpace(body.Anzeigename) == "" || !auth.PasswordOK(body.Passwort) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check Benutzername, Anzeigename and password", "")
		return
	}
	hash, err := auth.Hash(body.Passwort, a.passwords.Params)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var email *string
	if body.Email != nil && strings.TrimSpace(*body.Email) != "" {
		e := auth.NormEmail(*body.Email)
		email = &e
	}
	created, err := a.store.CreateNutzer(r.Context(), store.NewNutzer{
		Anzeigename:   strings.TrimSpace(body.Anzeigename),
		Email:         email,
		Benutzername:  name,
		PasswortHash:  &hash,
		IstAdminLokal: true,
		Sprache:       a.cfg.DefaultLocale,
		Aktiv:         true,
	}, store.Actor{Art: "system", IP: ClientIP(r, a.cfg.TrustedProxies)}, "nutzer.angelegt")
	if err != nil {
		writeProblem(w, http.StatusConflict, "konflikt", "Could not create the Admin", "")
		return
	}
	_ = os.Remove(filepath.Join(a.cfg.DataDir, "setup.token"))
	if err := a.beginSession(r, created); err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeNutzer(w, http.StatusCreated, created)
}

func (a *App) GetMeSessions(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListSessions(r.Context(), n.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	token := a.sessions.Token(r.Context())
	out := make([]api.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, api.Session{
			Id:            row.OeffentlichID,
			Aktuell:       token != "" && row.Token == token,
			Ip:            row.Ip,
			UserAgent:     row.UserAgent,
			LetzteNutzung: row.LetzteNutzung,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (a *App) DeleteMeSession(w http.ResponseWriter, r *http.Request, id string) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListSessions(r.Context(), n.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var current bool
	found := false
	token := a.sessions.Token(r.Context())
	for _, row := range rows {
		if row.OeffentlichID == id {
			found = true
			current = row.Token == token
			break
		}
	}
	if !found {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Session not found", "")
		return
	}
	if err := a.store.DeleteSessionFor(r.Context(), n.ID, id); err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	if current {
		_ = a.sessions.Destroy(r.Context())
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetAdminNutzer(w http.ResponseWriter, r *http.Request, params api.GetAdminNutzerParams) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	limit := int64(50)
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 200 {
		limit = int64(*params.Limit)
	}
	rows, err := a.store.ListNutzer(r.Context(), params.Cursor, limit+1)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var next *string
	if int64(len(rows)) > limit {
		rows = rows[:limit]
		id := rows[len(rows)-1].ID
		next = &id
	}
	items := make([]api.Nutzer, 0, len(rows))
	for _, row := range rows {
		items = append(items, toNutzer(row))
	}
	writeJSON(w, http.StatusOK, api.NutzerListe{Items: items, NextCursor: next})
}

func (a *App) PostAdminNutzer(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	var body api.NutzerCreate
	if !decodeJSON(w, r, &body) {
		return
	}
	name := auth.NormName(body.Benutzername)
	if !auth.BenutzernameOK(name) || strings.TrimSpace(body.Anzeigename) == "" || !auth.PasswordOK(body.Passwort) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check Benutzername, Anzeigename and password", "")
		return
	}
	hash, err := auth.Hash(body.Passwort, a.passwords.Params)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var email *string
	if body.Email != nil && strings.TrimSpace(*body.Email) != "" {
		e := auth.NormEmail(*body.Email)
		email = &e
	}
	sprache := a.cfg.DefaultLocale
	if body.Sprache != nil && (*body.Sprache == "de" || *body.Sprache == "en") {
		sprache = string(*body.Sprache)
	}
	istAdmin := false
	if body.IstAdmin != nil {
		istAdmin = *body.IstAdmin
	}
	created, err := a.store.CreateNutzer(r.Context(), store.NewNutzer{
		Anzeigename:   strings.TrimSpace(body.Anzeigename),
		Email:         email,
		Benutzername:  name,
		PasswortHash:  &hash,
		IstAdminLokal: istAdmin,
		Sprache:       sprache,
		Aktiv:         true,
	}, a.actor(r, admin), "nutzer.angelegt")
	if err != nil {
		writeProblem(w, http.StatusConflict, "konflikt", "Could not create the Nutzer", "")
		return
	}
	writeNutzer(w, http.StatusCreated, created)
}

func (a *App) PatchAdminNutzer(w http.ResponseWriter, r *http.Request, id string, params api.PatchAdminNutzerParams) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	target, err := a.store.GetNutzer(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Nutzer not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	if version != target.Version {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.NutzerAdminPatch
	if !decodeJSON(w, r, &body) {
		return
	}
	aktiv := target.Aktiv
	if body.Aktiv != nil {
		aktiv = *body.Aktiv
	}
	istAdmin := target.IstAdminLokal
	if body.IstAdminLokal != nil {
		istAdmin = *body.IstAdminLokal
	}
	var hash *string
	if body.Passwort != nil {
		if !auth.PasswordOK(*body.Passwort) {
			writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Password must be 12 to 256 characters", "")
			return
		}
		h, err := auth.Hash(*body.Passwort, a.passwords.Params)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
			return
		}
		hash = &h
	}
	updated, err := a.store.UpdateAdmin(r.Context(), id, version, aktiv, istAdmin, hash, a.actor(r, admin))
	if errors.Is(err, store.ErrLastAdmin) {
		writeProblem(w, http.StatusConflict, "letzter_admin", "The last Admin cannot be removed", "")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeNutzer(w, http.StatusOK, updated)
}

func (a *App) GetAuthOidcStart(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.OIDC.Enabled {
		writeProblem(w, http.StatusNotFound, "oidc_deaktiviert", "OIDC is disabled", "")
		return
	}
	ip := ClientIP(r, a.cfg.TrustedProxies)
	if !a.limits.allow("oidc:"+ip, 20, time.Minute) {
		w.Header().Set("Retry-After", "60")
		writeProblem(w, http.StatusTooManyRequests, "rate_limit", "Too many requests", "")
		return
	}
	p, err := a.provider(r.Context())
	if err != nil {
		a.log.Error("oidc discovery", "err", err)
		writeProblem(w, http.StatusBadGateway, "oidc", "Identity provider is unavailable", "")
		return
	}
	state, err := randomToken(24)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	nonce, err := randomToken(24)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	verifier := oauth2.GenerateVerifier()
	pending := oidcPending{State: state, Nonce: nonce, Verifier: verifier, Exp: time.Now().Add(10 * time.Minute).Unix()}
	raw, err := signOIDC(a.oidcKey, pending)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	a.setOIDCCookie(w, raw)
	loc := a.oauth(p).AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce))
	http.Redirect(w, r, loc, http.StatusFound)
}

func (a *App) GetAuthOidcCallback(w http.ResponseWriter, r *http.Request, params api.GetAuthOidcCallbackParams) {
	fail := func() {
		a.clearOIDCCookie(w)
		http.Redirect(w, r, "/login?error=oidc", http.StatusFound)
	}
	if !a.cfg.OIDC.Enabled || params.Code == nil || params.State == nil {
		fail()
		return
	}
	c, err := r.Cookie(a.cfg.CookieName("rk_oidc"))
	if err != nil {
		fail()
		return
	}
	pending, err := openOIDC(a.oidcKey, c.Value)
	if err != nil || subtle.ConstantTimeCompare([]byte(pending.State), []byte(*params.State)) != 1 {
		fail()
		return
	}
	p, err := a.provider(r.Context())
	if err != nil {
		fail()
		return
	}
	tok, err := a.oauth(p).Exchange(r.Context(), *params.Code, oauth2.VerifierOption(pending.Verifier))
	if err != nil {
		a.log.Warn("oidc exchange", "err", err)
		fail()
		return
	}
	rawID, _ := tok.Extra("id_token").(string)
	idTok, err := p.Verifier(&oidc.Config{ClientID: a.cfg.OIDC.ClientID}).Verify(r.Context(), rawID)
	if err != nil {
		a.log.Warn("oidc verify", "err", err)
		fail()
		return
	}
	claims, groups, err := a.oidcClaims(r, p, tok, idTok)
	if err != nil || claims.Nonce != pending.Nonce {
		fail()
		return
	}
	n, err := a.resolveExternal(r.Context(), externalLogin{
		Art:           "oidc",
		Issuer:        a.cfg.OIDC.IssuerURL,
		Subject:       claims.Sub,
		Username:      claims.PreferredUsername,
		Name:          claims.Name,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Groups:        groups,
		AdminGroup:    a.cfg.OIDC.AdminGroup,
		AllowedGroup:  a.cfg.OIDC.AllowedGroup,
		AllowEmail:    a.cfg.OIDC.AllowEmailLinking,
		Sprache:       a.cfg.DefaultLocale,
		IP:            ClientIP(r, a.cfg.TrustedProxies),
	})
	if err != nil {
		a.log.Warn("oidc provision", "err", err)
		fail()
		return
	}
	a.clearOIDCCookie(w)
	if err := a.beginSession(r, n); err != nil {
		fail()
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

type oidcClaims struct {
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Nonce             string `json:"nonce"`
}

func (a *App) oidcClaims(r *http.Request, p *oidc.Provider, tok *oauth2.Token, idTok *oidc.IDToken) (oidcClaims, []string, error) {
	var claims oidcClaims
	if err := idTok.Claims(&claims); err != nil {
		return claims, nil, err
	}
	var raw map[string]any
	if err := idTok.Claims(&raw); err != nil {
		return claims, nil, err
	}
	groups := groupsFromClaim(raw[a.cfg.OIDC.GroupsClaim])
	if _, ok := raw[a.cfg.OIDC.GroupsClaim]; !ok {
		if info, err := p.UserInfo(r.Context(), oauth2.StaticTokenSource(tok)); err == nil {
			var extra map[string]any
			if err := info.Claims(&extra); err == nil {
				groups = groupsFromClaim(extra[a.cfg.OIDC.GroupsClaim])
				if claims.Email == "" {
					if s, ok := extra["email"].(string); ok {
						claims.Email = s
					}
				}
			}
		}
	}
	if !claims.EmailVerified {
		if v, ok := raw["email_verified"].(string); ok && (v == "true" || v == "1") {
			claims.EmailVerified = true
		}
	}
	return claims, groups, nil
}

func groupsFromClaim(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		g, _ := auth.ParseGroups([]byte(t))
		return g
	default:
		return nil
	}
}

func (a *App) provider(ctx context.Context) (*oidc.Provider, error) {
	a.oidcMu.Lock()
	defer a.oidcMu.Unlock()
	if a.oidcP != nil {
		return a.oidcP, nil
	}
	if strings.HasPrefix(a.cfg.OIDC.IssuerURL, "http://") {
		ctx = oidc.InsecureIssuerURLContext(ctx, a.cfg.OIDC.IssuerURL)
	}
	p, err := oidc.NewProvider(ctx, a.cfg.OIDC.IssuerURL)
	if err != nil {
		return nil, err
	}
	a.oidcP = p
	return p, nil
}

func (a *App) oauth(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.cfg.OIDC.ClientID,
		ClientSecret: a.cfg.OIDC.ClientSecret,
		RedirectURL:  a.cfg.AppBaseURL + "/api/v1/auth/oidc/callback",
		Endpoint:     p.Endpoint(),
		Scopes:       a.cfg.OIDC.Scopes,
	}
}
