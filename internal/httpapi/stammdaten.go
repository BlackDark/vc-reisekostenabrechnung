package httpapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/files"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) GetArbeitgeber(w http.ResponseWriter, r *http.Request, params api.GetArbeitgeberParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	limit := limitOf(params.Limit)
	rows, err := a.store.ListArbeitgeber(r.Context(), n.ID, params.Cursor, limit+1)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items, next := pageArbeitgeber(rows, limit)
	writeJSON(w, http.StatusOK, api.ArbeitgeberListe{Items: items, NextCursor: next})
}

func (a *App) PostArbeitgeber(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.ArbeitgeberWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, ok := arbeitgeberFromWrite(body)
	if !ok {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check name, address and prefix", "")
		return
	}
	row, err := a.store.CreateArbeitgeber(r.Context(), n.ID, in, a.actor(r, n))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeObject(w, http.StatusCreated, row.Version, toArbeitgeber(row))
}

func (a *App) GetArbeitgeberById(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetArbeitgeber(r.Context(), n.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Arbeitgeber not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeObject(w, http.StatusOK, row.Version, toArbeitgeber(row))
}

func (a *App) PatchArbeitgeber(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchArbeitgeberParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	prev, err := a.store.GetArbeitgeber(r.Context(), n.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Arbeitgeber not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var body api.ArbeitgeberPatch
	if !decodeJSON(w, r, &body) {
		return
	}
	in := store.ArbeitgeberInput{
		Name: prev.Name, Anschrift: prev.Anschrift, UstID: prev.UstID, Steuernummer: prev.Steuernummer,
		Praefix: prev.AbrechnungsnummerPraefix, IstStandard: prev.IstStandard, Archiviert: prev.Archiviert,
	}
	if body.Name != nil {
		in.Name = strings.TrimSpace(*body.Name)
	}
	if body.Anschrift != nil {
		in.Anschrift = strings.TrimSpace(*body.Anschrift)
	}
	if body.UstId != nil {
		in.UstID = emptyNil(*body.UstId)
	}
	if body.Steuernummer != nil {
		in.Steuernummer = emptyNil(*body.Steuernummer)
	}
	if body.AbrechnungsnummerPraefix != nil {
		in.Praefix = strings.TrimSpace(*body.AbrechnungsnummerPraefix)
	}
	if body.IstStandard != nil {
		in.IstStandard = *body.IstStandard
	}
	if body.Archiviert != nil {
		in.Archiviert = *body.Archiviert
	}
	if !textOK(in.Name, 200) || !textOK(in.Anschrift, 2000) || !praefixOK(in.Praefix) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check name, address and prefix", "")
		return
	}
	row, err := a.store.UpdateArbeitgeber(r.Context(), n.ID, id, version, in, a.actor(r, n))
	if errors.Is(err, store.ErrConflict) {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Arbeitgeber not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeObject(w, http.StatusOK, row.Version, toArbeitgeber(row))
}

func (a *App) GetArbeitgeberLogo(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	ag, err := a.store.GetArbeitgeber(r.Context(), n.ID, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && ag.LogoDateiID == nil) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Logo not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	file, err := a.store.GetDatei(r.Context(), n.ID, *ag.LogoDateiID)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Logo not found", "")
		return
	}
	body, err := files.Read(a.fileRoot(), file.SpeicherSchluessel)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Logo not found", "")
		return
	}
	w.Header().Set("Content-Type", file.Mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (a *App) PostArbeitgeberLogo(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostArbeitgeberLogoParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	body, ok := readUpload(w, r, 2<<20)
	if !ok {
		return
	}
	mime, ok := imageMIME(body)
	if !ok {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Logo must be PNG or JPEG", "")
		return
	}
	key := "nutzer/" + n.ID + "/" + newStorageID()
	if err := files.Write(a.fileRoot(), key, body); err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	row, err := a.store.SetLogo(r.Context(), n.ID, id, version, mime, body, key, a.actor(r, n))
	if err != nil {
		_ = files.Remove(a.fileRoot(), key)
		if errors.Is(err, store.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Arbeitgeber not found", "")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeObject(w, http.StatusOK, row.Version, toArbeitgeber(row))
}

func (a *App) GetTaetigkeitsstaetten(w http.ResponseWriter, r *http.Request, params api.GetTaetigkeitsstaettenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	limit := limitOf(params.Limit)
	rows, err := a.store.ListTaetigkeiten(r.Context(), n.ID, params.Cursor, limit+1)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.Taetigkeitsstaette, 0, len(rows))
	var next *string
	if int64(len(rows)) > limit {
		rows = rows[:limit]
		id := rows[len(rows)-1].ID
		next = &id
	}
	for _, row := range rows {
		items = append(items, toTaetigkeit(row))
	}
	writeJSON(w, http.StatusOK, api.TaetigkeitsstaetteListe{Items: items, NextCursor: next})
}

func (a *App) PostTaetigkeitsstaette(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.TaetigkeitsstaetteWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, ok := a.taetigkeitInput(w, r, body)
	if !ok {
		return
	}
	row, err := a.store.CreateTaetigkeit(r.Context(), n.ID, in, a.actor(r, n))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeObject(w, http.StatusCreated, row.Version, toTaetigkeit(row))
}

func (a *App) GetTaetigkeitsstaette(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetTaetigkeit(r.Context(), n.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Tätigkeitsstätte not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeObject(w, http.StatusOK, row.Version, toTaetigkeit(row))
}

func (a *App) PatchTaetigkeitsstaette(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchTaetigkeitsstaetteParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.TaetigkeitsstaetteWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, ok := a.taetigkeitInput(w, r, body)
	if !ok {
		return
	}
	row, err := a.store.UpdateTaetigkeit(r.Context(), n.ID, id, version, in, a.actor(r, n))
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Tätigkeitsstätte not found", "")
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
	writeObject(w, http.StatusOK, row.Version, toTaetigkeit(row))
}

func (a *App) DeleteTaetigkeitsstaette(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	err := a.store.DeleteTaetigkeit(r.Context(), n.ID, id, a.actor(r, n))
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Tätigkeitsstätte not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetSatztabellen(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	rows, err := a.store.ListSatztabellen(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.SatztabelleKurz, 0, len(rows))
	for _, row := range rows {
		abroad, err := a.store.ListAusland(r.Context(), int(row.Jahr))
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
			return
		}
		n := len(abroad)
		items = append(items, api.SatztabelleKurz{
			Jahr: int(row.Jahr), Status: row.Status, Quelle: row.Quelle, Version: row.Version, Auslandssaetze: &n,
		})
	}
	writeJSON(w, http.StatusOK, api.SatztabelleListe{Items: items})
}

func (a *App) GetSatztabelle(w http.ResponseWriter, r *http.Request, jahr api.Jahr) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	view, ok := a.yearView(w, r, jahr)
	if !ok {
		return
	}
	writeObject(w, http.StatusOK, view.Version, view)
}

func (a *App) PatchSatztabelle(w http.ResponseWriter, r *http.Request, jahr api.Jahr, params api.PatchSatztabelleParams) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.SatzOverrideRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	grund := strings.TrimSpace(body.Grund)
	if grund == "" || len(grund) > 500 || body.NeuerWert < 0 || body.NeuerWert > 100_000_000 {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check field, value and reason", "")
		return
	}
	land, ort := "", ""
	if body.LandIso != nil {
		land = strings.ToUpper(strings.TrimSpace(*body.LandIso))
	}
	if body.Satzort != nil {
		ort = satz.NormalizeSatzort(land, *body.Satzort)
	}
	if land != "" && (len(land) != 2) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "land_iso must be an ISO code", "")
		return
	}
	_, err := a.store.OverrideSatztabelle(r.Context(), jahr, version, land, ort, body.Feld, body.NeuerWert, grund, a.actor(r, admin))
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Rate not found", "")
		return
	}
	if errors.Is(err, store.ErrInvalid) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Unknown field", "")
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
	view, ok := a.yearView(w, r, jahr)
	if !ok {
		return
	}
	writeObject(w, http.StatusOK, view.Version, view)
}

