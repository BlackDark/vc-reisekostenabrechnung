package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/ki"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) PostBelegKi(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if !a.cfg.AI.Enabled {
		writeProblem(w, http.StatusConflict, "ki_deaktiviert", "KI is disabled", "")
		return
	}
	if !a.limits.allow("ai:"+n.ID, a.cfg.RateAI.Count, a.cfg.RateAI.Per) {
		w.Header().Set("Retry-After", "60")
		writeProblem(w, http.StatusTooManyRequests, "rate_limit", "Too many requests", "")
		return
	}
	row, err := a.store.GetBeleg(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	if row.Typ == "e_rechnung_xml" {
		writeProblem(w, http.StatusUnprocessableEntity, "ki_xml", "E-invoice XML is not sent to KI", "")
		return
	}
	if !n.KiErlaubt {
		writeProblem(w, http.StatusConflict, "ki_nicht_erlaubt", "KI opt-in is required", "")
		return
	}
	if row.Status != "zur_bestaetigung" && row.Status != "bestaetigt" {
		writeProblem(w, http.StatusConflict, "beleg_nicht_bereit", "Beleg is not ready", "")
		return
	}
	job, err := a.store.EnqueueKI(r.Context(), row.ID)
	if writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusAccepted, api.KiAuftrag{Status: "laeuft", JobId: &job.ID})
}

func (a *App) GetBelegKi(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if _, err := a.store.GetBeleg(r.Context(), n.ID, id); writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, a.kiStand(r.Context(), n, id))
}

func (a *App) GetBelegTexte(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListBelegtexte(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.Belegtext, 0, len(rows))
	for _, row := range rows {
		items = append(items, toBelegtext(row))
	}
	writeJSON(w, http.StatusOK, api.BelegtextListe{Items: items})
}

func (a *App) PostBelegText(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.BelegtextWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Felder == nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "felder is required", "")
		return
	}
	raw, err := json.Marshal(body.Felder)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "felder is invalid", "")
		return
	}
	text := ""
	if body.Volltext != nil {
		text = *body.Volltext
	} else if body.Felder.Volltext != nil {
		text = *body.Felder.Volltext
	}
	row, err := a.store.ConfirmBelegtext(r.Context(), n.ID, id, string(raw), text, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toBelegtext(row))
}

func (a *App) kiStand(ctx context.Context, n sqlitedb.Nutzer, belegID string) api.KiStand {
	if !a.cfg.AI.Enabled {
		return api.KiStand{Status: "aus"}
	}
	if !n.KiErlaubt {
		return api.KiStand{Status: "nicht_erlaubt"}
	}
	job, err := a.store.LatestKIJob(ctx, belegID)
	if errors.Is(err, store.ErrNotFound) {
		return api.KiStand{Status: "keine"}
	}
	if err != nil {
		return api.KiStand{Status: "nicht_erreichbar"}
	}
	switch job.Status {
	case "pending", "running":
		return api.KiStand{Status: "laeuft"}
	case "failed":
		return api.KiStand{Status: "nicht_erreichbar"}
	}
	rows, err := a.store.ListBelegtexte(ctx, n.ID, belegID)
	if err != nil {
		return api.KiStand{Status: "leer"}
	}
	var latest *sqlitedb.Belegtext
	for i := range rows {
		if rows[i].Quelle == "ki" {
			latest = &rows[i]
		}
	}
	if latest == nil {
		return api.KiStand{Status: "leer"}
	}
	suggestion, ok := decodeVorschlag(latest.Felder)
	if !ok {
		return api.KiStand{Status: "leer"}
	}
	return api.KiStand{Status: "vorschlag", Vorschlag: &suggestion}
}

func (a *App) jobKI(ctx context.Context, jobID, belegID string) error {
	row, err := a.store.GetBelegByID(ctx, belegID)
	if err != nil {
		return err
	}
	if !a.cfg.AI.Enabled || a.ki == nil || row.Typ == "e_rechnung_xml" {
		return a.finishKI(ctx, jobID, belegID, ki.Outcome{Empty: true}, "")
	}
	images, err := a.kiImages(ctx, row)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return a.finishKI(ctx, jobID, belegID, ki.Outcome{Empty: true}, "")
	}
	if err := a.kiGate.Acquire(ctx); err != nil {
		return err
	}
	defer a.kiGate.Release()
	start := time.Now()
	out, err := a.ki.Extract(ctx, images)
	a.log.Info("ki.auslesen", "beleg_id", belegID, "modell", a.cfg.AI.Model, "dauer_ms", time.Since(start).Milliseconds(), "erfolg", err == nil && !out.Empty)
	if err != nil {
		return err
	}
	return a.finishKI(ctx, jobID, belegID, out, out.Mode)
}

func (a *App) finishKI(ctx context.Context, jobID, belegID string, out ki.Outcome, mode string) error {
	felder := `{"leer":true}`
	volltext := ""
	if !out.Empty {
		raw, err := json.Marshal(vorschlagFromFields(out.Fields))
		if err != nil {
			return err
		}
		felder = string(raw)
		volltext = out.Fields.Volltext
	}
	meta, err := json.Marshal(map[string]any{"modell": a.cfg.AI.Model, "leer": out.Empty, "modus": mode})
	if err != nil {
		return err
	}
	return a.store.CompleteKI(ctx, jobID, belegID, felder, volltext, meta)
}

