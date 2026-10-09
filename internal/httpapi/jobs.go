package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/png"
	"io"
	"path/filepath"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// StartJobs resumes work that was running when the process stopped, then polls the queue.
func (a *App) StartJobs(ctx context.Context) {
	if err := a.store.ResumeJobs(ctx); err != nil {
		a.log.Error("resume jobs", "err", err)
	}
	n := a.cfg.JobWorkers
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		go a.jobLoop(ctx)
	}
}

func (a *App) jobLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		ok, err := a.ProcessNext(ctx)
		if err != nil {
			a.log.Error("job", "err", err)
		}
		if ok {
			continue
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// ProcessNext runs one due job. The boolean is false when the queue is empty.
func (a *App) ProcessNext(ctx context.Context) (bool, error) {
	job, err := a.store.ClaimJob(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := a.runJob(ctx, job); err != nil {
		a.log.Error("job failed", "id", job.ID, "art", job.Art, "err", err)
		if ferr := a.store.FailBelegJob(ctx, job, err); ferr != nil {
			return true, ferr
		}
	}
	return true, nil
}

func (a *App) runJob(ctx context.Context, job sqlitedb.Job) error {
	if job.Payload == nil || *job.Payload == "" {
		return errors.New("job without beleg")
	}
	belegID := *job.Payload
	switch job.Art {
	case "aufbereiten":
		return a.jobFoto(ctx, job.ID, belegID)
	case "pdf_vorschau":
		return a.jobPDF(ctx, job.ID, belegID)
	case "ki_auslesen":
		return a.jobKI(ctx, job.ID, belegID)
	case "erfassung_loeschen":
		keys, err := a.store.CompleteKarenz(ctx, belegID, job.ID)
		if err != nil {
			return err
		}
		a.dropKeys(ctx, keys, "beleg.erfassung_geloescht")
		return nil
	default:
		return errors.New("unknown job")
	}
}

func (a *App) jobFoto(ctx context.Context, jobID, belegID string) error {
	row, err := a.store.GetBelegByID(ctx, belegID)
	if err != nil {
		return err
	}
	if row.Status == "zur_bestaetigung" || row.Status == "bestaetigt" || row.Status == "storniert" {
		return a.store.CompleteBeleg(ctx, belegID, jobID, row.PipelineVersion, row.PipelineParameter, row.Seiten, nil)
	}
	files, err := a.store.ListBelegdateien(ctx, belegID)
	if err != nil {
		return err
	}
	settings := a.pipeSettings()
	var derived []store.DateiIn
	pages := 0
	for _, f := range files {
		if f.Variante != "erfassung" {
			continue
		}
		pages++
		rc, err := a.blobs.Get(ctx, f.SpeicherSchluessel)
		if err != nil {
			return err
		}
		img, _, decErr := belegpipe.DecodeLimited(rc, belegpipe.MaxPixels)
		_ = rc.Close()
		if decErr != nil {
			return decErr
		}
		photo, err := belegpipe.Process(img, settings)
		if err != nil {
			return err
		}
		parts := []struct {
			variante, ext, mime string
			body                []byte
		}{
			{"archiv", photo.ArchivExt, photo.ArchivMIME, photo.Archiv},
			{"vorschau", "webp", "image/webp", photo.Preview},
			{"export_jpeg", "jpg", "image/jpeg", photo.ExportJPEG},
		}
		for _, p := range parts {
			key := blobKey(row.NutzerID, belegID, p.variante, f.Seite, p.ext)
			sum := belegpipe.SHA256Hex(p.body)
			if err := a.blobs.PutIfAbsent(ctx, key, bytes.NewReader(p.body), sum); err != nil {
				return err
			}
			derived = append(derived, store.DateiIn{
				Variante: p.variante, Seite: f.Seite, Key: key, MIME: p.mime, Bytes: int64(len(p.body)), SHA256: sum,
			})
		}
	}
	if pages == 0 {
		return errors.New("no capture")
	}
	param := mergeParam(row.PipelineParameter, settings)
	return a.store.CompleteBeleg(ctx, belegID, jobID, belegpipe.Version, param, int64(pages), derived)
}

func (a *App) jobPDF(ctx context.Context, jobID, belegID string) error {
	row, err := a.store.GetBelegByID(ctx, belegID)
	if err != nil {
		return err
	}
	if row.Status == "zur_bestaetigung" || row.Status == "bestaetigt" || row.Status == "storniert" {
		return a.store.CompleteBeleg(ctx, belegID, jobID, row.PipelineVersion, row.PipelineParameter, row.Seiten, nil)
	}
	files, err := a.store.ListBelegdateien(ctx, belegID)
	if err != nil {
		return err
	}
	orig, ok := findDatei(files, "original", 1)
	if !ok {
		return errors.New("no original")
	}
	rc, err := a.blobs.Get(ctx, orig.SpeicherSchluessel)
	if err != nil {
		return err
	}
	pdf, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return err
	}
	pages := int(row.Seiten)
	if pages < 1 {
		pages = 1
	}
	bin := a.typstBin()
	param := map[string]any{"pipeline": belegpipe.Version}
	var derived []store.DateiIn
	if bin == "" {
		param["vorschau"] = "uebersprungen"
	} else {
		settings := a.pipeSettings()
		for page := 1; page <= pages; page++ {
			png, err := belegpipe.RenderPDFPage(bin, pdf, page)
			if err != nil {
				return err
			}
			img, _, err := image.Decode(bytes.NewReader(png))
			if err != nil {
				return err
			}
			preview, jpg, err := belegpipe.EncodePreviewJPEG(img, settings.JPEGQuality, settings.PreviewWebP)
			if err != nil {
				return err
			}
			seite := int64(page)
			for _, p := range []struct {
				variante, ext, mime string
				body                []byte
			}{
				{"vorschau", "webp", "image/webp", preview},
				{"export_jpeg", "jpg", "image/jpeg", jpg},
			} {
				key := blobKey(row.NutzerID, belegID, p.variante, seite, p.ext)
				sum := belegpipe.SHA256Hex(p.body)
				if err := a.blobs.PutIfAbsent(ctx, key, bytes.NewReader(p.body), sum); err != nil {
					return err
				}
				derived = append(derived, store.DateiIn{
					Variante: p.variante, Seite: seite, Key: key, MIME: p.mime, Bytes: int64(len(p.body)), SHA256: sum,
				})
			}
		}
		param["vorschau"] = "typst"
	}
	raw, err := json.Marshal(param)
	if err != nil {
		return err
	}
	return a.store.CompleteBeleg(ctx, belegID, jobID, belegpipe.Version, string(raw), row.Seiten, derived)
}

func (a *App) pipeSettings() belegpipe.Settings {
	return belegpipe.NormalizeSettings(belegpipe.Settings{
		Format:      a.cfg.BelegFormat,
		AVIFQuality: a.cfg.BelegAVIFQuality,
		AVIFSpeed:   a.cfg.BelegAVIFSpeed,
		WebPQuality: a.cfg.BelegWebPQuality,
		JPEGQuality: a.cfg.BelegJPEGQuality,
	})
}

func (a *App) karenz() time.Duration {
	if a.cfg.BelegKarenz > 0 {
		return a.cfg.BelegKarenz
	}
	return 720 * time.Hour
}

func (a *App) maxUpload() int64 {
	if a.cfg.UploadMaxBytes > 0 {
		return a.cfg.UploadMaxBytes
	}
	return 25 << 20
}

func (a *App) typstBin() string {
	p := a.cfg.TypstPath
	base := filepath.Base(p)
	if base != "typst" && base != "typst.exe" {
		return ""
	}
	if err := typstOK(p); err != nil {
		return ""
	}
	return p
}

func mergeParam(existing string, s belegpipe.Settings) string {
	base := map[string]any{}
	if existing != "" {
		_ = json.Unmarshal([]byte(existing), &base)
	}
	base["format"] = s.Format
	base["avif_quality"] = s.AVIFQuality
	base["avif_speed"] = s.AVIFSpeed
	base["webp_quality"] = s.WebPQuality
	base["jpeg_quality"] = s.JPEGQuality
	base["pipeline"] = belegpipe.Version
	raw, err := json.Marshal(base)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
