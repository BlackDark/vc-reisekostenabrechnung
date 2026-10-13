package httpapi

import (
	"net/http"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func toMailErlaubt(e sqlitedb.MailErlaubt) api.MailErlaubt {
	created, _ := time.Parse(time.RFC3339, e.ErstelltAm)
	return api.MailErlaubt{Wert: e.Wert, CreatedAt: created}
}

// GetMeMailErlaubt lists the addresses and domains a Nutzer accepts receipts from.
func (a *App) GetMeMailErlaubt(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListMailErlaubt(r.Context(), n.ID)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.MailErlaubt, 0, len(rows))
	for _, e := range rows {
		items = append(items, toMailErlaubt(e))
	}
	writeJSON(w, http.StatusOK, api.MailErlaubtListe{Items: items})
}

// PostMeMailErlaubt adds one address or domain to the allowlist.
func (a *App) PostMeMailErlaubt(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.MailErlaubtInput
	if !decodeJSON(w, r, &body) {
		return
	}
	wert, good := store.NormalizeMailAllow(body.Wert)
	if !good {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Give an email address or @domain", "")
		return
	}
	rows, err := a.store.ListMailErlaubt(r.Context(), n.ID)
	if writeStoreErr(w, err) {
		return
	}
	for _, e := range rows {
		if e.Wert == wert {
			writeProblem(w, http.StatusConflict, "vorhanden", "Already allowed", "")
			return
		}
	}
	now := time.Now().UTC()
	if err := a.store.AddMailErlaubt(r.Context(), n.ID, wert, now.Format(time.RFC3339), a.actor(r, n)); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, api.MailErlaubt{Wert: wert, CreatedAt: now})
}

// DeleteMeMailErlaubt removes one allowlist entry by its value.
func (a *App) DeleteMeMailErlaubt(w http.ResponseWriter, r *http.Request, wert string) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	norm, good := store.NormalizeMailAllow(wert)
	if !good {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	done, err := a.store.RemoveMailErlaubt(r.Context(), n.ID, norm, a.actor(r, n))
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
