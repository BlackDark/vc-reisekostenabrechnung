package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// ArbeitgeberInput is a create or update of one Arbeitgeber.
type ArbeitgeberInput struct {
	Name         string
	Anschrift    string
	UstID        *string
	Steuernummer *string
	Praefix      string
	IstStandard  bool
	Archiviert   bool
}

// TaetigkeitInput is a create or update of one Tätigkeitsstätte.
type TaetigkeitInput struct {
	Bezeichnung string
	Anschrift   string
	LandISO     string
	Satzort     string
	Kunde       *string
}

// SeedSatztabellen inserts the shipped 2024–2026 tables once.
func (s *Store) SeedSatztabellen(ctx context.Context) error {
	cat, err := satz.Load()
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		for _, year := range []int{2024, 2025, 2026} {
			_, err := q.GetSatztabelle(ctx, int64(year))
			if err == nil {
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err := insertYear(ctx, q, cat.Years[year], "aktiv", cat.Years[year].Inland.Quelle); err != nil {
				return err
			}
		}
		return nil
	})
}

func insertYear(ctx context.Context, q *sqlitedb.Queries, y *satz.Year, status, quelle string) error {
	now := time.Now().UTC()
	ersatz, err := json.Marshal(y.Inland.Ersatz)
	if err != nil {
		return err
	}
	in := y.Inland
	if _, err := q.InsertSatztabelle(ctx, sqlitedb.InsertSatztabelleParams{
		Jahr: int64(y.Jahr), Status: status, Quelle: quelle,
		VmaInland24h: in.VMA24h, VmaInland8h: in.VMA8h,
		KuerzungFruehstueckPct: in.KuerzungFruehstueckPct, KuerzungHauptmahlzeitPct: in.KuerzungHauptmahlzeitPct,
		UebernachtungInlandPauschale: in.Uebernachtung, KmKraftwagen: in.KmKraftwagen, KmAnderesMotorfahrzeug: in.KmAnderes,
		SachbezugFruehstueck: in.SachbezugFruehstueck, SachbezugHauptmahlzeit: in.SachbezugHauptmahlzeit,
		UeblicheMahlzeitGrenze: in.UeblicheMahlzeit, Kleinbetragsgrenze: in.Kleinbetrag, BewirtungAbzugPct: in.BewirtungAbzug,
		UstSaetze: in.UstSaetze, AufbewahrungJahre: in.AufbewahrungJahre, Ersatzlaender: string(ersatz),
		FlugZwischentageLand: in.Flug, SchiffLand: in.Schiff, ErstelltAm: now, GeaendertAm: now,
	}); err != nil {
		return err
	}
	for _, row := range y.Rows {
		if err := q.InsertAuslandssatz(ctx, sqlitedb.InsertAuslandssatzParams{
			ID: id.Must(), Jahr: int64(y.Jahr), LandIso: row.LandISO, LandNameDe: row.LandNameDE,
			Satzort: row.Satzort, OrtName: row.OrtName, Vma24h: row.VMA24h, Vma8h: row.VMA8h, Uebernachtung: row.Uebernachtung,
		}); err != nil {
			return err
		}
	}
	return nil
}

// EffectiveYear loads one Satztabelle with Admin overrides applied.
func (s *Store) EffectiveYear(ctx context.Context, jahr int) (*satz.Year, error) {
	var y *satz.Year
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		var err error
		y, err = loadEffective(ctx, q, jahr)
		return err
	})
	return y, err
}

func loadEffective(ctx context.Context, q *sqlitedb.Queries, jahr int) (*satz.Year, error) {
	row, err := q.GetSatztabelle(ctx, int64(jahr))
	if err != nil {
		return nil, mapErr(err)
	}
	rows, err := q.ListAuslandssaetze(ctx, int64(jahr))
	if err != nil {
		return nil, err
	}
	overrides, err := q.ListSatzOverrides(ctx, int64(jahr))
	if err != nil {
		return nil, err
	}
	y := yearFromRow(row, rows)
	for _, o := range overrides {
		if err := applyOverride(y, o); err != nil {
			return nil, err
		}
	}
	y.Index()
	return y, nil
}

