package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// DateiIn is one blob already stored for a Beleg.
type DateiIn struct {
	Variante string
	Seite    int64
	Key      string
	MIME     string
	Bytes    int64
	SHA256   string
}

// NewBeleg is an upload that already has its bytes in storage.
type NewBeleg struct {
	ID          string
	Typ         string
	Seiten      int64
	SHA256      string
	DuplikatVon *string
	Parameter   string
	Status      string
	Dateien     []DateiIn
	JobArt      string
	JobAt       time.Time
}

// FindBelegDuplicate matches sha256_original or an Archivbeleg/original checksum.
func (s *Store) FindBelegDuplicate(ctx context.Context, nutzer, original string, files []string) (sqlitedb.Beleg, bool, error) {
	q := s.readQ()
	row, err := q.FindBelegBySHA(ctx, sqlitedb.FindBelegBySHAParams{NutzerID: nutzer, Sha256: original})
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return sqlitedb.Beleg{}, false, err
	}
	seen := map[string]struct{}{original: {}}
	for _, h := range files {
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		row, err = q.FindBelegByDateiSHA(ctx, sqlitedb.FindBelegByDateiSHAParams{NutzerID: nutzer, Sha256: h})
		if err == nil {
			return row, true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return sqlitedb.Beleg{}, false, err
		}
	}
	return sqlitedb.Beleg{}, false, nil
}

// CreateBeleg inserts the Beleg, its files and an optional job.
func (s *Store) CreateBeleg(ctx context.Context, nutzerID string, in NewBeleg, actor Actor) (sqlitedb.Beleg, error) {
	if in.ID == "" {
		in.ID = id.Must()
	}
	if in.Status == "" {
		in.Status = "in_aufbereitung"
	}
	if in.Parameter == "" {
		in.Parameter = "{}"
	}
	var out sqlitedb.Beleg
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		row, err := q.InsertBeleg(ctx, sqlitedb.InsertBelegParams{
			ID: in.ID, NutzerID: nutzerID, Typ: in.Typ, Status: in.Status, Seiten: in.Seiten,
			Sha256Original: in.SHA256, PipelineVersion: "", PipelineParameter: in.Parameter,
			DuplikatVon: in.DuplikatVon, ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		for _, d := range in.Dateien {
			if _, err := q.InsertBelegdatei(ctx, dateiParams(row.ID, d)); err != nil {
				return err
			}
		}
		if in.JobArt != "" {
			when := in.JobAt
			if when.IsZero() {
				when = now
			}
			if err := insertJob(ctx, q, in.JobArt, row.ID, when, now); err != nil {
				return err
			}
		}
		if err := appendAudit(ctx, q, actor, "beleg.hochgeladen", "beleg", row.ID, nil, mustJSON(map[string]any{
			"typ": row.Typ, "status": row.Status, "sha256_original": row.Sha256Original,
		}), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// GetBeleg loads one Beleg owned by the Nutzer.
func (s *Store) GetBeleg(ctx context.Context, nutzerID, belegID string) (sqlitedb.Beleg, error) {
	row, err := s.readQ().GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID})
	return row, mapErr(err)
}

// GetBelegByID loads a Beleg for a background job.
func (s *Store) GetBelegByID(ctx context.Context, belegID string) (sqlitedb.Beleg, error) {
	row, err := s.readQ().GetBelegByID(ctx, belegID)
	return row, mapErr(err)
}

// ListBelege returns a page. status empty means all. eingang limits the inbox.
func (s *Store) ListBelege(ctx context.Context, nutzerID, status, cursor string, eingang bool, limit int64) ([]sqlitedb.Beleg, error) {
	flag := int64(0)
	if eingang {
		flag = 1
	}
	return s.readQ().ListBelege(ctx, sqlitedb.ListBelegeParams{
		NutzerID: nutzerID, Status: status, NurEingang: flag, Cursor: cursor, LimitN: limit,
	})
}

// ListBelegdateien lists files of a Beleg.
func (s *Store) ListBelegdateien(ctx context.Context, belegID string) ([]sqlitedb.Belegdatei, error) {
	return s.readQ().ListBelegdateien(ctx, belegID)
}

// ListDuplikatBelege lists receipts explicitly kept despite a duplicate.
func (s *Store) ListDuplikatBelege(ctx context.Context, nutzerID string) ([]sqlitedb.Beleg, error) {
	return s.readQ().ListOffeneDuplikate(ctx, nutzerID)
}

// ConfirmBeleg assigns a gapless Belegnummer and schedules capture-file deletion.
func (s *Store) ConfirmBeleg(ctx context.Context, nutzerID, belegID string, version int64, karenz time.Duration, actor Actor) (sqlitedb.Beleg, error) {
	if karenz <= 0 {
		karenz = 720 * time.Hour
	}
	var out sqlitedb.Beleg
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		cur, err := q.GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if cur.Version != version {
			return ErrConflict
		}
		if cur.Status != "zur_bestaetigung" {
			return &CodeError{Code: "beleg_status"}
		}
		now := time.Now().UTC()
		year := int64(now.In(berlin()).Year())
		n, err := q.BumpBelegnummer(ctx, sqlitedb.BumpBelegnummerParams{NutzerID: nutzerID, Jahr: year})
		if err != nil {
			return err
		}
		nummer := fmt.Sprintf("%d-%04d", year, n)
		keep := RetentionDeadline(now)
		loeschen := now.Add(karenz)
		von := actorID(actor)
		out, err = q.ConfirmBeleg(ctx, sqlitedb.ConfirmBelegParams{
			Belegnummer: &nummer, BestaetigtAm: &now, BestaetigtVon: von, AufbewahrenBis: &keep,
			ErfassungLoeschenAm: &loeschen, GeaendertAm: now, ID: belegID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapErr(err)
		}
		if err := q.MarkBelegdateienFest(ctx, belegID); err != nil {
			return err
		}
		if err := insertJob(ctx, q, "erfassung_loeschen", belegID, loeschen, now); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "beleg.bestaetigt", "beleg", belegID, nil, mustJSON(map[string]any{
			"belegnummer": nummer, "aufbewahren_bis": keep,
		}), nil)
	})
	return out, err
}