func (a *App) kiImages(ctx context.Context, row sqlitedb.Beleg) ([][]byte, error) {
	files, err := a.store.ListBelegdateien(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	var pages []sqlitedb.Belegdatei
	for _, f := range files {
		if f.Variante == "export_jpeg" {
			pages = append(pages, f)
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Seite < pages[j].Seite })
	if len(pages) == 0 && row.Typ == "pdf" {
		return a.pdfJPEGs(ctx, row, files)
	}
	if len(pages) > 3 {
		pages = pages[:3]
	}
	return a.readFitted(ctx, pages)
}

func (a *App) pdfJPEGs(ctx context.Context, row sqlitedb.Beleg, files []sqlitedb.Belegdatei) ([][]byte, error) {
	bin := a.typstBin()
	if bin == "" {
		return nil, nil
	}
	orig, ok := findDatei(files, "original", 1)
	if !ok {
		return nil, nil
	}
	rc, err := a.blobs.Get(ctx, orig.SpeicherSchluessel)
	if err != nil {
		return nil, err
	}
	pdf, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return nil, err
	}
	pages := 1
	if row.Seiten > 1 && row.Seiten <= 3 {
		pages = int(row.Seiten)
	} else if row.Seiten > 3 {
		pages = 3
	}
	quality := a.cfg.BelegJPEGQuality
	if quality < 1 || quality > 100 {
		quality = 70
	}
	var out [][]byte
	for page := 1; page <= pages; page++ {
		png, err := belegpipe.RenderPDFPage(ctx, bin, pdf, page)
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(bytes.NewReader(png))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		fitted, err := ki.FitJPEG(buf.Bytes(), a.kiMaxEdge())
		if err != nil {
			return nil, err
		}
		out = append(out, fitted)
	}
	return out, nil
}

func (a *App) readFitted(ctx context.Context, pages []sqlitedb.Belegdatei) ([][]byte, error) {
	var out [][]byte
	for _, f := range pages {
		rc, err := a.blobs.Get(ctx, f.SpeicherSchluessel)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		fitted, err := ki.FitJPEG(raw, a.kiMaxEdge())
		if err != nil {
			return nil, err
		}
		out = append(out, fitted)
	}
	return out, nil
}

func (a *App) kiMaxEdge() int {
	if a.cfg.AI.MaxImagePx >= 256 {
		return a.cfg.AI.MaxImagePx
	}
	return 2480
}

func vorschlagFromFields(f ki.Fields) api.KiVorschlag {
	shares := make([]api.KiAnteil, 0, len(f.Steueranteile))
	for _, sh := range f.Steueranteile {
		netto, steuer := sh.Netto, sh.Steuer
		shares = append(shares, api.KiAnteil{Satz: sh.Satz, BruttoCent: sh.Brutto, NettoCent: &netto, SteuerCent: &steuer})
	}
	gross := f.BetragBrutto
	tip := f.Trinkgeld
	conf := map[string]float32{}
	for k, v := range f.Konfidenz {
		conf[k] = float32(v)
	}
	out := api.KiVorschlag{
		Leistender: strPtr(f.Leistender), Datum: strPtr(f.Datum), Waehrung: strPtr(f.Waehrung),
		BetragBruttoCent: &gross, EmpfaengerName: strPtr(f.EmpfaengerName), Volltext: strPtr(f.Volltext),
		TrinkgeldCent: &tip,
	}
	if f.Kostenart != "" {
		out.Kostenart = strPtr(f.Kostenart)
	}
	if f.Rechnungsart != "" {
		out.Rechnungsart = strPtr(f.Rechnungsart)
	}
	if f.Rechnungsnummer != "" {
		out.Rechnungsnummer = strPtr(f.Rechnungsnummer)
	}
	if f.UstIDLeistender != "" {
		out.UstIdLeistender = strPtr(f.UstIDLeistender)
	}
	if len(shares) > 0 {
		out.Steueranteile = &shares
	}
	if len(conf) > 0 {
		out.Konfidenz = &conf
	}
	return out
}

func decodeVorschlag(raw string) (api.KiVorschlag, bool) {
	var v api.KiVorschlag
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return api.KiVorschlag{}, false
	}
	if v.BetragBruttoCent == nil || *v.BetragBruttoCent <= 0 {
		return api.KiVorschlag{}, false
	}
	return v, true
}

func toBelegtext(row sqlitedb.Belegtext) api.Belegtext {
	out := api.Belegtext{
		Id: row.ID, BelegId: row.BelegID, Version: row.Version, Quelle: row.Quelle, Volltext: row.Volltext,
		BestaetigtAm: row.BestaetigtAm,
	}
	if v, ok := decodeVorschlag(row.Felder); ok {
		out.Felder = &v
	}
	return out
}