func yearFromRow(row sqlitedb.Satztabelle, abroad []sqlitedb.Auslandssatz) *satz.Year {
	ersatz := map[string]string{}
	_ = json.Unmarshal([]byte(row.Ersatzlaender), &ersatz)
	y := &satz.Year{
		Jahr: int(row.Jahr), Status: row.Status,
		Inland: satz.Inland{
			VMA24h: row.VmaInland24h, VMA8h: row.VmaInland8h,
			KuerzungFruehstueckPct: row.KuerzungFruehstueckPct, KuerzungHauptmahlzeitPct: row.KuerzungHauptmahlzeitPct,
			Uebernachtung: row.UebernachtungInlandPauschale, KmKraftwagen: row.KmKraftwagen, KmAnderes: row.KmAnderesMotorfahrzeug,
			SachbezugFruehstueck: row.SachbezugFruehstueck, SachbezugHauptmahlzeit: row.SachbezugHauptmahlzeit,
			UeblicheMahlzeit: row.UeblicheMahlzeitGrenze, Kleinbetrag: row.Kleinbetragsgrenze, BewirtungAbzug: row.BewirtungAbzugPct,
			AufbewahrungJahre: row.AufbewahrungJahre, UstSaetze: row.UstSaetze, Ersatz: ersatz,
			Flug: row.FlugZwischentageLand, Schiff: row.SchiffLand, Quelle: row.Quelle,
		},
	}
	for _, a := range abroad {
		y.Rows = append(y.Rows, satz.Row{
			LandISO: a.LandIso, LandNameDE: a.LandNameDe, Satzort: a.Satzort, OrtName: a.OrtName,
			VMA24h: a.Vma24h, VMA8h: a.Vma8h, Uebernachtung: a.Uebernachtung,
		})
	}
	y.Index()
	return y
}

func applyOverride(y *satz.Year, o sqlitedb.SatzOverride) error {
	n, err := strconv.ParseInt(o.NeuerWert, 10, 64)
	if err != nil || n < 0 || n > 100_000_000 {
		return fmt.Errorf("override %s: %w", o.Feld, ErrInvalid)
	}
	land, ort := "", ""
	if o.LandIso != nil {
		land = *o.LandIso
	}
	if o.Satzort != nil {
		ort = *o.Satzort
	}
	if land == "" {
		if !satz.SetInlandField(&y.Inland, o.Feld, n) {
			return fmt.Errorf("override field %s: %w", o.Feld, ErrInvalid)
		}
		return nil
	}
	for i := range y.Rows {
		if y.Rows[i].LandISO == land && y.Rows[i].Satzort == ort {
			switch o.Feld {
			case "vma_24h":
				y.Rows[i].VMA24h = n
			case "vma_8h":
				y.Rows[i].VMA8h = n
			case "uebernachtung":
				y.Rows[i].Uebernachtung = n
			default:
				return fmt.Errorf("override field %s: %w", o.Feld, ErrInvalid)
			}
			return nil
		}
	}
	return nil
}

// Satz resolves a rate for a calendar year, including overrides and fallbacks.
func (s *Store) Satz(ctx context.Context, jahr int, land, satzort string) (satz.Hit, error) {
	y, err := s.EffectiveYear(ctx, jahr)
	if err != nil {
		return satz.Hit{}, err
	}
	return y.Satz(land, satzort)
}

// ListSatztabellen returns every year header.
func (s *Store) ListSatztabellen(ctx context.Context) ([]sqlitedb.Satztabelle, error) {
	return s.readQ().ListSatztabellen(ctx)
}

// GetSatztabelle returns the stored header without applying overrides.
func (s *Store) GetSatztabelle(ctx context.Context, jahr int) (sqlitedb.Satztabelle, error) {
	row, err := s.readQ().GetSatztabelle(ctx, int64(jahr))
	return row, mapErr(err)
}

// ListLaender returns the distinct countries of one year.
func (s *Store) ListLaender(ctx context.Context, jahr int) ([]sqlitedb.ListLaenderRow, error) {
	return s.readQ().ListLaender(ctx, int64(jahr))
}

