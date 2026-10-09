package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) GetBelege(w http.ResponseWriter, r *http.Request, params api.GetBelegeParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	status := ""
	if params.Status != nil {
		status = *params.Status
	}
	cursor := ""
	if params.Cursor != nil {
		cursor = *params.Cursor
	}
	eingang := params.Eingang != nil && *params.Eingang
	limit := limitOf(params.Limit)
	rows, err := a.store.ListBelege(r.Context(), n.ID, status, cursor, eingang, limit)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.Beleg, 0, len(rows))
	for _, row := range rows {
		items = append(items, toBeleg(row))
	}
	var next *string
	if int64(len(rows)) == limit && len(rows) > 0 {
		id := rows[len(rows)-1].ID
		next = &id
	}
	writeJSON(w, http.StatusOK, api.BelegListe{Items: items, NextCursor: next})
}

func (a *App) GetBeleg(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetBeleg(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, toBeleg(row))
}

func (a *App) GetBelegDuplikate(w http.ResponseWriter, r *http.Request, params api.GetBelegDuplikateParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, found, err := a.store.FindBelegDuplicate(r.Context(), n.ID, params.Sha256, []string{params.Sha256})
	if writeStoreErr(w, err) {
		return
	}
	items := []api.Beleg{}
	if found {
		items = append(items, toBeleg(row))
	}
	writeJSON(w, http.StatusOK, api.BelegListe{Items: items})
}

func (a *App) PostBelege(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	files, err := a.readUploads(w, r)
	if err != nil {
		return
	}
	if len(files) == 0 {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "A file is required", "")
		return
	}
	kind, bodies, err := classifyUploads(files)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Unsupported file", err.Error())
		return
	}
	original, parts := hashParts(bodies)
	dup, found, err := a.store.FindBelegDuplicate(r.Context(), n.ID, original, parts)
	if writeStoreErr(w, err) {
		return
	}
	confirmed := formBool(r, "duplikat_bestaetigt")
	if found && !confirmed {
		writeProblem(w, http.StatusConflict, "duplikat", "Duplicate receipt", dup.ID)
		return
	}
	var dupOf *string
	if found {
		dupOf = &dup.ID
	}
	belegID := id.Must()
	profil := formProfil(r)
	ecken := r.FormValue("ecken")
	param, err := json.Marshal(map[string]string{"profil": profil, "ecken": ecken})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	stored, dateien, err := a.storeOriginals(r, n.ID, belegID, kind, bodies)
	if err != nil {
		a.dropKeys(r.Context(), stored, "beleg.hochgeladen")
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	in := store.NewBeleg{
		ID: belegID, Typ: kind.typ, Seiten: int64(len(bodies)), SHA256: original, DuplikatVon: dupOf,
		Parameter: string(param), Status: kind.status, Dateien: dateien, JobArt: kind.job,
	}
	if kind.typ == "pdf" || kind.typ == "e_rechnung_hybrid" {
		in.Seiten = int64(kind.pages)
	}
	row, err := a.store.CreateBeleg(r.Context(), n.ID, in, a.actor(r, n))
	if err != nil {
		a.dropKeys(r.Context(), stored, "beleg.hochgeladen")
		if writeStoreErr(w, err) {
			return
		}
	}
	writeObject(w, http.StatusAccepted, row.Version, toBeleg(row))
}

func (a *App) PostBelegNeuAufbereiten(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostBelegNeuAufbereitenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	profil := formProfil(r)
	ecken := r.FormValue("ecken")
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if _, err := a.readUploads(w, r); err != nil {
			return
		}
		profil = formProfil(r)
		ecken = r.FormValue("ecken")
	}
	param, err := json.Marshal(map[string]string{"profil": profil, "ecken": ecken})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	var row sqlitedb.Beleg
	var keys []string
	headers := r.MultipartForm
	if headers != nil && len(headers.File["datei"]) > 0 {
		files := headers.File["datei"]
		kind, bodies, cerr := classifyUploads(files)
		if cerr != nil || kind.typ != "foto" {
			writeProblem(w, http.StatusUnprocessableEntity, "validierung", "A photo is required", "")
			return
		}
		dateien := planDateien(n.ID, id, kind, bodies)
		row, keys, err = a.store.ReplaceCapture(r.Context(), n.ID, id, version, string(param), dateien, a.actor(r, n))
		if writeStoreErr(w, err) {
			return
		}
		a.dropKeys(r.Context(), keys, "beleg.neu_aufbereitet")
		if _, _, serr := a.storeOriginals(r, n.ID, id, kind, bodies); serr != nil {
			writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
			return
		}
		writeObject(w, http.StatusAccepted, row.Version, toBeleg(row))
		return
	}
	row, keys, err = a.store.ReprocessBeleg(r.Context(), n.ID, id, version, string(param), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	a.dropKeys(r.Context(), keys, "beleg.neu_aufbereitet")
	writeObject(w, http.StatusAccepted, row.Version, toBeleg(row))
}

func (a *App) PostBelegBestaetigen(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostBelegBestaetigenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	row, err := a.store.ConfirmBeleg(r.Context(), n.ID, id, version, a.karenz(), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, toBeleg(row))
}