// StornoBeleg cancels a confirmed Beleg. The number is not reused.
func (s *Store) StornoBeleg(ctx context.Context, nutzerID, belegID string, version int64, grund string, actor Actor) (sqlitedb.Beleg, error) {
	if grund == "" {
		return sqlitedb.Beleg{}, &CodeError{Code: "validierung"}
	}
	var out sqlitedb.Beleg
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		cur, err := q.GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if cur.Version != version {
			return ErrConflict
		}
		if cur.Status != "bestaetigt" {
			return &CodeError{Code: "beleg_status"}
		}
		now := time.Now().UTC()
		out, err = q.StornoBeleg(ctx, sqlitedb.StornoBelegParams{
			StornoGrund: &grund, StorniertAm: &now, GeaendertAm: now,
			ID: belegID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapErr(err)
		}
		return appendAudit(ctx, q, actor, "beleg.storniert", "beleg", belegID, nil, mustJSON(map[string]any{
			"status": "storniert",
		}), &grund)
	})
	return out, err
}

// DeleteBeleg removes an unconfirmed Beleg and returns storage keys.
func (s *Store) DeleteBeleg(ctx context.Context, nutzerID, belegID string, version int64, actor Actor) ([]string, error) {
	var keys []string
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		cur, err := q.GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if cur.Version != version {
			return ErrConflict
		}
		if cur.Status == "bestaetigt" || cur.Status == "storniert" {
			return &CodeError{Code: "beleg_fest"}
		}
		files, err := q.ListBelegdateien(ctx, belegID)
		if err != nil {
			return err
		}
		keys = dateiKeys(files)
		if err := q.DeleteJobsForBeleg(ctx, &belegID); err != nil {
			return err
		}
		n, err := q.DeleteBelegOffen(ctx, sqlitedb.DeleteBelegOffenParams{ID: belegID, NutzerID: nutzerID, Version: version})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrConflict
		}
		return appendAudit(ctx, q, actor, "beleg.geloescht", "beleg", belegID, mustJSON(map[string]any{
			"status": cur.Status, "sha256_original": cur.Sha256Original,
		}), nil, nil)
	})
	return keys, err
}