func (a *App) GetAuslandssaetze(w http.ResponseWriter, r *http.Request, jahr api.Jahr, params api.GetAuslandssaetzeParams) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	y, err := a.store.EffectiveYear(r.Context(), jahr)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Satztabelle not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	q := ""
	if params.Q != nil {
		q = strings.ToLower(strings.TrimSpace(*params.Q))
	}
	land := ""
	if params.LandIso != nil {
		land = strings.ToUpper(strings.TrimSpace(*params.LandIso))
	}
	rows := make([]satz.Row, 0, len(y.Rows))
	for _, row := range y.Rows {
		if land != "" && row.LandISO != land {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(row.LandNameDE), q) &&
			!strings.Contains(strings.ToLower(row.OrtName), q) &&
			!strings.Contains(strings.ToLower(row.LandISO), q) &&
			!strings.Contains(strings.ToLower(row.Satzort), q) {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].LandNameDE == rows[j].LandNameDE {
			return rows[i].Satzort < rows[j].Satzort
		}
		return rows[i].LandNameDE < rows[j].LandNameDE
	})
	limit := limitOf(params.Limit)
	start := 0
	if params.Cursor != nil && *params.Cursor != "" {
		for i, row := range rows {
			if rateKey(row) == *params.Cursor {
				start = i + 1
				break
			}
		}
	}
	end := start + int(limit)
	var next *string
	if end < len(rows) {
		key := rateKey(rows[end-1])
		next = &key
	} else {
		end = len(rows)
	}
	if start > len(rows) {
		start = len(rows)
	}
	items := make([]api.Auslandssatz, 0, end-start)
	for _, row := range rows[start:end] {
		ort := row.OrtName
		items = append(items, api.Auslandssatz{
			LandIso: row.LandISO, LandNameDe: row.LandNameDE, Satzort: row.Satzort,
			OrtName: &ort, Vma24h: row.VMA24h, Vma8h: row.VMA8h, Uebernachtung: row.Uebernachtung,
		})
	}
	writeJSON(w, http.StatusOK, api.AuslandssatzListe{Items: items, NextCursor: next})
}

