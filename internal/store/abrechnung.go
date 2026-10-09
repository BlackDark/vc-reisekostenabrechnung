package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// AbrechnungInput is a new or edited Abrechnung (SPEC 3.4).
type AbrechnungInput struct {
	ArbeitgeberID string
	ZeitraumArt   string
	Von           string
	Bis           string
	Titel         string
	Sprache       string
	ReiseIDs      *[]string
	VorschussIDs  *[]string
}

// AbrechnungBundle is one Abrechnung with its selection and the period suggestion.
type AbrechnungBundle struct {
	Row                  sqlitedb.Abrechnung
	ReiseIDs             []string
	VorschussIDs         []string
	VorschlaegeReise     []string
	VorschlaegeVorschuss []string
}

// EinreichenInput is the server-side check result stored with the export job.
type EinreichenInput struct {
	Version   int64
	Blocker   []string
	Warnungen []string
	Quittiert []string
	Snapshot  string
}

// CreateAbrechnung inserts a draft and selects the suggested Reisen and Vorschüsse.
func (s *Store) CreateAbrechnung(ctx context.Context, nutzerID string, in AbrechnungInput, actor Actor) (AbrechnungBundle, error) {
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		ag, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: in.ArbeitgeberID, NutzerID: nutzerID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &CodeError{Code: "arbeitgeber"}
			}
			return err
		}
		if err := validZeitraum(in.ZeitraumArt, in.Von, in.Bis); err != nil {
			return err
		}
		sprache := in.Sprache
		if sprache != "en" {
			sprache = "de"
		}
		titel := strings.TrimSpace(in.Titel)
		if titel == "" {
			titel = defaultTitel(in.ZeitraumArt, in.Von, in.Bis, sprache)
		}
		if utf8.RuneCountInString(titel) > 200 {
			return &CodeError{Code: "titel"}
		}
		now := time.Now().UTC()
		row, err := q.InsertAbrechnung(ctx, sqlitedb.InsertAbrechnungParams{
			ID: id.Must(), NutzerID: nutzerID, ArbeitgeberID: ag.ID, ZeitraumArt: in.ZeitraumArt,
			Von: in.Von, Bis: in.Bis, Titel: titel, ExportSprache: sprache, ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		reisen, advances, err := candidates(ctx, q, row)
		if err != nil {
			return err
		}
		pickedReisen := pick(in.ReiseIDs, reisen)
		pickedAdvances := pick(in.VorschussIDs, advances)
		if err := applyReisen(ctx, q, row, nil, pickedReisen, now); err != nil {
			return err
		}
		if err := applyVorschuesse(ctx, q, row, pickedAdvances, now); err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.angelegt", "abrechnung", row.ID, nil, mustJSON(row), nil); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, row.ID)
		return err
	})
	return out, err
}

