package httpapi

import (
	"net/http"
	"strconv"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) GetAbrechnungen(w http.ResponseWriter, r *http.Request, params api.GetAbrechnungenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var limit int64
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 200 {
		limit = int64(*params.Limit)
	}
	rows, err := a.store.ListAbrechnungen(r.Context(), n.ID, params.Cursor, limit)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.Abrechnung, 0, len(rows))
	for _, row := range rows {
		b, err := a.store.GetAbrechnungBundle(r.Context(), n.ID, row.ID)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
			return
		}
		items = append(items, toAbrechnung(b))
	}
	var next *string
	if params.Limit != nil && len(rows) == *params.Limit && len(rows) > 0 {
		id := rows[len(rows)-1].ID
		next = &id
	}
	writeJSON(w, http.StatusOK, api.AbrechnungListe{Items: items, NextCursor: next})
}

func (a *App) PostAbrechnung(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.AbrechnungWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in := store.AbrechnungInput{
		ArbeitgeberID: body.ArbeitgeberId, ZeitraumArt: body.ZeitraumArt, Von: body.Von, Bis: body.Bis,
		Titel: deref(body.Titel), Sprache: deref(body.Sprache),
		ReiseIDs: body.ReiseIds, VorschussIDs: body.VorschussIds,
	}
	b, err := a.store.CreateAbrechnung(r.Context(), n.ID, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusCreated, b.Row.Version, toAbrechnung(b))
}