func (a *App) GetSatzOverrides(w http.ResponseWriter, r *http.Request, jahr api.Jahr) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	if _, err := a.store.GetSatztabelle(r.Context(), jahr); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Satztabelle not found", "")
		return
	}
	rows, err := a.store.ListOverrides(r.Context(), jahr)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.SatzOverride, 0, len(rows))
	for _, row := range rows {
		items = append(items, api.SatzOverride{
			Id: row.ID, Jahr: int(row.Jahr), LandIso: row.LandIso, Satzort: row.Satzort,
			Feld: row.Feld, AlterWert: row.AlterWert, NeuerWert: row.NeuerWert, Grund: row.Grund,
			AdminId: &row.AdminID, Zeitpunkt: row.Zeitpunkt.UTC(),
		})
	}
	writeJSON(w, http.StatusOK, api.SatzOverrideListe{Items: items})
}

func (a *App) PostSatztabelleImport(w http.ResponseWriter, r *http.Request, jahr api.Jahr) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	body, ok := readUpload(w, r, 1<<20)
	if !ok {
		return
	}
	parsed, errs := satz.ParseImport(bytes.NewReader(body), jahr)
	if len(errs) > 0 {
		fields := make([]api.FieldError, 0, len(errs))
		for _, e := range errs {
			fields = append(fields, api.FieldError{Pointer: "/zeilen/" + strconv.Itoa(e.Zeile), Code: e.Code})
		}
		writeProblemFields(w, http.StatusUnprocessableEntity, "validierung", "The CSV was not imported", fields)
		return
	}
	row, err := a.store.ImportSatztabelle(r.Context(), jahr, parsed, a.actor(r, admin))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeJSON(w, http.StatusOK, api.SatztabelleImport{Jahr: int(row.Jahr), Status: row.Status, Zeilen: len(parsed)})
}

func (a *App) PostSatztabelleAktivieren(w http.ResponseWriter, r *http.Request, jahr api.Jahr) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	if _, err := a.store.ActivateSatztabelle(r.Context(), jahr, a.actor(r, admin)); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Satztabelle not found", "")
		return
	} else if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	view, ok := a.yearView(w, r, jahr)
	if !ok {
		return
	}
	writeObject(w, http.StatusOK, view.Version, view)
}

func (a *App) GetAdminIdentitaeten(w http.ResponseWriter, r *http.Request, id api.Id) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	if _, err := a.store.GetNutzer(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Nutzer not found", "")
		return
	}
	rows, err := a.store.ListIdentitaeten(r.Context(), id)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.Identitaet, 0, len(rows))
	for _, row := range rows {
		items = append(items, toIdentitaet(row))
	}
	writeJSON(w, http.StatusOK, api.IdentitaetListe{Items: items})
}

func (a *App) PostAdminIdentitaet(w http.ResponseWriter, r *http.Request, id api.Id) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	if _, err := a.store.GetNutzer(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Nutzer not found", "")
		return
	}
	var body api.IdentitaetWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	art := string(body.Art)
	aussteller := strings.TrimSpace(body.Aussteller)
	subjekt := strings.TrimSpace(body.Subjekt)
	if (art != "oidc" && art != "header") || aussteller == "" || subjekt == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check art, issuer and subject", "")
		return
	}
	if err := a.store.LinkIdentitaet(r.Context(), id, art, aussteller, subjekt, a.actor(r, admin)); err != nil {
		writeProblem(w, http.StatusConflict, "konflikt", "Identity is already linked", "")
		return
	}
	rows, err := a.store.ListIdentitaeten(r.Context(), id)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	for _, row := range rows {
		if row.Art == art && row.Aussteller == aussteller && row.Subjekt == subjekt {
			writeJSON(w, http.StatusCreated, toIdentitaet(row))
			return
		}
	}
	writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
}

