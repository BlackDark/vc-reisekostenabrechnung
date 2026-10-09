package store

import (
	"context"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// RetentionItem is one Beleg or Export in the admin retention report.
type RetentionItem struct {
	Art             string
	ID              string
	NutzerID        string
	Bezeichnung     string
	AufbewahrenBis  string
	Abgelaufen      bool
	InhaltGeloescht bool
	SHA256          string
}

// PurgeInput selects expired files. Ablaufhemmung must be confirmed.
type PurgeInput struct {
	BelegIDs      []string
	ExportIDs     []string
	Grund         string
	Ablaufhemmung bool
	Today         string
}

// RetentionReport lists confirmed receipts and finished exports.
func (s *Store) RetentionReport(ctx context.Context, today string) ([]RetentionItem, error) {
	if today == "" {
		today = BerlinDate(time.Now())
	}
	var items []RetentionItem
	belege, err := s.readQ().ListRetentionBelege(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range belege {
		items = append(items, RetentionItem{
			Art: "beleg", ID: row.ID, NutzerID: row.NutzerID, Bezeichnung: row.Bezeichnung,
			AufbewahrenBis: row.AufbewahrenBis, Abgelaufen: row.AufbewahrenBis < today,
			InhaltGeloescht: row.InhaltGeloeschtAm != nil, SHA256: row.Sha256Original,
		})
	}
	exporte, err := s.readQ().ListRetentionExporte(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range exporte {
		name := row.Bezeichnung
		if row.Version > 0 {
			name = name + " v" + itoa(row.Version)
		}
		items = append(items, RetentionItem{
			Art: "export", ID: row.ID, NutzerID: row.NutzerID, Bezeichnung: name,
			AufbewahrenBis: row.AufbewahrenBis, Abgelaufen: row.AufbewahrenBis != "" && row.AufbewahrenBis < today,
			InhaltGeloescht: row.InhaltGeloeschtAm != nil, SHA256: row.Sha256,
		})
	}
	if items == nil {
		items = []RetentionItem{}
	}
	return items, nil
}

// PurgeRetention deletes file contents after the retention date. Metadata stays in the audit log.
// It returns storage keys the caller must remove.
func (s *Store) PurgeRetention(ctx context.Context, actor Actor, in PurgeInput) ([]string, error) {
	if !in.Ablaufhemmung {
		return nil, &CodeError{Code: "ablaufhemmung"}
	}
	grund := strings.TrimSpace(in.Grund)
	if grund == "" {
		return nil, &CodeError{Code: "grund"}
	}
	if len(in.BelegIDs) == 0 && len(in.ExportIDs) == 0 {
		return nil, &CodeError{Code: "keine_auswahl"}
	}
	today := in.Today
	if today == "" {
		today = BerlinDate(time.Now())
	}
	var keys []string
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		for _, id := range in.BelegIDs {
			row, err := q.GetBelegByID(ctx, id)
			if err != nil {
				return mapErr(err)
			}
			if row.InhaltGeloeschtAm != nil {
				return &CodeError{Code: "bereits_geloescht"}
			}
			if row.AufbewahrenBis == nil || *row.AufbewahrenBis >= today || (row.Status != "bestaetigt" && row.Status != "storniert") {
				return &CodeError{Code: "frist"}
			}
			files, err := q.ListBelegdateien(ctx, id)
			if err != nil {
				return err
			}
			for _, f := range files {
				keys = append(keys, f.SpeicherSchluessel)
			}
			if err := q.DeleteBelegdateienByBeleg(ctx, id); err != nil {
				return err
			}
			if err := q.DeleteBelegtexteByBeleg(ctx, id); err != nil {
				return err
			}
			if err := q.MarkBelegInhaltGeloescht(ctx, sqlitedb.MarkBelegInhaltGeloeschtParams{
				Jetzt: &now, Grund: &grund, ID: id,
			}); err != nil {
				return err
			}
			nummer := ""
			if row.Belegnummer != nil {
				nummer = *row.Belegnummer
			}
			if err := appendAudit(ctx, q, actor, "aufbewahrung.geloescht", "beleg", id, nil, mustJSON(map[string]any{
				"belegnummer": nummer, "sha256": row.Sha256Original, "aufbewahren_bis": deref(row.AufbewahrenBis),
				"geloescht_am": now.Format(time.RFC3339),
			}), &grund); err != nil {
				return err
			}
		}
		for _, id := range in.ExportIDs {
			row, err := q.GetExportByID(ctx, id)
			if err != nil {
				return mapErr(err)
			}
			if row.InhaltGeloeschtAm != nil {
				return &CodeError{Code: "bereits_geloescht"}
			}
			if row.Status != "fertig" || row.AufbewahrenBis == nil || *row.AufbewahrenBis >= today {
				return &CodeError{Code: "frist"}
			}
			if row.PdfSchluessel != nil && *row.PdfSchluessel != "" {
				keys = append(keys, *row.PdfSchluessel)
			}
			if row.ZipSchluessel != nil && *row.ZipSchluessel != "" {
				keys = append(keys, *row.ZipSchluessel)
			}
			sha := ""
			if row.PdfSha256 != nil {
				sha = *row.PdfSha256
			}
			if err := q.MarkExportInhaltGeloescht(ctx, sqlitedb.MarkExportInhaltGeloeschtParams{
				Jetzt: &now, Grund: &grund, ID: id,
			}); err != nil {
				return err
			}
			if err := appendAudit(ctx, q, actor, "aufbewahrung.geloescht", "export", id, nil, mustJSON(map[string]any{
				"sha256": sha, "aufbewahren_bis": deref(row.AufbewahrenBis), "geloescht_am": now.Format(time.RFC3339),
			}), &grund); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// EnsureRetentionReport queues the monthly report when this month has none.
func (s *Store) EnsureRetentionReport(ctx context.Context) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		open, err := q.CountOpenJobsByArt(ctx, "aufbewahrung_bericht")
		if err != nil {
			return err
		}
		if open > 0 {
			return nil
		}
		now := time.Now().UTC()
		done, err := q.CountDoneJobsSince(ctx, sqlitedb.CountDoneJobsSinceParams{
			Art: "aufbewahrung_bericht", Seit: monthStartBerlin(now),
		})
		if err != nil {
			return err
		}
		if done > 0 {
			return nil
		}
		return insertJob(ctx, q, "aufbewahrung_bericht", "bericht", now, now)
	})
}

// FinishRetentionReport stores the monthly counts and schedules the next run.
func (s *Store) FinishRetentionReport(ctx context.Context, jobID string, abgelaufen, offen int) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		if err := appendAudit(ctx, q, Actor{Art: "system"}, "aufbewahrung.bericht", "system", "aufbewahrung", nil, mustJSON(map[string]any{
			"abgelaufen": abgelaufen, "offen": offen, "monat": BerlinDate(now)[:7],
		}), nil); err != nil {
			return err
		}
		if err := q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: now, ID: jobID}); err != nil {
			return err
		}
		return insertJob(ctx, q, "aufbewahrung_bericht", "bericht", nextRetentionAt(now), now)
	})
}