// ListAusland returns the shipped rows. Effective values come from EffectiveYear.
func (s *Store) ListAusland(ctx context.Context, jahr int) ([]sqlitedb.Auslandssatz, error) {
	return s.readQ().ListAuslandssaetze(ctx, int64(jahr))
}

// ListOverrides returns the append-only override log for one year.
func (s *Store) ListOverrides(ctx context.Context, jahr int) ([]sqlitedb.SatzOverride, error) {
	return s.readQ().ListSatzOverrides(ctx, int64(jahr))
}

// OverrideSatztabelle records one Admin override and bumps the year version.
func (s *Store) OverrideSatztabelle(ctx context.Context, jahr int, version int64, land, ort, feld string, neu int64, grund string, actor Actor) (sqlitedb.Satztabelle, error) {
	var out sqlitedb.Satztabelle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		y, err := loadEffective(ctx, q, jahr)
		if err != nil {
			return err
		}
		alt, err := currentCents(y, land, ort, feld)
		if err != nil {
			return err
		}
		row, err := q.GetSatztabelle(ctx, int64(jahr))
		if err != nil {
			return mapErr(err)
		}
		if row.Version != version {
			return ErrConflict
		}
		now := time.Now().UTC()
		var landPtr, ortPtr *string
		if land != "" {
			landPtr = &land
			ortPtr = &ort
		}
		ov, err := q.InsertSatzOverride(ctx, sqlitedb.InsertSatzOverrideParams{
			ID: id.Must(), Jahr: int64(jahr), LandIso: landPtr, Satzort: ortPtr, Feld: feld,
			AlterWert: strconv.FormatInt(alt, 10), NeuerWert: strconv.FormatInt(neu, 10),
			Grund: grund, AdminID: deref(actor.NutzerID), Zeitpunkt: now,
		})
		if err != nil {
			return err
		}
		updated, err := q.TouchSatztabelle(ctx, sqlitedb.TouchSatztabelleParams{
			Status: row.Status, GeaendertAm: now, Jahr: int64(jahr), Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		out = updated
		return appendAudit(ctx, q, actor, "satztabelle.override", "satztabelle", strconv.Itoa(jahr),
			mustJSON(map[string]any{"feld": feld, "wert": alt}),
			mustJSON(map[string]any{"feld": feld, "wert": neu, "land_iso": land, "satzort": ort, "override_id": ov.ID}),
			&grund)
	})
	return out, err
}

func currentCents(y *satz.Year, land, ort, feld string) (int64, error) {
	if land == "" {
		n, ok := satz.InlandField(y.Inland, feld)
		if !ok {
			return 0, ErrInvalid
		}
		return n, nil
	}
	for _, row := range y.Rows {
		if row.LandISO == land && row.Satzort == ort {
			switch feld {
			case "vma_24h":
				return row.VMA24h, nil
			case "vma_8h":
				return row.VMA8h, nil
			case "uebernachtung":
				return row.Uebernachtung, nil
			default:
				return 0, ErrInvalid
			}
		}
	}
	return 0, ErrNotFound
}

// ImportSatztabelle replaces one year's Auslandssätze and leaves the year as Entwurf.
func (s *Store) ImportSatztabelle(ctx context.Context, jahr int, rows []satz.Row, actor Actor) (sqlitedb.Satztabelle, error) {
	var out sqlitedb.Satztabelle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		existing, err := q.GetSatztabelle(ctx, int64(jahr))
		if errors.Is(err, sql.ErrNoRows) {
			in := satz.InlandFor(jahr)
			y := &satz.Year{Jahr: jahr, Inland: in, Rows: rows}
			if err := insertYear(ctx, q, y, "entwurf", "CSV-Import"); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			if err := q.DeleteAuslandssaetze(ctx, int64(jahr)); err != nil {
				return err
			}
			if err := q.DeleteSatzOverrides(ctx, int64(jahr)); err != nil {
				return err
			}
			for _, row := range rows {
				if err := q.InsertAuslandssatz(ctx, sqlitedb.InsertAuslandssatzParams{
					ID: id.Must(), Jahr: int64(jahr), LandIso: row.LandISO, LandNameDe: row.LandNameDE,
					Satzort: row.Satzort, OrtName: row.OrtName, Vma24h: row.VMA24h, Vma8h: row.VMA8h, Uebernachtung: row.Uebernachtung,
				}); err != nil {
					return err
				}
			}
			if _, err := q.TouchSatztabelle(ctx, sqlitedb.TouchSatztabelleParams{
				Status: "entwurf", GeaendertAm: now, Jahr: int64(jahr), Version: existing.Version,
			}); err != nil {
				return err
			}
		}
		out, err = q.GetSatztabelle(ctx, int64(jahr))
		if err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "satztabelle.import", "satztabelle", strconv.Itoa(jahr), nil,
			mustJSON(map[string]any{"zeilen": len(rows), "status": "entwurf"}), nil)
	})
	return out, err
}