func (a *App) PostBelegStornieren(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostBelegStornierenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.BelegStorno
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.StornoBeleg(r.Context(), n.ID, id, version, strings.TrimSpace(body.Grund), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, toBeleg(row))
}

func (a *App) DeleteBeleg(w http.ResponseWriter, r *http.Request, id api.Id, params api.DeleteBelegParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	keys, err := a.store.DeleteBeleg(r.Context(), n.ID, id, version, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	a.dropKeys(r.Context(), keys, "beleg.geloescht")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetBelegVorschau(w http.ResponseWriter, r *http.Request, id api.Id) {
	a.serveDatei(w, r, id, "vorschau", 1, true)
}

func (a *App) GetBelegSeite(w http.ResponseWriter, r *http.Request, id api.Id, n int) {
	if n < 1 {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if _, err := a.store.GetBeleg(r.Context(), user.ID, id); writeStoreErr(w, err) {
		return
	}
	files, err := a.store.ListBelegdateien(r.Context(), id)
	if writeStoreErr(w, err) {
		return
	}
	if f, ok := findDatei(files, "archiv", int64(n)); ok {
		a.writeBlob(w, r, f, false)
		return
	}
	if f, ok := findDatei(files, "original", int64(n)); ok {
		a.writeBlob(w, r, f, false)
		return
	}
	if n == 1 {
		if f, ok := findDatei(files, "original", 1); ok {
			a.writeBlob(w, r, f, false)
			return
		}
	}
	writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
}

func (a *App) GetBelegOriginal(w http.ResponseWriter, r *http.Request, id api.Id) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetBeleg(r.Context(), user.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	files, err := a.store.ListBelegdateien(r.Context(), id)
	if writeStoreErr(w, err) {
		return
	}
	variante := "erfassung"
	if row.Typ != "foto" {
		variante = "original"
	}
	f, ok := findDatei(files, variante, 1)
	if !ok {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	a.writeBlob(w, r, f, false)
}

func (a *App) serveDatei(w http.ResponseWriter, r *http.Request, id, variante string, seite int64, inline bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if _, err := a.store.GetBeleg(r.Context(), user.ID, id); writeStoreErr(w, err) {
		return
	}
	files, err := a.store.ListBelegdateien(r.Context(), id)
	if writeStoreErr(w, err) {
		return
	}
	f, ok := findDatei(files, variante, seite)
	if !ok {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	a.writeBlob(w, r, f, inline)
}

func (a *App) writeBlob(w http.ResponseWriter, r *http.Request, f sqlitedb.Belegdatei, inline bool) {
	rc, err := a.blobs.Get(r.Context(), f.SpeicherSchluessel)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	defer func() { _ = rc.Close() }()
	h := w.Header()
	h.Set("Content-Type", f.Mime)
	h.Set("X-Content-Type-Options", "nosniff")
	disp := "attachment"
	if inline {
		disp = "inline"
	}
	h.Set("Content-Disposition", disp+`; filename="`+blobName(f)+`"`)
	if f.Unveraenderbar {
		h.Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "private, max-age=60")
	}
	if f.Mime == "application/pdf" {
		h.Set("Content-Security-Policy", "sandbox")
	}
	_, _ = io.Copy(w, rc)
}

func (a *App) readUploads(w http.ResponseWriter, r *http.Request) ([]*multipart.FileHeader, error) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUpload())
	mem := a.maxUpload()
	if mem > 8<<20 {
		mem = 8 << 20
	}
	if err := r.ParseMultipartForm(mem); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			writeProblem(w, http.StatusRequestEntityTooLarge, "datei_zu_gross", "File is too large", "")
			return nil, err
		}
		writeProblem(w, http.StatusBadRequest, "ungueltige_anfrage", "Bad request", "")
		return nil, err
	}
	if r.MultipartForm == nil {
		return nil, nil
	}
	return r.MultipartForm.File["datei"], nil
}

type uploadKind struct {
	typ    string
	status string
	job    string
	pages  int
}

func classifyUploads(files []*multipart.FileHeader) (uploadKind, [][]byte, error) {
	bodies := make([][]byte, 0, len(files))
	kinds := make([]string, 0, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			return uploadKind{}, nil, err
		}
		b, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return uploadKind{}, nil, err
		}
		if len(b) == 0 {
			return uploadKind{}, nil, errors.New("empty file")
		}
		kind := belegpipe.Sniff(b)
		if kind == "" {
			return uploadKind{}, nil, errors.New("unknown type")
		}
		bodies = append(bodies, b)
		kinds = append(kinds, kind)
	}
	images := kinds[0] == "jpeg" || kinds[0] == "png" || kinds[0] == "webp"
	for _, k := range kinds {
		isImg := k == "jpeg" || k == "png" || k == "webp"
		if isImg != images {
			return uploadKind{}, nil, errors.New("mixed types")
		}
	}
	if images {
		for _, b := range bodies {
			if _, _, err := belegpipe.DecodeLimited(bytes.NewReader(b), belegpipe.MaxPixels); err != nil {
				return uploadKind{}, nil, err
			}
		}
		return uploadKind{typ: "foto", status: "in_aufbereitung", job: "aufbereiten", pages: len(bodies)}, bodies, nil
	}
	if len(bodies) != 1 {
		return uploadKind{}, nil, errors.New("one document")
	}
	b := bodies[0]
	switch kinds[0] {
	case "pdf":
		pages, err := belegpipe.PDFPages(b)
		if err != nil {
			return uploadKind{}, nil, err
		}
		typ := "pdf"
		if belegpipe.PDFHybrid(b) {
			typ = "e_rechnung_hybrid"
		}
		return uploadKind{typ: typ, status: "in_aufbereitung", job: "pdf_vorschau", pages: pages}, bodies, nil
	case "xml":
		if _, err := belegpipe.XMLRoot(b); err != nil {
			return uploadKind{}, nil, err
		}
		return uploadKind{typ: "e_rechnung_xml", status: "zur_bestaetigung", pages: 1}, bodies, nil
	default:
		return uploadKind{}, nil, errors.New("unknown type")
	}
}

