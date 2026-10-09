package httpapi

import (
	"net/http"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

func (a *App) GetAdminAufbewahrung(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	a.writeRetention(w, r, store.BerlinDate(time.Now()))
}

func (a *App) PostAdminAufbewahrungLoeschen(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	var body api.AufbewahrungLoeschen
	if !decodeJSON(w, r, &body) {
		return
	}
	in := store.PurgeInput{Grund: body.Grund, Ablaufhemmung: body.AblaufhemmungBestaetigt}
	if body.BelegIds != nil {
		in.BelegIDs = *body.BelegIds
	}
	if body.ExportIds != nil {
		in.ExportIDs = *body.ExportIds
	}
	keys, err := a.store.PurgeRetention(r.Context(), a.actor(r, n), in)
	if writeStoreErr(w, err) {
		return
	}
	for _, key := range keys {
		if err := a.blobs.DeleteForRetention(r.Context(), key, n.ID); err != nil {
			a.log.Error("retention delete", "key", key, "err", err)
			writeProblem(w, http.StatusInternalServerError, "speicher", "Storage delete failed", "")
			return
		}
	}
	a.writeRetention(w, r, store.BerlinDate(time.Now()))
}

func (a *App) writeRetention(w http.ResponseWriter, r *http.Request, today string) {
	items, err := a.store.RetentionReport(r.Context(), today)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	out := make([]api.AufbewahrungPosten, 0, len(items))
	for _, item := range items {
		out = append(out, api.AufbewahrungPosten{
			Art: api.AufbewahrungPostenArt(item.Art), Id: item.ID, NutzerId: item.NutzerID,
			Bezeichnung: item.Bezeichnung, AufbewahrenBis: item.AufbewahrenBis,
			Abgelaufen: item.Abgelaufen, InhaltGeloescht: item.InhaltGeloescht, Sha256: item.SHA256,
		})
	}
	writeJSON(w, http.StatusOK, api.AufbewahrungBericht{
		Heute: today, HinweisCode: "ablaufhemmung", Items: out,
	})
}