// ActivateSatztabelle marks a year aktiv.
func (s *Store) ActivateSatztabelle(ctx context.Context, jahr int, actor Actor) (sqlitedb.Satztabelle, error) {
	var out sqlitedb.Satztabelle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		row, err := q.GetSatztabelle(ctx, int64(jahr))
		if err != nil {
			return mapErr(err)
		}
		if row.Status == "aktiv" {
			out = row
			return nil
		}
		updated, err := q.TouchSatztabelle(ctx, sqlitedb.TouchSatztabelleParams{
			Status: "aktiv", GeaendertAm: time.Now().UTC(), Jahr: int64(jahr), Version: row.Version,
		})
		if err != nil {
			return err
		}
		out = updated
		return appendAudit(ctx, q, actor, "satztabelle.aktiviert", "satztabelle", strconv.Itoa(jahr), nil,
			mustJSON(map[string]string{"status": "aktiv"}), nil)
	})
	return out, err
}

// CreateArbeitgeber inserts an Arbeitgeber and keeps exactly one Standard.
func (s *Store) CreateArbeitgeber(ctx context.Context, nutzerID string, in ArbeitgeberInput, actor Actor) (sqlitedb.Arbeitgeber, error) {
	var out sqlitedb.Arbeitgeber
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		n, err := q.CountAktiveArbeitgeber(ctx, nutzerID)
		if err != nil {
			return err
		}
		standard := (in.IstStandard || n == 0) && !in.Archiviert
		now := time.Now().UTC()
		newID := id.Must()
		if standard {
			if err := demoteStandards(ctx, q, nutzerID, newID, now, actor); err != nil {
				return err
			}
		}
		row, err := q.InsertArbeitgeber(ctx, sqlitedb.InsertArbeitgeberParams{
			ID: newID, NutzerID: nutzerID, Name: in.Name, Anschrift: in.Anschrift,
			UstID: in.UstID, Steuernummer: in.Steuernummer, IstStandard: standard,
			Konstellation: "arbeitgebererstattung", AbrechnungsnummerPraefix: in.Praefix,
			Archiviert: in.Archiviert, ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		out = row
		return appendAudit(ctx, q, actor, "arbeitgeber.angelegt", "arbeitgeber", row.ID, nil, agJSON(row), nil)
	})
	return out, err
}