func hashParts(bodies [][]byte) (string, []string) {
	all := sha256.New()
	parts := make([]string, len(bodies))
	for i, b := range bodies {
		parts[i] = belegpipe.SHA256Hex(b)
		_, _ = all.Write(b)
	}
	return hex.EncodeToString(all.Sum(nil)), parts
}

func planDateien(nutzerID, belegID string, kind uploadKind, bodies [][]byte) []store.DateiIn {
	dateien := make([]store.DateiIn, 0, len(bodies))
	for i, b := range bodies {
		seite := int64(i + 1)
		sniff := belegpipe.Sniff(b)
		variante := "erfassung"
		if kind.typ != "foto" {
			variante = "original"
		}
		dateien = append(dateien, store.DateiIn{
			Variante: variante, Seite: seite, Key: blobKey(nutzerID, belegID, variante, seite, extOfKind(sniff)),
			MIME: mimeOfKind(sniff), Bytes: int64(len(b)), SHA256: belegpipe.SHA256Hex(b),
		})
	}
	return dateien
}

func (a *App) storeOriginals(r *http.Request, nutzerID, belegID string, kind uploadKind, bodies [][]byte) ([]string, []store.DateiIn, error) {
	dateien := planDateien(nutzerID, belegID, kind, bodies)
	keys := make([]string, 0, len(dateien))
	for i, d := range dateien {
		if err := a.blobs.PutIfAbsent(r.Context(), d.Key, bytes.NewReader(bodies[i]), d.SHA256); err != nil {
			return keys, nil, err
		}
		keys = append(keys, d.Key)
	}
	return keys, dateien, nil
}

func (a *App) dropKeys(ctx context.Context, keys []string, ref string) {
	for _, key := range keys {
		if err := a.blobs.DeleteForRetention(ctx, key, ref); err != nil {
			a.log.Error("delete blob", "key", key, "err", err)
		}
	}
}

func blobKey(nutzer, beleg, variante string, seite int64, ext string) string {
	return fmt.Sprintf("nutzer/%s/belege/%s/%s-%d.%s", nutzer, beleg, variante, seite, ext)
}

func extOfKind(kind string) string {
	switch kind {
	case "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	case "pdf":
		return "pdf"
	case "xml":
		return "xml"
	default:
		return "bin"
	}
}

func mimeOfKind(kind string) string {
	switch kind {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "pdf":
		return "application/pdf"
	case "xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

func blobName(f sqlitedb.Belegdatei) string {
	ext := "bin"
	switch f.Mime {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	case "image/avif":
		ext = "avif"
	case "application/pdf":
		ext = "pdf"
	case "application/xml":
		ext = "xml"
	}
	return fmt.Sprintf("%s-%d.%s", f.Variante, f.Seite, ext)
}

func findDatei(files []sqlitedb.Belegdatei, variante string, seite int64) (sqlitedb.Belegdatei, bool) {
	for _, f := range files {
		if f.Variante == variante && f.Seite == seite {
			return f, true
		}
	}
	return sqlitedb.Belegdatei{}, false
}

func formBool(r *http.Request, key string) bool {
	v := strings.ToLower(strings.TrimSpace(r.FormValue(key)))
	return v == "1" || v == "true" || v == "on" || v == "ja"
}

func formProfil(r *http.Request) string {
	if r.FormValue("profil") == "a4" {
		return "a4"
	}
	return "bon"
}

func toBeleg(row sqlitedb.Beleg) api.Beleg {
	var pipe *string
	if row.PipelineVersion != "" {
		pipe = &row.PipelineVersion
	}
	return api.Beleg{
		Id: row.ID, Belegnummer: row.Belegnummer, Typ: row.Typ, Status: row.Status,
		Seiten: intFrom64(row.Seiten), Sha256Original: row.Sha256Original, PipelineVersion: pipe,
		DuplikatVon: row.DuplikatVon, ErstelltAm: row.ErstelltAm.UTC(), Version: intFrom64(row.Version),
	}
}

func intFrom64(n int64) int {
	if n > math.MaxInt || n < math.MinInt {
		return 0
	}
	return int(n)
}