// GetAbrechnungBundle loads one Abrechnung for its Nutzer.
func (s *Store) GetAbrechnungBundle(ctx context.Context, nutzerID, abrechnungID string) (AbrechnungBundle, error) {
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		var err error
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// ListAbrechnungen returns a page of Abrechnungen.
func (s *Store) ListAbrechnungen(ctx context.Context, nutzerID string, cursor *string, limit int64) ([]sqlitedb.Abrechnung, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.readQ().ListAbrechnungen(ctx, sqlitedb.ListAbrechnungenParams{
		NutzerID: nutzerID, Cursor: cursor, LimitN: limit,
	})
}

// PatchAbrechnung updates the title and export language of a draft.
func (s *Store) PatchAbrechnung(ctx context.Context, nutzerID, abrechnungID string, version int64, titel, sprache string, actor Actor) (AbrechnungBundle, error) {
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := editable(ctx, q, nutzerID, abrechnungID)
		if err != nil {
			return err
		}
		if prev.Version != version {
			return ErrConflict
		}
		titel = strings.TrimSpace(titel)
		if titel == "" || utf8.RuneCountInString(titel) > 200 {
			return &CodeError{Code: "titel"}
		}
		if sprache != "de" && sprache != "en" {
			return &CodeError{Code: "sprache"}
		}
		row, err := q.UpdateAbrechnungKopf(ctx, sqlitedb.UpdateAbrechnungKopfParams{
			Titel: titel, ExportSprache: sprache, GeaendertAm: time.Now().UTC(),
			ID: abrechnungID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.geaendert", "abrechnung", abrechnungID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// SetAbrechnungReisen replaces the selected Reisen of a draft.
func (s *Store) SetAbrechnungReisen(ctx context.Context, nutzerID, abrechnungID string, version int64, ids []string, actor Actor) (AbrechnungBundle, error) {
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := editable(ctx, q, nutzerID, abrechnungID)
		if err != nil {
			return err
		}
		if prev.Version != version {
			return ErrConflict
		}
		cand, _, err := candidates(ctx, q, prev)
		if err != nil {
			return err
		}
		if !subset(ids, cand) {
			return &CodeError{Code: "reise_zeitraum"}
		}
		now := time.Now().UTC()
		if _, err := q.UpdateAbrechnungKopf(ctx, sqlitedb.UpdateAbrechnungKopfParams{
			Titel: prev.Titel, ExportSprache: prev.ExportSprache, GeaendertAm: now,
			ID: abrechnungID, NutzerID: nutzerID, Version: version,
		}); err != nil {
			return mapUpdateErr(err)
		}
		current, err := q.ListAbrechnungReiseIDs(ctx, abrechnungID)
		if err != nil {
			return err
		}
		if err := applyReisen(ctx, q, prev, current, uniq(ids), now); err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.reisen", "abrechnung", abrechnungID, mustJSON(current), mustJSON(ids), nil); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// SetAbrechnungVorschuesse replaces the settled advances of a draft.
func (s *Store) SetAbrechnungVorschuesse(ctx context.Context, nutzerID, abrechnungID string, version int64, ids []string, actor Actor) (AbrechnungBundle, error) {
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := editable(ctx, q, nutzerID, abrechnungID)
		if err != nil {
			return err
		}
		if prev.Version != version {
			return ErrConflict
		}
		_, cand, err := candidates(ctx, q, prev)
		if err != nil {
			return err
		}
		if !subset(ids, cand) {
			return &CodeError{Code: "vorschuss_arbeitgeber"}
		}
		now := time.Now().UTC()
		if _, err := q.UpdateAbrechnungKopf(ctx, sqlitedb.UpdateAbrechnungKopfParams{
			Titel: prev.Titel, ExportSprache: prev.ExportSprache, GeaendertAm: now,
			ID: abrechnungID, NutzerID: nutzerID, Version: version,
		}); err != nil {
			return mapUpdateErr(err)
		}
		if err := applyVorschuesse(ctx, q, prev, uniq(ids), now); err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.vorschuesse", "abrechnung", abrechnungID, nil, mustJSON(ids), nil); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// DeleteAbrechnung removes a draft that was never exported.
func (s *Store) DeleteAbrechnung(ctx context.Context, nutzerID, abrechnungID string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.EinreichungLaeuft || prev.Status != "entwurf" || prev.AktuelleExportVersion != 0 {
			return &CodeError{Code: "uebergang"}
		}
		now := time.Now().UTC()
		ids, err := q.ListAbrechnungReiseIDs(ctx, abrechnungID)
		if err != nil {
			return err
		}
		for _, reiseID := range ids {
			if err := q.SetReiseStatus(ctx, sqlitedb.SetReiseStatusParams{
				Status: "offen", GeaendertAm: now, ID: reiseID, NutzerID: nutzerID,
			}); err != nil {
				return err
			}
		}
		if err := q.ReleaseVorschuesse(ctx, sqlitedb.ReleaseVorschuesseParams{
			GeaendertAm: now, NutzerID: nutzerID, AbrechnungID: &abrechnungID,
		}); err != nil {
			return err
		}
		n, err := q.DeleteAbrechnungEntwurf(ctx, sqlitedb.DeleteAbrechnungEntwurfParams{ID: abrechnungID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		if n == 0 {
			return &CodeError{Code: "uebergang"}
		}
		return appendAudit(ctx, q, actor, "abrechnung.geloescht", "abrechnung", abrechnungID, mustJSON(prev), nil, nil)
	})
}

// Einreichen locks the selected Reisen and queues export version n+1.
// The status stays entwurf until the export finishes (einreichung_laeuft).
func (s *Store) Einreichen(ctx context.Context, nutzerID, abrechnungID string, in EinreichenInput, actor Actor) (AbrechnungBundle, error) {
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := editable(ctx, q, nutzerID, abrechnungID)
		if err != nil {
			return err
		}
		if prev.Version != in.Version {
			return ErrConflict
		}
		ids, err := q.ListAbrechnungReiseIDs(ctx, abrechnungID)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return &CodeError{Code: "keine_reise"}
		}
		if len(in.Blocker) > 0 {
			return &CodeError{Code: "blocker"}
		}
		have := map[string]struct{}{}
		for _, key := range in.Quittiert {
			have[key] = struct{}{}
		}
		for _, key := range in.Warnungen {
			if _, ok := have[key]; !ok {
				return &CodeError{Code: "warnung_offen"}
			}
		}
		if strings.TrimSpace(in.Snapshot) == "" {
			return &CodeError{Code: "snapshot"}
		}
		nummer := prev.Abrechnungsnummer
		if nummer == nil || *nummer == "" {
			ag, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: prev.ArbeitgeberID, NutzerID: nutzerID})
			if err != nil {
				return err
			}
			year := yearOf(prev.Bis)
			n, err := q.NextAbrechnungsnummer(ctx, sqlitedb.NextAbrechnungsnummerParams{
				NutzerID: nutzerID, ArbeitgeberID: prev.ArbeitgeberID, Jahr: int64(year),
			})
			if err != nil {
				return err
			}
			prefix := ag.AbrechnungsnummerPraefix
			if prefix == "" {
				prefix = "RK"
			}
			formatted := fmt.Sprintf("%s-%d-%03d", prefix, year, n)
			nummer = &formatted
		}
		now := time.Now().UTC()
		row, err := q.MarkEinreichung(ctx, sqlitedb.MarkEinreichungParams{
			Abrechnungsnummer: nummer, QuittierteWarnungen: mustJSONString(in.Quittiert),
			GeaendertAm: now, ID: abrechnungID, NutzerID: nutzerID, Version: in.Version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		for _, reiseID := range ids {
			if err := q.SetReiseStatus(ctx, sqlitedb.SetReiseStatusParams{
				Status: "gesperrt", GeaendertAm: now, ID: reiseID, NutzerID: nutzerID,
			}); err != nil {
				return err
			}
		}
		ver, err := q.MaxExportVersion(ctx, abrechnungID)
		if err != nil {
			return err
		}
		anlass := "einreichung"
		if prev.AktuelleExportVersion > 0 {
			anlass = "einreichung_nach_entsperrung"
		}
		exp, err := q.InsertExport(ctx, sqlitedb.InsertExportParams{
			ID: id.Must(), AbrechnungID: abrechnungID, NutzerID: nutzerID, Version: ver + 1,
			Anlass: anlass, ErstelltAm: now, Snapshot: in.Snapshot,
		})
		if err != nil {
			return err
		}
		if err := insertJob(ctx, q, "export", exp.ID, now, now); err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.einreichen", "abrechnung", abrechnungID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// CompleteExport marks the export finished and the Abrechnung eingereicht.
func (s *Store) CompleteExport(ctx context.Context, jobID, exportID, pdfKey, pdfSHA, zipKey, zipSHA, snapshot string) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		exp, err := q.GetExportByID(ctx, exportID)
		if err != nil {
			return mapErr(err)
		}
		if exp.Status == "fertig" {
			return q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: time.Now().UTC(), ID: jobID})
		}
		now := time.Now().UTC()
		if snapshot == "" {
			snapshot = exp.Snapshot
		}
		done, err := q.FinishExportRow(ctx, sqlitedb.FinishExportRowParams{
			PdfSchluessel: &pdfKey, PdfSha256: &pdfSHA, ZipSchluessel: &zipKey, ZipSha256: &zipSHA,
			Snapshot: snapshot, ID: exportID,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := q.MarkExportErsetzt(ctx, sqlitedb.MarkExportErsetztParams{
			ErsetztDurchVersion: &done.Version, AbrechnungID: exp.AbrechnungID, ID: exportID,
		}); err != nil {
			return err
		}
		row, err := q.FinishEinreichung(ctx, sqlitedb.FinishEinreichungParams{
			EingereichtAm: &now, AktuelleExportVersion: done.Version, GeaendertAm: now, ID: exp.AbrechnungID,
		})
		if err != nil {
			return err
		}
		if err := q.FinishJob(ctx, sqlitedb.FinishJobParams{Jetzt: now, ID: jobID}); err != nil {
			return err
		}
		return appendAudit(ctx, q, Actor{Art: "system", NutzerID: &exp.NutzerID}, "abrechnung.eingereicht", "abrechnung", exp.AbrechnungID, nil, mustJSON(row), nil)
	})
}

// FailExportJob records a failed export and unlocks the Reisen again.
func (s *Store) FailExportJob(ctx context.Context, job sqlitedb.Job, cause error) error {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	exportID := ""
	if job.Payload != nil {
		exportID = *job.Payload
	}
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		if err := q.FailJobRow(ctx, sqlitedb.FailJobRowParams{Fehler: &msg, Jetzt: now, ID: job.ID}); err != nil {
			return err
		}
		if exportID == "" {
			return nil
		}
		exp, err := q.GetExportByID(ctx, exportID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if exp.Status != "in_erstellung" {
			return nil
		}
		if err := q.FailExportRow(ctx, sqlitedb.FailExportRowParams{Fehler: &msg, ID: exportID}); err != nil {
			return err
		}
		if _, err := q.RollbackEinreichung(ctx, sqlitedb.RollbackEinreichungParams{
			EinreichungFehler: &msg, GeaendertAm: now, ID: exp.AbrechnungID,
		}); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		ids, err := q.ListAbrechnungReiseIDs(ctx, exp.AbrechnungID)
		if err != nil {
			return err
		}
		for _, reiseID := range ids {
			if err := q.SetReiseStatus(ctx, sqlitedb.SetReiseStatusParams{
				Status: "in_entwurf", GeaendertAm: now, ID: reiseID, NutzerID: exp.NutzerID,
			}); err != nil {
				return err
			}
		}
		return appendAudit(ctx, q, Actor{Art: "system", NutzerID: &exp.NutzerID}, "abrechnung.export_fehlgeschlagen", "abrechnung", exp.AbrechnungID, nil, nil, &msg)
	})
}

// Entsperren returns an eingereicht Abrechnung to entwurf and writes the reason.
func (s *Store) Entsperren(ctx context.Context, nutzerID, abrechnungID string, version int64, grund string, actor Actor) (AbrechnungBundle, error) {
	grund = strings.TrimSpace(grund)
	if utf8.RuneCountInString(grund) < 10 {
		return AbrechnungBundle{}, &CodeError{Code: "grund"}
	}
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Status != "eingereicht" || prev.EinreichungLaeuft {
			return &CodeError{Code: "uebergang"}
		}
		if prev.Version != version {
			return ErrConflict
		}
		now := time.Now().UTC()
		row, err := q.MarkEntsperrt(ctx, sqlitedb.MarkEntsperrtParams{
			GeaendertAm: now, ID: abrechnungID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		ids, err := q.ListAbrechnungReiseIDs(ctx, abrechnungID)
		if err != nil {
			return err
		}
		for _, reiseID := range ids {
			if err := q.SetReiseStatus(ctx, sqlitedb.SetReiseStatusParams{
				Status: "in_entwurf", GeaendertAm: now, ID: reiseID, NutzerID: nutzerID,
			}); err != nil {
				return err
			}
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.entsperrt", "abrechnung", abrechnungID, mustJSON(prev), mustJSON(row), &grund); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// MarkBezahlt records that the employer paid an eingereicht Abrechnung.
func (s *Store) MarkBezahlt(ctx context.Context, nutzerID, abrechnungID string, version int64, datum, vermerk string, actor Actor) (AbrechnungBundle, error) {
	if _, err := time.Parse("2006-01-02", datum); err != nil {
		return AbrechnungBundle{}, &CodeError{Code: "bezahlt_datum"}
	}
	today := time.Now().In(berlin()).Format("2006-01-02")
	if datum > today {
		return AbrechnungBundle{}, &CodeError{Code: "bezahlt_datum"}
	}
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Status != "eingereicht" {
			return &CodeError{Code: "uebergang"}
		}
		if prev.Version != version {
			return ErrConflict
		}
		row, err := q.MarkBezahlt(ctx, sqlitedb.MarkBezahltParams{
			BezahltAm: &datum, BezahltVermerk: emptyNil(strings.TrimSpace(vermerk)),
			GeaendertAm: time.Now().UTC(), ID: abrechnungID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.bezahlt", "abrechnung", abrechnungID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// BezahltZuruecknehmen returns a paid Abrechnung to eingereicht.
func (s *Store) BezahltZuruecknehmen(ctx context.Context, nutzerID, abrechnungID string, version int64, grund string, actor Actor) (AbrechnungBundle, error) {
	grund = strings.TrimSpace(grund)
	if grund == "" {
		return AbrechnungBundle{}, &CodeError{Code: "grund"}
	}
	var out AbrechnungBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Status != "bezahlt" {
			return &CodeError{Code: "uebergang"}
		}
		if prev.Version != version {
			return ErrConflict
		}
		row, err := q.MarkBezahltZurueck(ctx, sqlitedb.MarkBezahltZurueckParams{
			GeaendertAm: time.Now().UTC(), ID: abrechnungID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "abrechnung.bezahlt_zurueck", "abrechnung", abrechnungID, mustJSON(prev), mustJSON(row), &grund); err != nil {
			return err
		}
		out, err = loadAbrechnung(ctx, q, nutzerID, abrechnungID)
		return err
	})
	return out, err
}

// ListExporte returns every export version of an Abrechnung.
func (s *Store) ListExporte(ctx context.Context, nutzerID, abrechnungID string) ([]sqlitedb.Export, error) {
	if _, err := s.readQ().GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID}); err != nil {
		return nil, mapErr(err)
	}
	return s.readQ().ListExporte(ctx, sqlitedb.ListExporteParams{AbrechnungID: abrechnungID, NutzerID: nutzerID})
}

// GetExport loads one export for its Nutzer.
func (s *Store) GetExport(ctx context.Context, nutzerID, exportID string) (sqlitedb.Export, error) {
	row, err := s.readQ().GetExport(ctx, sqlitedb.GetExportParams{ID: exportID, NutzerID: nutzerID})
	return row, mapErr(err)
}

// GetExportByID loads an export for the job worker.
func (s *Store) GetExportByID(ctx context.Context, exportID string) (sqlitedb.Export, error) {
	row, err := s.readQ().GetExportByID(ctx, exportID)
	return row, mapErr(err)
}

func editable(ctx context.Context, q *sqlitedb.Queries, nutzerID, abrechnungID string) (sqlitedb.Abrechnung, error) {
	row, err := q.GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID})
	if err != nil {
		return sqlitedb.Abrechnung{}, mapErr(err)
	}
	if row.EinreichungLaeuft {
		return sqlitedb.Abrechnung{}, &CodeError{Code: "einreichung_laeuft"}
	}
	if row.Status != "entwurf" {
		return sqlitedb.Abrechnung{}, &CodeError{Code: "uebergang"}
	}
	return row, nil
}

func loadAbrechnung(ctx context.Context, q *sqlitedb.Queries, nutzerID, abrechnungID string) (AbrechnungBundle, error) {
	row, err := q.GetAbrechnung(ctx, sqlitedb.GetAbrechnungParams{ID: abrechnungID, NutzerID: nutzerID})
	if err != nil {
		return AbrechnungBundle{}, mapErr(err)
	}
	ids, err := q.ListAbrechnungReiseIDs(ctx, abrechnungID)
	if err != nil {
		return AbrechnungBundle{}, err
	}
	adv, err := q.ListVorschuesseOfAbrechnung(ctx, sqlitedb.ListVorschuesseOfAbrechnungParams{
		NutzerID: nutzerID, AbrechnungID: &abrechnungID,
	})
	if err != nil {
		return AbrechnungBundle{}, err
	}
	vorschussIDs := make([]string, 0, len(adv))
	for _, v := range adv {
		vorschussIDs = append(vorschussIDs, v.ID)
	}
	reisen, advances, err := candidates(ctx, q, row)
	if err != nil {
		return AbrechnungBundle{}, err
	}
	return AbrechnungBundle{
		Row: row, ReiseIDs: ids, VorschussIDs: vorschussIDs,
		VorschlaegeReise: reisen, VorschlaegeVorschuss: advances,
	}, nil
}

func candidates(ctx context.Context, q *sqlitedb.Queries, row sqlitedb.Abrechnung) ([]string, []string, error) {
	open, err := q.ListReisenByArbeitgeberStatus(ctx, sqlitedb.ListReisenByArbeitgeberStatusParams{
		NutzerID: row.NutzerID, ArbeitgeberID: row.ArbeitgeberID, Status: "offen",
	})
	if err != nil {
		return nil, nil, err
	}
	draft, err := q.ListReisenByArbeitgeberStatus(ctx, sqlitedb.ListReisenByArbeitgeberStatusParams{
		NutzerID: row.NutzerID, ArbeitgeberID: row.ArbeitgeberID, Status: "in_entwurf",
	})
	if err != nil {
		return nil, nil, err
	}
	selected, err := q.ListAbrechnungReiseIDs(ctx, row.ID)
	if err != nil {
		return nil, nil, err
	}
	own := map[string]struct{}{}
	for _, id := range selected {
		own[id] = struct{}{}
	}
	var reisen []string
	for _, trip := range append(open, draft...) {
		if trip.Status == "in_entwurf" {
			if _, ok := own[trip.ID]; !ok {
				continue
			}
		}
		ok, err := endInPeriod(trip, row.Von, row.Bis)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			reisen = append(reisen, trip.ID)
		}
	}
	rows, err := q.ListVorschuesseByArbeitgeber(ctx, sqlitedb.ListVorschuesseByArbeitgeberParams{
		NutzerID: row.NutzerID, ArbeitgeberID: row.ArbeitgeberID,
	})
	if err != nil {
		return nil, nil, err
	}
	var advances []string
	for _, v := range rows {
		if v.Datum > row.Bis {
			continue
		}
		if v.AbrechnungID != nil && *v.AbrechnungID != row.ID {
			continue
		}
		advances = append(advances, v.ID)
	}
	return reisen, advances, nil
}

func applyReisen(ctx context.Context, q *sqlitedb.Queries, row sqlitedb.Abrechnung, current, want []string, now time.Time) error {
	for _, reiseID := range current {
		if err := q.SetReiseStatus(ctx, sqlitedb.SetReiseStatusParams{
			Status: "offen", GeaendertAm: now, ID: reiseID, NutzerID: row.NutzerID,
		}); err != nil {
			return err
		}
	}
	if err := q.DeleteAbrechnungReisen(ctx, row.ID); err != nil {
		return err
	}
	for _, reiseID := range want {
		trip, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: row.NutzerID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &CodeError{Code: "reise_zeitraum"}
			}
			return err
		}
		if trip.ArbeitgeberID != row.ArbeitgeberID {
			return &CodeError{Code: "reise_arbeitgeber"}
		}
		if trip.Status != "offen" && trip.Status != "in_entwurf" {
			return &CodeError{Code: "reise_gesperrt"}
		}
		ok, err := endInPeriod(trip, row.Von, row.Bis)
		if err != nil {
			return err
		}
		if !ok {
			return &CodeError{Code: "reise_zeitraum"}
		}
		if err := q.InsertAbrechnungReise(ctx, sqlitedb.InsertAbrechnungReiseParams{
			AbrechnungID: row.ID, ReiseID: reiseID,
		}); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return &CodeError{Code: "reise_belegt"}
			}
			return err
		}
		if err := q.SetReiseStatus(ctx, sqlitedb.SetReiseStatusParams{
			Status: "in_entwurf", GeaendertAm: now, ID: reiseID, NutzerID: row.NutzerID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyVorschuesse(ctx context.Context, q *sqlitedb.Queries, row sqlitedb.Abrechnung, want []string, now time.Time) error {
	if err := q.ReleaseVorschuesse(ctx, sqlitedb.ReleaseVorschuesseParams{
		GeaendertAm: now, NutzerID: row.NutzerID, AbrechnungID: &row.ID,
	}); err != nil {
		return err
	}
	for _, vid := range want {
		got, err := q.GetVorschuss(ctx, sqlitedb.GetVorschussParams{ID: vid, NutzerID: row.NutzerID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &CodeError{Code: "vorschuss_arbeitgeber"}
			}
			return err
		}
		if got.ArbeitgeberID != row.ArbeitgeberID {
			return &CodeError{Code: "vorschuss_arbeitgeber"}
		}
		if got.Datum > row.Bis {
			return &CodeError{Code: "vorschuss_datum"}
		}
		if _, err := q.ClaimVorschuss(ctx, sqlitedb.ClaimVorschussParams{
			AbrechnungID: &row.ID, GeaendertAm: now, ID: vid, NutzerID: row.NutzerID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &CodeError{Code: "vorschuss_belegt"}
			}
			return err
		}
	}
	return nil
}

func endInPeriod(trip sqlitedb.Reise, von, bis string) (bool, error) {
	loc, err := time.LoadLocation(trip.EndeZone)
	if err != nil {
		return false, &CodeError{Code: "zeitzone"}
	}
	day := trip.Ende.In(loc).Format("2006-01-02")
	return day >= von && day <= bis, nil
}

func validZeitraum(art, von, bis string) error {
	a, errA := time.Parse("2006-01-02", von)
	b, errB := time.Parse("2006-01-02", bis)
	if errA != nil || errB != nil || a.After(b) {
		return &CodeError{Code: "zeitraum"}
	}
	switch art {
	case "tag":
		if von != bis {
			return &CodeError{Code: "zeitraum"}
		}
	case "woche":
		if a.Weekday() != time.Monday || b.Weekday() != time.Sunday || b.Sub(a) != 6*24*time.Hour {
			return &CodeError{Code: "zeitraum"}
		}
	case "monat":
		if a.Day() != 1 || a.Month() != b.Month() || a.Year() != b.Year() || b.AddDate(0, 0, 1).Day() != 1 {
			return &CodeError{Code: "zeitraum"}
		}
	case "quartal":
		if a.Day() != 1 || (int(a.Month())-1)%3 != 0 || !a.AddDate(0, 3, -1).Equal(b) {
			return &CodeError{Code: "zeitraum"}
		}
	case "frei":
	default:
		return &CodeError{Code: "zeitraum"}
	}
	return nil
}

func defaultTitel(art, von, bis, sprache string) string {
	a, _ := time.Parse("2006-01-02", von)
	de := [...]string{"", "Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}
	en := [...]string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	if art == "monat" && sprache == "en" {
		return fmt.Sprintf("Travel expenses %s %d", en[a.Month()], a.Year())
	}
	if art == "monat" {
		return fmt.Sprintf("Reisekosten %s %d", de[a.Month()], a.Year())
	}
	if sprache == "en" {
		return fmt.Sprintf("Travel expenses %s – %s", von, bis)
	}
	return fmt.Sprintf("Reisekosten %s – %s", von, bis)
}

func pick(explicit *[]string, all []string) []string {
	if explicit == nil {
		return append([]string{}, all...)
	}
	return uniq(*explicit)
}

func uniq(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func subset(ids, allowed []string) bool {
	ok := map[string]struct{}{}
	for _, id := range allowed {
		ok[id] = struct{}{}
	}
	for _, id := range ids {
		if _, found := ok[id]; !found {
			return false
		}
	}
	return true
}

func yearOf(iso string) int {
	if len(iso) >= 4 {
		var y int
		_, _ = fmt.Sscanf(iso[:4], "%d", &y)
		if y > 0 {
			return y
		}
	}
	return time.Now().UTC().Year()
}

func berlin() *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.UTC
	}
	return loc
}

func mustJSONString(v any) string {
	return string(mustJSON(v))
}

// WarnKey identifies one warning for the quittance list.
func WarnKey(code, objektID string) string {
	return code + "\t" + objektID
}