func (a *App) DeleteAdminIdentitaet(w http.ResponseWriter, r *http.Request, id api.Id, params api.DeleteAdminIdentitaetParams) {
	admin, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	err := a.store.UnlinkIdentitaet(r.Context(), id, params.IdentitaetId, a.actor(r, admin))
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Identity not found", "")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetAdminProtokoll(w http.ResponseWriter, r *http.Request, params api.GetAdminProtokollParams) {
	if _, ok := a.requireAdmin(w, r); !ok {
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
	rows, err := a.store.ListProtokoll(r.Context(), typ, oid, limit)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.ProtokollEreignis, 0, len(rows))
	for _, row := range rows {
		items = append(items, api.ProtokollEreignis{
			Id: row.ID, Zeitpunkt: row.Zeitpunkt.UTC(), Aktion: row.Aktion,
			ObjektTyp: row.ObjektTyp, ObjektId: row.ObjektID, AkteurArt: row.AkteurArt, Grund: row.Grund,
		})
	}
	writeJSON(w, http.StatusOK, api.ProtokollListe{Items: items})
}

func (a *App) yearView(w http.ResponseWriter, r *http.Request, jahr int) (api.Satztabelle, bool) {
	row, err := a.store.GetSatztabelle(r.Context(), jahr)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Satztabelle not found", "")
		return api.Satztabelle{}, false
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return api.Satztabelle{}, false
	}
	y, err := a.store.EffectiveYear(r.Context(), jahr)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return api.Satztabelle{}, false
	}
	laender, err := a.store.ListLaender(r.Context(), jahr)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return api.Satztabelle{}, false
	}
	lands := make([]api.Land, 0, len(laender))
	for _, land := range laender {
		lands = append(lands, api.Land{LandIso: land.LandIso, LandNameDe: land.LandNameDe})
	}
	in := y.Inland
	return api.Satztabelle{
		Jahr: int(row.Jahr), Status: api.SatztabelleStatus(row.Status), Quelle: row.Quelle, Version: row.Version,
		VmaInland24h: in.VMA24h, VmaInland8h: in.VMA8h,
		KuerzungFruehstueckPct: &in.KuerzungFruehstueckPct, KuerzungHauptmahlzeitPct: &in.KuerzungHauptmahlzeitPct,
		UebernachtungInlandPauschale: in.Uebernachtung, KmKraftwagen: &in.KmKraftwagen, KmAnderesMotorfahrzeug: &in.KmAnderes,
		SachbezugFruehstueck: &in.SachbezugFruehstueck, SachbezugHauptmahlzeit: &in.SachbezugHauptmahlzeit,
		UeblicheMahlzeitGrenze: &in.UeblicheMahlzeit, Kleinbetragsgrenze: &in.Kleinbetrag, BewirtungAbzugPct: &in.BewirtungAbzug,
		AufbewahrungJahre: &in.AufbewahrungJahre, FlugZwischentageLand: &in.Flug, SchiffLand: &in.Schiff,
		UstSaetze: &in.UstSaetze, Laender: lands,
	}, true
}

func (a *App) taetigkeitInput(w http.ResponseWriter, r *http.Request, body api.TaetigkeitsstaetteWrite) (store.TaetigkeitInput, bool) {
	name := strings.TrimSpace(body.Bezeichnung)
	land := strings.ToUpper(strings.TrimSpace(body.LandIso))
	ort := ""
	if body.Satzort != nil {
		ort = satz.NormalizeSatzort(land, *body.Satzort)
	}
	anschrift := ""
	if body.Anschrift != nil {
		anschrift = strings.TrimSpace(*body.Anschrift)
	}
	if !textOK(name, 200) || len(land) != 2 || len(anschrift) > 2000 {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check name, country and place", "")
		return store.TaetigkeitInput{}, false
	}
	if err := a.store.KnownSatzort(r.Context(), land, ort); errors.Is(err, store.ErrInvalid) {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Unknown Satzort", "")
		return store.TaetigkeitInput{}, false
	} else if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return store.TaetigkeitInput{}, false
	}
	var kunde *string
	if body.Kunde != nil {
		kunde = emptyNil(*body.Kunde)
	}
	return store.TaetigkeitInput{Bezeichnung: name, Anschrift: anschrift, LandISO: land, Satzort: ort, Kunde: kunde}, true
}