// ReprocessBeleg drops derived files and queues the pipeline again.
func (s *Store) ReprocessBeleg(ctx context.Context, nutzerID, belegID string, version int64, parameter string, actor Actor) (sqlitedb.Beleg, []string, error) {
	if parameter == "" {
		parameter = "{}"
	}
	var out sqlitedb.Beleg
	var keys []string
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		cur, err := q.GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if cur.Version != version {
			return ErrConflict
		}
		if cur.Status != "zur_bestaetigung" && cur.Status != "fehlgeschlagen" {
			return &CodeError{Code: "beleg_status"}
		}
		files, err := q.ListBelegdateien(ctx, belegID)
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.Variante == "archiv" || f.Variante == "vorschau" || f.Variante == "export_jpeg" {
				if f.Unveraenderbar {
					return &CodeError{Code: "beleg_fest"}
				}
				keys = append(keys, f.SpeicherSchluessel)
			}
		}
		for _, variante := range []string{"archiv", "vorschau", "export_jpeg"} {
			if err := q.DeleteBelegdateiVariante(ctx, sqlitedb.DeleteBelegdateiVarianteParams{BelegID: belegID, Variante: variante}); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		out, err = q.TouchBelegReprocess(ctx, sqlitedb.TouchBelegReprocessParams{
			PipelineParameter: parameter, GeaendertAm: now, ID: belegID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapErr(err)
		}
		art := "aufbereiten"
		if cur.Typ == "pdf" || cur.Typ == "e_rechnung_hybrid" {
			art = "pdf_vorschau"
		}
		if err := insertJob(ctx, q, art, belegID, now, now); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "beleg.neu_aufbereitet", "beleg", belegID, nil, mustJSON(map[string]any{
			"status": "in_aufbereitung",
		}), nil)
	})
	return out, keys, err
}

// ReplaceCapture swaps the capture files and queues processing again.
func (s *Store) ReplaceCapture(ctx context.Context, nutzerID, belegID string, version int64, parameter string, files []DateiIn, actor Actor) (sqlitedb.Beleg, []string, error) {
	if parameter == "" {
		parameter = "{}"
	}
	var out sqlitedb.Beleg
	var keys []string
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		cur, err := q.GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if cur.Version != version {
			return ErrConflict
		}
		if cur.Status == "bestaetigt" || cur.Status == "storniert" {
			return &CodeError{Code: "beleg_fest"}
		}
		existing, err := q.ListBelegdateien(ctx, belegID)
		if err != nil {
			return err
		}
		for _, f := range existing {
			if f.Unveraenderbar {
				return &CodeError{Code: "beleg_fest"}
			}
			keys = append(keys, f.SpeicherSchluessel)
		}
		for _, variante := range []string{"erfassung", "archiv", "vorschau", "export_jpeg", "original"} {
			if err := q.DeleteBelegdateiVariante(ctx, sqlitedb.DeleteBelegdateiVarianteParams{BelegID: belegID, Variante: variante}); err != nil {
				return err
			}
		}
		for _, d := range files {
			if _, err := q.InsertBelegdatei(ctx, dateiParams(belegID, d)); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		out, err = q.TouchBelegReprocess(ctx, sqlitedb.TouchBelegReprocessParams{
			PipelineParameter: parameter, GeaendertAm: now, ID: belegID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapErr(err)
		}
		if err := insertJob(ctx, q, "aufbereiten", belegID, now, now); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "beleg.neu_aufbereitet", "beleg", belegID, nil, mustJSON(map[string]any{
			"status": "in_aufbereitung",
		}), nil)
	})
	return out, keys, err
}

// CompleteBeleg stores derived files and marks the Beleg ready.
func (s *Store) CompleteBeleg(ctx context.Context, belegID, jobID, pipelineVersion, parameter string, seiten int64, files []DateiIn) error {
	if parameter == "" {
		parameter = "{}"
	}
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		cur, err := q.GetBelegByID(ctx, belegID)
		if err != nil {
			return mapErr(err)
		}
		now := time.Now().UTC()
		if cur.Status == "zur_bestaetigung" || cur.Status == "bestaetigt" || cur.Status == "storniert" {
			return q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: now, ID: jobID})
		}
		for _, variante := range []string{"archiv", "vorschau", "export_jpeg"} {
			if err := q.DeleteBelegdateiVariante(ctx, sqlitedb.DeleteBelegdateiVarianteParams{BelegID: belegID, Variante: variante}); err != nil {
				return err
			}
		}
		for _, d := range files {
			if _, err := q.InsertBelegdatei(ctx, dateiParams(belegID, d)); err != nil {
				return err
			}
		}
		if _, err := q.UpdateBelegReady(ctx, sqlitedb.UpdateBelegReadyParams{
			Seiten: seiten, PipelineVersion: pipelineVersion, PipelineParameter: parameter, GeaendertAm: now, ID: belegID,
		}); err != nil {
			return err
		}
		return q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: now, ID: jobID})
	})
}

