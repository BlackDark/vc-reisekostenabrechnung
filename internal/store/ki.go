package store

import (
	"context"
	"errors"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// EnqueueKI queues receipt reading. A job that is already waiting is returned as-is.
func (s *Store) EnqueueKI(ctx context.Context, belegID string) (sqlitedb.Job, error) {
	existing, err := s.LatestKIJob(ctx, belegID)
	if err == nil && (existing.Status == "pending" || existing.Status == "running") {
		return existing, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return sqlitedb.Job{}, err
	}
	now := time.Now().UTC()
	var job sqlitedb.Job
	err = s.tx(ctx, func(q *sqlitedb.Queries) error {
		var ins error
		job, ins = q.InsertJob(ctx, sqlitedb.InsertJobParams{
			ID: id.Must(), Art: "ki_auslesen", Payload: &belegID,
			NaechsterVersuchAm: &now, ErstelltAm: now, GeaendertAm: now,
		})
		return ins
	})
	return job, err
}

// LatestKIJob is the newest ki_auslesen job for a Beleg.
func (s *Store) LatestKIJob(ctx context.Context, belegID string) (sqlitedb.Job, error) {
	payload := belegID
	row, err := s.readQ().LatestJob(ctx, sqlitedb.LatestJobParams{Art: "ki_auslesen", Payload: &payload})
	return row, mapErr(err)
}

// CompleteKI stores an unconfirmed Belegtext and finishes the job.
func (s *Store) CompleteKI(ctx context.Context, jobID, belegID, felder, volltext string, meta []byte) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		if _, err := insertBelegtext(ctx, q, belegID, "ki", felder, volltext, nil, nil); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: now, ID: jobID}); err != nil {
			return err
		}
		return appendAudit(ctx, q, Actor{Art: "system"}, "ki.auslesen", "beleg", belegID, nil, meta, nil)
	})
}

// ConfirmBelegtext stores the fields the Nutzer accepted as a new version.
func (s *Store) ConfirmBelegtext(ctx context.Context, nutzerID, belegID, felder, volltext string, actor Actor) (sqlitedb.Belegtext, error) {
	if _, err := s.GetBeleg(ctx, nutzerID, belegID); err != nil {
		return sqlitedb.Belegtext{}, err
	}
	now := time.Now().UTC()
	var row sqlitedb.Belegtext
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		var ins error
		row, ins = insertBelegtext(ctx, q, belegID, "manuell", felder, volltext, &now, &nutzerID)
		if ins != nil {
			return ins
		}
		return appendAudit(ctx, q, actor, "belegtext.bestaetigt", "beleg", belegID, nil, []byte(`{"quelle":"manuell"}`), nil)
	})
	return row, err
}

// ListBelegtexte lists versions of one Beleg owned by the Nutzer.
func (s *Store) ListBelegtexte(ctx context.Context, nutzerID, belegID string) ([]sqlitedb.Belegtext, error) {
	if _, err := s.GetBeleg(ctx, nutzerID, belegID); err != nil {
		return nil, err
	}
	rows, err := s.readQ().ListBelegtexte(ctx, belegID)
	return rows, mapErr(err)
}

func insertBelegtext(ctx context.Context, q *sqlitedb.Queries, belegID, quelle, felder, volltext string, confirmed *time.Time, by *string) (sqlitedb.Belegtext, error) {
	ver, err := q.MaxBelegtextVersion(ctx, belegID)
	if err != nil {
		return sqlitedb.Belegtext{}, err
	}
	if felder == "" {
		felder = "{}"
	}
	return q.InsertBelegtext(ctx, sqlitedb.InsertBelegtextParams{
		ID: id.Must(), BelegID: belegID, Version: ver + 1, Quelle: quelle,
		Felder: felder, Volltext: volltext, BestaetigtAm: confirmed, BestaetigtVon: by,
	})
}
