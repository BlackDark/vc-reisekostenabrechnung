package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/export"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) jobExport(ctx context.Context, jobID, exportID string) error {
	exp, err := a.store.GetExportByID(ctx, exportID)
	if err != nil {
		return err
	}
	var snap export.Snapshot
	if err := json.Unmarshal([]byte(exp.Snapshot), &snap); err != nil {
		return err
	}
	claim, err := a.store.GetAbrechnungBundle(ctx, exp.NutzerID, exp.AbrechnungID)
	if err != nil {
		return err
	}
	if claim.Row.Abrechnungsnummer == nil || *claim.Row.Abrechnungsnummer == "" {
		return errors.New("missing abrechnungsnummer")
	}
	ver := bounded(exp.Version)
	if ver < 1 {
		return errors.New("export version")
	}
	snap.Abrechnung.Nummer = *claim.Row.Abrechnungsnummer
	snap.Abrechnung.Version = int(ver)
	snap.Abrechnung.Status = "eingereicht"
	snap.ErzeugtAm = exp.ErstelltAm.UTC().Format(time.RFC3339)
	snap.Abrechnung.EingereichtAm = snap.ErzeugtAm
	if a.Version != "" {
		snap.AppVersion = a.Version
	}
	events, err := a.store.ListProtokoll(ctx, "abrechnung", exp.AbrechnungID, 200)
	if err != nil {
		return err
	}
	snap.Protokoll = nil
	for _, ev := range events {
		grund := ""
		if ev.Grund != nil {
			grund = *ev.Grund
		}
		snap.Protokoll = append(snap.Protokoll, export.Ereignis{
			Zeitpunkt: ev.Zeitpunkt.UTC().Format(time.RFC3339), AkteurArt: ev.AkteurArt,
			Aktion: ev.Aktion, ObjektTyp: ev.ObjektTyp, ObjektID: ev.ObjektID, Grund: grund,
		})
	}
	snap.WarnungenQuittiert = quittungen(claim.Row.QuittierteWarnungen)
	sources, err := a.exportSources(ctx, exp.NutzerID, snap)
	if err != nil {
		return err
	}
	bin := a.typstBin()
	if bin == "" {
		return errors.New("typst missing")
	}
	doc, err := renderExport(bin, snap, sources, false)
	if err != nil {
		return err
	}
	if err := export.ValidateJSON(doc.JSON); err != nil {
		return err
	}
	pdfKey := fmt.Sprintf("export/%s/v%d.pdf", exportID, ver)
	zipKey := fmt.Sprintf("export/%s/v%d.zip", exportID, ver)
	if err := a.putBlob(ctx, pdfKey, doc.PDF); err != nil {
		return err
	}
	if err := a.putBlob(ctx, zipKey, doc.ZIP); err != nil {
		return err
	}
	return a.store.CompleteExport(ctx, jobID, exportID, pdfKey, belegpipe.SHA256Hex(doc.PDF), zipKey, belegpipe.SHA256Hex(doc.ZIP), string(doc.JSON))
}

func (a *App) exportSources(ctx context.Context, nutzerID string, snap export.Snapshot) ([]export.Source, error) {
	var out []export.Source
	for _, beleg := range snap.Belege {
		if _, err := a.store.GetBeleg(ctx, nutzerID, beleg.ID); err != nil {
			return nil, err
		}
		files, err := a.store.ListBelegdateien(ctx, beleg.ID)
		if err != nil {
			return nil, err
		}
		names := map[string]string{}
		for _, d := range beleg.Dateien {
			names[d.Variante+"/"+itoa(int64(d.Seite))] = d.Name
		}
		add := func(f sqlitedb.Belegdatei, art string) error {
			body, err := a.readBlob(ctx, f.SpeicherSchluessel)
			if err != nil {
				return err
			}
			name := names[f.Variante+"/"+itoa(f.Seite)]
			if name == "" {
				name = beleg.Nummer
			}
			out = append(out, export.Source{
				Name: name, MIME: f.Mime, Bytes: body, Art: art,
				Nummer: beleg.Nummer, Seite: int(bounded(f.Seite)), Kopf: beleg.Nummer, Text: beleg.Text,
			})
			return nil
		}
		switch beleg.Typ {
		case "pdf", "e_rechnung_hybrid":
			if f, ok := oneFile(files, "original", "application/pdf"); ok {
				if err := add(f, "pdf"); err != nil {
					return nil, err
				}
			}
			if beleg.Typ == "e_rechnung_hybrid" {
				if f, ok := oneFile(files, "original", "xml"); ok {
					if err := add(f, "xml"); err != nil {
						return nil, err
					}
				}
			}
		case "e_rechnung_xml":
			if f, ok := oneFile(files, "original", "xml"); ok {
				if err := add(f, "xml"); err != nil {
					return nil, err
				}
			}
		default:
			var pages []sqlitedb.Belegdatei
			for _, f := range files {
				if f.Variante == "export_jpeg" {
					pages = append(pages, f)
				}
			}
			if len(pages) == 0 {
				if f, ok := oneFile(files, "original", "image/"); ok {
					pages = []sqlitedb.Belegdatei{f}
				}
			}
			for _, f := range pages {
				if err := add(f, "jpeg"); err != nil {
					return nil, err
				}
			}
		}
	}
	return out, nil
}

func oneFile(files []sqlitedb.Belegdatei, variante, mime string) (sqlitedb.Belegdatei, bool) {
	for _, f := range files {
		if f.Variante == variante && strings.Contains(f.Mime, mime) {
			return f, true
		}
	}
	for _, f := range files {
		if strings.Contains(f.Mime, mime) {
			return f, true
		}
	}
	return sqlitedb.Belegdatei{}, false
}

func quittungen(raw string) []export.Quittung {
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil || len(keys) == 0 {
		return []export.Quittung{}
	}
	out := make([]export.Quittung, 0, len(keys))
	for _, key := range keys {
		code, objekt, _ := strings.Cut(key, "\t")
		out = append(out, export.Quittung{Code: code, ObjektID: objekt})
	}
	return out
}

func renderExport(typst string, snap export.Snapshot, sources []export.Source, draft bool) (export.Result, error) {
	return export.Render(typst, snap, sources, draft)
}

func (a *App) putBlob(ctx context.Context, key string, body []byte) error {
	return a.blobs.PutIfAbsent(ctx, key, bytes.NewReader(body), belegpipe.SHA256Hex(body))
}

func (a *App) readBlob(ctx context.Context, key string) ([]byte, error) {
	rc, err := a.blobs.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func writeFile(w http.ResponseWriter, mime, name string, immutable bool, body []byte) {
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if immutable {
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