// UpdateArbeitgeber applies a patch and preserves invariant I3.
func (s *Store) UpdateArbeitgeber(ctx context.Context, nutzerID, agID string, version int64, in ArbeitgeberInput, actor Actor) (sqlitedb.Arbeitgeber, error) {
	var out sqlitedb.Arbeitgeber
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: agID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Version != version {
			return ErrConflict
		}
		now := time.Now().UTC()
		standard := in.IstStandard && !in.Archiviert
		if !in.Archiviert && prev.Archiviert {
			n, err := q.CountAktiveArbeitgeber(ctx, nutzerID)
			if err != nil {
				return err
			}
			if n == 0 {
				standard = true
			}
		}
		var promote *sqlitedb.Arbeitgeber
		if !in.Archiviert && !prev.Archiviert && prev.IstStandard && !in.IstStandard {
			n, err := q.CountAktiveArbeitgeber(ctx, nutzerID)
			if err != nil {
				return err
			}
			if n <= 1 {
				standard = true
			} else {
				other, err := q.FirstOtherAktiverArbeitgeber(ctx, sqlitedb.FirstOtherAktiverArbeitgeberParams{NutzerID: nutzerID, ID: agID})
				if err != nil {
					return err
				}
				promote = &other
			}
		}
		if standard && !prev.IstStandard {
			if err := demoteStandards(ctx, q, nutzerID, agID, now, actor); err != nil {
				return err
			}
		}
		row, err := q.UpdateArbeitgeber(ctx, sqlitedb.UpdateArbeitgeberParams{
			Name: in.Name, Anschrift: in.Anschrift, UstID: in.UstID, Steuernummer: in.Steuernummer,
			LogoDateiID: prev.LogoDateiID, IstStandard: standard, Konstellation: prev.Konstellation,
			AbrechnungsnummerPraefix: in.Praefix, Archiviert: in.Archiviert, GeaendertAm: now,
			ID: agID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if promote == nil && in.Archiviert && prev.IstStandard {
			other, err := q.FirstOtherAktiverArbeitgeber(ctx, sqlitedb.FirstOtherAktiverArbeitgeberParams{NutzerID: nutzerID, ID: agID})
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err == nil {
				promote = &other
			}
		}
		if promote != nil {
			promoted, err := q.UpdateArbeitgeber(ctx, sqlitedb.UpdateArbeitgeberParams{
				Name: promote.Name, Anschrift: promote.Anschrift, UstID: promote.UstID, Steuernummer: promote.Steuernummer,
				LogoDateiID: promote.LogoDateiID, IstStandard: true, Konstellation: promote.Konstellation,
				AbrechnungsnummerPraefix: promote.AbrechnungsnummerPraefix, Archiviert: false, GeaendertAm: now,
				ID: promote.ID, NutzerID: nutzerID, Version: promote.Version,
			})
			if err != nil {
				return err
			}
			if err := appendAudit(ctx, q, actor, "arbeitgeber.standard", "arbeitgeber", promote.ID, agJSON(*promote), agJSON(promoted), nil); err != nil {
				return err
			}
		}
		out = row
		return appendAudit(ctx, q, actor, "arbeitgeber.geaendert", "arbeitgeber", agID, agJSON(prev), agJSON(row), nil)
	})
	return out, err
}

func demoteStandards(ctx context.Context, q *sqlitedb.Queries, nutzerID, keepID string, now time.Time, actor Actor) error {
	existing, err := q.ListArbeitgeber(ctx, sqlitedb.ListArbeitgeberParams{NutzerID: nutzerID, LimitN: 200})
	if err != nil {
		return err
	}
	if err := q.ClearAndereStandards(ctx, sqlitedb.ClearAndereStandardsParams{GeaendertAm: now, NutzerID: nutzerID, ID: keepID}); err != nil {
		return err
	}
	for _, row := range existing {
		if row.ID == keepID || !row.IstStandard || row.Archiviert {
			continue
		}
		row.IstStandard = false
		if err := appendAudit(ctx, q, actor, "arbeitgeber.standard", "arbeitgeber", row.ID, nil, agJSON(row), nil); err != nil {
			return err
		}
	}
	return nil
}

// GetArbeitgeber loads one row for this Nutzer.
func (s *Store) GetArbeitgeber(ctx context.Context, nutzerID, id string) (sqlitedb.Arbeitgeber, error) {
	row, err := s.readQ().GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: id, NutzerID: nutzerID})
	return row, mapErr(err)
}

// ListArbeitgeber lists one Nutzer's Arbeitgeber.
func (s *Store) ListArbeitgeber(ctx context.Context, nutzerID string, cursor *string, limit int64) ([]sqlitedb.Arbeitgeber, error) {
	var c any
	if cursor != nil {
		c = *cursor
	}
	return s.readQ().ListArbeitgeber(ctx, sqlitedb.ListArbeitgeberParams{NutzerID: nutzerID, Cursor: c, LimitN: limit})
}

