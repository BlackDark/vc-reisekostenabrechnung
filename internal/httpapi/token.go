package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func apiTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func apiTimePtr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}

func toApiToken(t sqlitedb.ApiToken) api.ApiToken {
	return api.ApiToken{
		Id: t.ID, Name: t.Name, Scope: t.Scope,
		CreatedAt: apiTime(t.ErstelltAm), LastUsedAt: apiTimePtr(t.ZuletztBenutztAm), RevokedAt: apiTimePtr(t.WiderrufenAm),
	}
}

// GetMeTokens lists the signed-in Nutzer's API tokens. Hashes and raw values are
// never returned.
func (a *App) GetMeTokens(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListTokens(r.Context(), n.ID)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.ApiToken, 0, len(rows))
	for _, row := range rows {
		items = append(items, toApiToken(row))
	}
	writeJSON(w, http.StatusOK, api.ApiTokenListe{Items: items})
}

// PostMeToken creates an API token. The plaintext token is returned only in this
// response and cannot be recovered later.
func (a *App) PostMeToken(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.ApiTokenInput
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len([]rune(name)) > 64 {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Give the token a name (1 to 64 characters)", "")
		return
	}
	scope := store.TokenScopeUpload
	if body.Scope != nil {
		scope = string(*body.Scope)
	}
	if !store.ValidTokenScope(scope) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Unknown token scope", "")
		return
	}
	plain, hash, err := auth.NewAPIToken()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	tok, err := a.store.CreateToken(r.Context(), n.ID, name, scope, hash, time.Now().UTC().Format(time.RFC3339), a.actor(r, n))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	view := api.ApiTokenMitToken{
		Id: tok.ID, Name: tok.Name, Scope: tok.Scope, CreatedAt: apiTime(tok.ErstelltAm), Token: plain,
	}
	if scope == store.TokenScopeMailIngest && a.cfg.Mail.ForwardDomain != "" {
		addr := "belege+" + plain + "@" + a.cfg.Mail.ForwardDomain
		view.ForwardingAddress = &addr
	}
	writeJSON(w, http.StatusCreated, view)
}

// DeleteMeToken revokes one of the signed-in Nutzer's tokens.
func (a *App) DeleteMeToken(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	done, err := a.store.RevokeToken(r.Context(), n.ID, id, time.Now().UTC().Format(time.RFC3339), a.actor(r, n))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	if !done {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// requireTokenActor resolves the Nutzer from a bearer API token of the given
// scope or from the session cookie. A token authorizes only the endpoints that
// ask for its scope; every other endpoint still requires the session.
func (a *App) requireTokenActor(w http.ResponseWriter, r *http.Request, scope string) (sqlitedb.Nutzer, bool) {
	zero := sqlitedb.Nutzer{}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		plain := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		tok, err := a.store.FindActiveToken(r.Context(), auth.HashBearer(plain))
		if err != nil || tok.Scope != scope {
			writeProblem(w, http.StatusUnauthorized, "ungueltiges_token", "Invalid token", "")
			return zero, false
		}
		n, err := a.store.GetNutzer(r.Context(), tok.NutzerID)
		if err != nil || !n.Aktiv {
			writeProblem(w, http.StatusUnauthorized, "ungueltiges_token", "Invalid token", "")
			return zero, false
		}
		_ = a.store.TouchToken(r.Context(), tok.ID, nowRFC3339())
		return n, true
	}
	return a.requireUser(w, r)
}