// CompleteKarenz deletes capture-file rows and returns their storage keys.
func (s *Store) CompleteKarenz(ctx context.Context, belegID, jobID string) ([]string, error) {
	var keys []string
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		files, err := q.ListBelegdateien(ctx, belegID)
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.Variante == "erfassung" {
				keys = append(keys, f.SpeicherSchluessel)
			}
		}
		if err := q.DeleteBelegdateiVariante(ctx, sqlitedb.DeleteBelegdateiVarianteParams{BelegID: belegID, Variante: "erfassung"}); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := q.ClearErfassung(ctx, sqlitedb.ClearErfassungParams{GeaendertAm: now, ID: belegID}); err != nil {
			return err
		}
		if err := q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: now, ID: jobID}); err != nil {
			return err
		}
		return appendAudit(ctx, q, Actor{Art: "system"}, "beleg.erfassung_geloescht", "beleg", belegID, nil, mustJSON(map[string]any{
			"dateien": len(keys),
		}), nil)
	})
	return keys, err
}

// ClaimJob marks the next due job running. ErrNotFound means the queue is empty.
func (s *Store) ClaimJob(ctx context.Context) (sqlitedb.Job, error) {
	now := time.Now().UTC()
	var job sqlitedb.Job
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		var err error
		job, err = q.ClaimJob(ctx, sqlitedb.ClaimJobParams{Jetzt: now, Stale: now.Add(-15 * time.Minute)})
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return sqlitedb.Job{}, ErrNotFound
	}
	return job, err
}

// ResumeJobs returns running jobs to the queue after a restart.
func (s *Store) ResumeJobs(ctx context.Context) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		return q.ResumeJobs(ctx, time.Now().UTC())
	})
}

// FailBelegJob retries with backoff or marks the job and the Beleg failed.
func (s *Store) FailBelegJob(ctx context.Context, job sqlitedb.Job, cause error) error {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		limit := int64(5)
		if job.Art == "ki_auslesen" {
			limit = 1
		}
		if job.Versuche >= limit {
			if err := q.FailJobRow(ctx, sqlitedb.FailJobRowParams{Fehler: &msg, Jetzt: now, ID: job.ID}); err != nil {
				return err
			}
			if job.Payload != nil && (job.Art == "aufbereiten" || job.Art == "pdf_vorschau") {
				if err := q.SetBelegFailed(ctx, sqlitedb.SetBelegFailedParams{GeaendertAm: now, ID: *job.Payload}); err != nil {
					return err
				}
			}
			return nil
		}
		shift := job.Versuche
		if shift > 8 {
			shift = 8
		}
		delay := time.Duration(1<<uint(shift)) * time.Second
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		when := now.Add(delay)
		return q.RescheduleJob(ctx, sqlitedb.RescheduleJobParams{
			Fehler: &msg, NaechsterVersuchAm: &when, Jetzt: now, ID: job.ID,
		})
	})
}

func insertJob(ctx context.Context, q *sqlitedb.Queries, art, belegID string, when, now time.Time) error {
	_, err := q.InsertJob(ctx, sqlitedb.InsertJobParams{
		ID: id.Must(), Art: art, Payload: &belegID, NaechsterVersuchAm: &when, ErstelltAm: now, GeaendertAm: now,
	})
	return err
}

func dateiParams(belegID string, d DateiIn) sqlitedb.InsertBelegdateiParams {
	return sqlitedb.InsertBelegdateiParams{
		ID: id.Must(), BelegID: belegID, Variante: d.Variante, Seite: d.Seite,
		SpeicherSchluessel: d.Key, Mime: d.MIME, Bytes: d.Bytes, Sha256: d.SHA256,
	}
}

func dateiKeys(files []sqlitedb.Belegdatei) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.SpeicherSchluessel)
	}
	return out
}

func actorID(actor Actor) *string {
	if actor.NutzerID == nil || *actor.NutzerID == "" {
		return nil
	}
	return actor.NutzerID
}