// SetLogo stores logo metadata and points the Arbeitgeber at it.
func (s *Store) SetLogo(ctx context.Context, nutzerID, agID string, version int64, mime string, body []byte, key string, actor Actor) (sqlitedb.Arbeitgeber, error) {
	sum := sha256.Sum256(body)
	var out sqlitedb.Arbeitgeber
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: agID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Version != version {
			return ErrConflict
		}
		now := time.Now().UTC()
		file, err := q.InsertDatei(ctx, sqlitedb.InsertDateiParams{
			ID: id.Must(), NutzerID: nutzerID, Mime: mime, Bytes: int64(len(body)),
			Sha256: hex.EncodeToString(sum[:]), SpeicherSchluessel: key, ErstelltAm: now,
		})
		if err != nil {
			return err
		}
		row, err := q.UpdateArbeitgeber(ctx, sqlitedb.UpdateArbeitgeberParams{
			Name: prev.Name, Anschrift: prev.Anschrift, UstID: prev.UstID, Steuernummer: prev.Steuernummer,
			LogoDateiID: &file.ID, IstStandard: prev.IstStandard, Konstellation: prev.Konstellation,
			AbrechnungsnummerPraefix: prev.AbrechnungsnummerPraefix, Archiviert: prev.Archiviert,
			GeaendertAm: now, ID: agID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		out = row
		return appendAudit(ctx, q, actor, "arbeitgeber.logo", "arbeitgeber", agID, agJSON(prev), agJSON(row), nil)
	})
	return out, err
}

// GetDatei loads one file row owned by the Nutzer.
func (s *Store) GetDatei(ctx context.Context, nutzerID, id string) (sqlitedb.Datei, error) {
	row, err := s.readQ().GetDatei(ctx, sqlitedb.GetDateiParams{ID: id, NutzerID: nutzerID})
	return row, mapErr(err)
}

// CreateTaetigkeit inserts a Tätigkeitsstätte.
func (s *Store) CreateTaetigkeit(ctx context.Context, nutzerID string, in TaetigkeitInput, actor Actor) (sqlitedb.Taetigkeitsstaette, error) {
	var out sqlitedb.Taetigkeitsstaette
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		row, err := q.InsertTaetigkeitsstaette(ctx, sqlitedb.InsertTaetigkeitsstaetteParams{
			ID: id.Must(), NutzerID: nutzerID, Bezeichnung: in.Bezeichnung, Anschrift: in.Anschrift,
			LandIso: in.LandISO, Satzort: in.Satzort, Kunde: in.Kunde, ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		out = row
		return appendAudit(ctx, q, actor, "taetigkeitsstaette.angelegt", "taetigkeitsstaette", row.ID, nil, taetJSON(row), nil)
	})
	return out, err
}