func (a *App) fileRoot() string {
	if a.cfg.StorageLocalPath != "" {
		return a.cfg.StorageLocalPath
	}
	return a.cfg.DataDir + "/files"
}

func arbeitgeberFromWrite(body api.ArbeitgeberWrite) (store.ArbeitgeberInput, bool) {
	praefix := "RK"
	if body.AbrechnungsnummerPraefix != nil && strings.TrimSpace(*body.AbrechnungsnummerPraefix) != "" {
		praefix = strings.TrimSpace(*body.AbrechnungsnummerPraefix)
	}
	in := store.ArbeitgeberInput{
		Name: strings.TrimSpace(body.Name), Anschrift: strings.TrimSpace(body.Anschrift), Praefix: praefix,
		UstID: emptyNilPtr(body.UstId), Steuernummer: emptyNilPtr(body.Steuernummer),
	}
	if body.IstStandard != nil {
		in.IstStandard = *body.IstStandard
	}
	if !textOK(in.Name, 200) || !textOK(in.Anschrift, 2000) || !praefixOK(in.Praefix) {
		return store.ArbeitgeberInput{}, false
	}
	return in, true
}

func toArbeitgeber(row sqlitedb.Arbeitgeber) api.Arbeitgeber {
	return api.Arbeitgeber{
		Id: row.ID, Name: row.Name, Anschrift: row.Anschrift, UstId: row.UstID, Steuernummer: row.Steuernummer,
		LogoDateiId: row.LogoDateiID, IstStandard: row.IstStandard,
		Konstellation:            api.ArbeitgeberKonstellation(row.Konstellation),
		AbrechnungsnummerPraefix: row.AbrechnungsnummerPraefix, Archiviert: row.Archiviert, Version: row.Version,
	}
}

func toTaetigkeit(row sqlitedb.Taetigkeitsstaette) api.Taetigkeitsstaette {
	return api.Taetigkeitsstaette{
		Id: row.ID, Bezeichnung: row.Bezeichnung, Anschrift: row.Anschrift, LandIso: row.LandIso,
		Satzort: row.Satzort, Kunde: row.Kunde, Version: row.Version,
	}
}

func toIdentitaet(row sqlitedb.NutzerIdentitaet) api.Identitaet {
	return api.Identitaet{Id: row.ID, Art: api.IdentitaetArt(row.Art), Aussteller: row.Aussteller, Subjekt: row.Subjekt}
}

func pageArbeitgeber(rows []sqlitedb.Arbeitgeber, limit int64) ([]api.Arbeitgeber, *string) {
	var next *string
	if int64(len(rows)) > limit {
		rows = rows[:limit]
		id := rows[len(rows)-1].ID
		next = &id
	}
	items := make([]api.Arbeitgeber, 0, len(rows))
	for _, row := range rows {
		items = append(items, toArbeitgeber(row))
	}
	return items, next
}

func writeObject(w http.ResponseWriter, status int, version int64, body any) {
	w.Header().Set("ETag", strconv.FormatInt(version, 10))
	writeJSON(w, status, body)
}

func limitOf(limit *int) int64 {
	if limit == nil || *limit <= 0 || *limit > 200 {
		return 50
	}
	return int64(*limit)
}

func textOK(s string, max int) bool {
	return s != "" && len(s) <= max
}

func praefixOK(s string) bool {
	if s == "" || len(s) > 12 {
		return false
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func emptyNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func emptyNilPtr(s *string) *string {
	if s == nil {
		return nil
	}
	return emptyNil(*s)
}

func rateKey(row satz.Row) string {
	return row.LandISO + "|" + row.Satzort
}

func readUpload(w http.ResponseWriter, r *http.Request, max int64) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, max+8192)
	if err := r.ParseMultipartForm(max); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Upload could not be read", "")
		return nil, false
	}
	f, _, err := r.FormFile("datei")
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "File field datei is required", "")
		return nil, false
	}
	defer func() { _ = f.Close() }()
	body, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(body)) > max {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "File is too large", "")
		return nil, false
	}
	return body, true
}

func imageMIME(body []byte) (string, bool) {
	if len(body) >= 8 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "image/png", true
	}
	if len(body) >= 3 && body[0] == 0xff && body[1] == 0xd8 && body[2] == 0xff {
		return "image/jpeg", true
	}
	return "", false
}

func newStorageID() string { return id.Must() }