func (a *App) GetAbrechnung(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	b, err := a.store.GetAbrechnungBundle(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) PatchAbrechnung(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchAbrechnungParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.AbrechnungKopf
	if !decodeJSON(w, r, &body) {
		return
	}
	b, err := a.store.PatchAbrechnung(r.Context(), n.ID, id, version, body.Titel, body.Sprache, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) DeleteAbrechnung(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if writeStoreErr(w, a.store.DeleteAbrechnung(r.Context(), n.ID, id, a.actor(r, n))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) PutAbrechnungReisen(w http.ResponseWriter, r *http.Request, id api.Id, params api.PutAbrechnungReisenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.IdListe
	if !decodeJSON(w, r, &body) {
		return
	}
	b, err := a.store.SetAbrechnungReisen(r.Context(), n.ID, id, version, body.Ids, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) PutAbrechnungVorschuesse(w http.ResponseWriter, r *http.Request, id api.Id, params api.PutAbrechnungVorschuesseParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.IdListe
	if !decodeJSON(w, r, &body) {
		return
	}
	b, err := a.store.SetAbrechnungVorschuesse(r.Context(), n.ID, id, version, body.Ids, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) GetAbrechnungPruefung(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	view, err := a.reviewAbrechnung(r, n, id)
	if writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, view.pruefung)
}

func (a *App) GetAbrechnungVorschau(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	bin := a.typstBin()
	if bin == "" {
		writeProblem(w, http.StatusServiceUnavailable, "typst_fehlt", "Typst is missing", "")
		return
	}
	view, err := a.reviewAbrechnung(r, n, id)
	if writeStoreErr(w, err) {
		return
	}
	view.snap.Abrechnung.Nummer = "ENTWURF"
	view.snap.Abrechnung.Version = 1
	view.snap.Abrechnung.Status = "entwurf"
	sources, err := a.exportSources(r.Context(), n.ID, view.snap)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	doc, err := renderExport(bin, view.snap, sources, true)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "export", "Export failed", "")
		return
	}
	writeFile(w, "application/pdf", "vorschau.pdf", false, doc.PDF)
}

func (a *App) PostAbrechnungEinreichen(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostAbrechnungEinreichenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.Einreichen
	if !decodeJSON(w, r, &body) {
		return
	}
	view, err := a.reviewAbrechnung(r, n, id)
	if writeStoreErr(w, err) {
		return
	}
	raw, err := snapshotJSON(view.snap)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var keys []string
	for _, q := range body.QuittierteWarnungen {
		keys = append(keys, store.WarnKey(q.Code, q.ObjektId))
	}
	b, err := a.store.Einreichen(r.Context(), n.ID, id, store.EinreichenInput{
		Version: version, Blocker: view.blockerKeys, Warnungen: view.warnKeys, Quittiert: keys, Snapshot: string(raw),
	}, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusAccepted, b.Row.Version, toAbrechnung(b))
}

func (a *App) PostAbrechnungEntsperren(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostAbrechnungEntsperrenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.Grund
	if !decodeJSON(w, r, &body) {
		return
	}
	b, err := a.store.Entsperren(r.Context(), n.ID, id, version, body.Grund, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) PostAbrechnungBezahlt(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostAbrechnungBezahltParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.Bezahlt
	if !decodeJSON(w, r, &body) {
		return
	}
	b, err := a.store.MarkBezahlt(r.Context(), n.ID, id, version, body.BezahltAm, deref(body.Vermerk), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) PostAbrechnungBezahltZuruecknehmen(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostAbrechnungBezahltZuruecknehmenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.Grund
	if !decodeJSON(w, r, &body) {
		return
	}
	b, err := a.store.BezahltZuruecknehmen(r.Context(), n.ID, id, version, body.Grund, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, b.Row.Version, toAbrechnung(b))
}

func (a *App) GetAbrechnungExporte(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListExporte(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.Export, 0, len(rows))
	for _, row := range rows {
		items = append(items, toExport(row))
	}
	writeJSON(w, http.StatusOK, api.ExportListe{Items: items})
}

func (a *App) GetAbrechnungProtokoll(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if _, err := a.store.GetAbrechnungBundle(r.Context(), n.ID, id); writeStoreErr(w, err) {
		return
	}
	rows, err := a.store.ListProtokoll(r.Context(), "abrechnung", id, 200)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.ProtokollEreignis, 0, len(rows))
	for _, row := range rows {
		items = append(items, toProtokoll(row))
	}
	writeJSON(w, http.StatusOK, api.ProtokollListe{Items: items})
}

func (a *App) GetExport(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetExport(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toExport(row))
}

func (a *App) GetExportPdf(w http.ResponseWriter, r *http.Request, id api.Id) {
	a.writeExportFile(w, r, id, true)
}

func (a *App) GetExportZip(w http.ResponseWriter, r *http.Request, id api.Id) {
	a.writeExportFile(w, r, id, false)
}

func (a *App) writeExportFile(w http.ResponseWriter, r *http.Request, id string, pdf bool) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetExport(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	if row.Status != "fertig" {
		writeProblem(w, http.StatusConflict, "export", "Export is not ready", "")
		return
	}
	key := row.ZipSchluessel
	name := "abrechnung-v" + strconv.FormatInt(row.Version, 10) + ".zip"
	mime := "application/zip"
	if pdf {
		key = row.PdfSchluessel
		name = "abrechnung-v" + strconv.FormatInt(row.Version, 10) + ".pdf"
		mime = "application/pdf"
	}
	if key == nil || *key == "" {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	body, err := a.readBlob(r.Context(), *key)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	writeFile(w, mime, name, true, body)
}

func toAbrechnung(b store.AbrechnungBundle) api.Abrechnung {
	row := b.Row
	out := api.Abrechnung{
		Id: row.ID, ArbeitgeberId: row.ArbeitgeberID, Abrechnungsnummer: row.Abrechnungsnummer,
		ZeitraumArt: row.ZeitraumArt, Von: row.Von, Bis: row.Bis, Titel: row.Titel, Status: row.Status,
		EingereichtAm: row.EingereichtAm, BezahltAm: row.BezahltAm, BezahltVermerk: row.BezahltVermerk,
		ExportSprache: row.ExportSprache, AktuelleExportVersion: row.AktuelleExportVersion,
		EinreichungLaeuft: row.EinreichungLaeuft, EinreichungFehler: row.EinreichungFehler,
		ReiseIds: orEmpty(b.ReiseIDs), VorschussIds: orEmpty(b.VorschussIDs),
		VorschlaegeReise: orEmpty(b.VorschlaegeReise), VorschlaegeVorschuss: orEmpty(b.VorschlaegeVorschuss),
		Version: row.Version,
	}
	return out
}

func toExport(row sqlitedb.Export) api.Export {
	return api.Export{
		Id: row.ID, AbrechnungId: row.AbrechnungID, Version: row.Version, Anlass: row.Anlass,
		Status: row.Status, Fehler: row.Fehler, ErsetztDurchVersion: row.ErsetztDurchVersion,
		ErstelltAm: row.ErstelltAm,
	}
}

func toProtokoll(row sqlitedb.AuditEreigni) api.ProtokollEreignis {
	return api.ProtokollEreignis{
		Id: row.ID, Zeitpunkt: row.Zeitpunkt, Aktion: row.Aktion, ObjektTyp: row.ObjektTyp,
		ObjektId: row.ObjektID, AkteurArt: row.AkteurArt, Grund: row.Grund,
	}
}

func orEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