// UpdateTaetigkeit updates one Tätigkeitsstätte.
func (s *Store) UpdateTaetigkeit(ctx context.Context, nutzerID, id string, version int64, in TaetigkeitInput, actor Actor) (sqlitedb.Taetigkeitsstaette, error) {
	var out sqlitedb.Taetigkeitsstaette
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetTaetigkeitsstaette(ctx, sqlitedb.GetTaetigkeitsstaetteParams{ID: id, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		row, err := q.UpdateTaetigkeitsstaette(ctx, sqlitedb.UpdateTaetigkeitsstaetteParams{
			Bezeichnung: in.Bezeichnung, Anschrift: in.Anschrift, LandIso: in.LandISO, Satzort: in.Satzort,
			Kunde: in.Kunde, GeaendertAm: time.Now().UTC(), ID: id, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		out = row
		return appendAudit(ctx, q, actor, "taetigkeitsstaette.geaendert", "taetigkeitsstaette", id, taetJSON(prev), taetJSON(row), nil)
	})
	return out, err
}

// DeleteTaetigkeit removes one Tätigkeitsstätte.
func (s *Store) DeleteTaetigkeit(ctx context.Context, nutzerID, id string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetTaetigkeitsstaette(ctx, sqlitedb.GetTaetigkeitsstaetteParams{ID: id, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if err := q.DeleteTaetigkeitsstaette(ctx, sqlitedb.DeleteTaetigkeitsstaetteParams{ID: id, NutzerID: nutzerID}); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "taetigkeitsstaette.geloescht", "taetigkeitsstaette", id, taetJSON(prev), nil, nil)
	})
}

// GetTaetigkeit loads one Tätigkeitsstätte for this Nutzer.
func (s *Store) GetTaetigkeit(ctx context.Context, nutzerID, id string) (sqlitedb.Taetigkeitsstaette, error) {
	row, err := s.readQ().GetTaetigkeitsstaette(ctx, sqlitedb.GetTaetigkeitsstaetteParams{ID: id, NutzerID: nutzerID})
	return row, mapErr(err)
}

// ListTaetigkeiten lists one Nutzer's Tätigkeitsstätten.
func (s *Store) ListTaetigkeiten(ctx context.Context, nutzerID string, cursor *string, limit int64) ([]sqlitedb.Taetigkeitsstaette, error) {
	var c any
	if cursor != nil {
		c = *cursor
	}
	return s.readQ().ListTaetigkeitsstaetten(ctx, sqlitedb.ListTaetigkeitsstaettenParams{NutzerID: nutzerID, Cursor: c, LimitN: limit})
}

// ListIdentitaeten lists linked identities of one Nutzer.
func (s *Store) ListIdentitaeten(ctx context.Context, nutzerID string) ([]sqlitedb.NutzerIdentitaet, error) {
	return s.readQ().ListIdentitaetenByNutzer(ctx, nutzerID)
}

// UnlinkIdentitaet removes one identity.
func (s *Store) UnlinkIdentitaet(ctx context.Context, nutzerID, identID string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		row, err := q.GetIdentitaetByID(ctx, sqlitedb.GetIdentitaetByIDParams{ID: identID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if err := q.DeleteIdentitaet(ctx, sqlitedb.DeleteIdentitaetParams{ID: identID, NutzerID: nutzerID}); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "identitaet.geloest", "nutzer_identitaet", row.ID, mustJSON(map[string]string{
			"art": row.Art, "aussteller": row.Aussteller, "subjekt": row.Subjekt,
		}), nil, nil)
	})
}

// ListProtokoll returns recent audit events, optionally filtered.
func (s *Store) ListProtokoll(ctx context.Context, objektTyp, objektID string, limit int64) ([]sqlitedb.AuditEreigni, error) {
	var typ, id any
	if objektTyp != "" {
		typ = objektTyp
	}
	if objektID != "" {
		id = objektID
	}
	return s.readQ().ListAuditFiltered(ctx, sqlitedb.ListAuditFilteredParams{ObjektTyp: typ, ObjektID: id, LimitN: limit})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func agJSON(a sqlitedb.Arbeitgeber) json.RawMessage {
	return mustJSON(map[string]any{
		"id": a.ID, "nutzer_id": a.NutzerID, "name": a.Name, "anschrift": a.Anschrift,
		"ist_standard": a.IstStandard, "archiviert": a.Archiviert, "logo_datei_id": a.LogoDateiID,
		"praefix": a.AbrechnungsnummerPraefix,
	})
}

func taetJSON(a sqlitedb.Taetigkeitsstaette) json.RawMessage {
	return mustJSON(map[string]any{
		"id": a.ID, "nutzer_id": a.NutzerID, "bezeichnung": a.Bezeichnung,
		"land_iso": a.LandIso, "satzort": a.Satzort, "kunde": a.Kunde,
	})
}

// KnownSatzort reports whether satzort belongs to land in the newest active year.
func (s *Store) KnownSatzort(ctx context.Context, land, satzort string) error {
	years, err := s.ListSatztabellen(ctx)
	if err != nil {
		return err
	}
	var newest *sqlitedb.Satztabelle
	for i := range years {
		if years[i].Status != "aktiv" {
			continue
		}
		if newest == nil || years[i].Jahr > newest.Jahr {
			newest = &years[i]
		}
	}
	if newest == nil {
		return ErrInvalid
	}
	y, err := s.EffectiveYear(ctx, int(newest.Jahr))
	if err != nil {
		return err
	}
	land = strings.ToUpper(land)
	satzort = satz.NormalizeSatzort(land, satzort)
	if satzort == "" {
		return nil
	}
	for _, row := range y.Rows {
		if row.LandISO == land && row.Satzort == satzort {
			return nil
		}
	}
	return ErrInvalid
}
