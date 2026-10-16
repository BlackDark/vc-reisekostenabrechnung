package httpapi

import (
	"net/http"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
)

// GetProtokoll returns the signed-in Nutzer's own audit events, optionally
// filtered by object type and object id.
func (a *App) GetProtokoll(w http.ResponseWriter, r *http.Request, params api.GetProtokollParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	limit := limitOf(params.Limit)
	typ, oid := "", ""
	if params.ObjektTyp != nil {
		typ = *params.ObjektTyp
	}
	if params.ObjektId != nil {
		oid = *params.ObjektId
	}
	rows, err := a.store.ListProtokollForNutzer(r.Context(), n.ID, typ, oid, limit)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.ProtokollEreignis, 0, len(rows))
	for _, row := range rows {
		items = append(items, toProtokoll(row))
	}
	writeJSON(w, http.StatusOK, api.ProtokollListe{Items: items})
}