// FailPlainJob marks a non-Beleg job failed and retries it a day later.
func (s *Store) FailPlainJob(ctx context.Context, job sqlitedb.Job, cause error) error {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	payload := "bericht"
	if job.Payload != nil && *job.Payload != "" {
		payload = *job.Payload
	}
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		if err := q.FailJobRow(ctx, sqlitedb.FailJobRowParams{Fehler: &msg, Jetzt: now, ID: job.ID}); err != nil {
			return err
		}
		return insertJob(ctx, q, job.Art, payload, now.Add(24*time.Hour), now)
	})
}

// RetentionDeadline is 31 December of year+8 in Europe/Berlin (SPEC 12.4).
func RetentionDeadline(now time.Time) string {
	loc := berlin()
	return itoa(int64(now.In(loc).Year()+8)) + "-12-31"
}

// BerlinDate is the civil date in Europe/Berlin.
func BerlinDate(now time.Time) string {
	return now.In(berlin()).Format("2006-01-02")
}

func monthStartBerlin(now time.Time) time.Time {
	t := now.In(berlin())
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, berlin())
}

func nextRetentionAt(now time.Time) time.Time {
	t := now.In(berlin())
	return time.Date(t.Year(), t.Month()+1, 1, 6, 0, 0, 0, berlin())
}

func itoa(n int64) string {
	return fmtInt(n)
}

func fmtInt(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
