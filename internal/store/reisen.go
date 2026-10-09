package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// CodeError is a validation failure with a stable API code.
type CodeError struct{ Code string }

func (e *CodeError) Error() string { return e.Code }

func (e *CodeError) Unwrap() error { return ErrInvalid }

// OrtswechselInput is one leg of a Reise.
type OrtswechselInput struct {
	AbfahrtLokal                    string
	AbfahrtZone                     string
	AnkunftLokal                    string
	AnkunftZone                     string
	Verkehrsmittel                  string
	LandISO                         string
	Satzort                         string
	Ort                             string
	TaetigkeitsstaetteID            string
	ZwischenlandungMitUebernachtung bool
}

// ReiseInput creates or replaces a Reise and its Ortswechsel.
type ReiseInput struct {
	ArbeitgeberID     string
	Anlass            string
	Projekt           string
	BeginnLokal       string
	BeginnZone        string
	EndeLokal         string
	EndeZone          string
	Notiz             string
	VorlageID         string
	UnterkunftDefault string
	Ortswechsel       []OrtswechselInput
}

// ReisetagInput updates one Reisetag. LandISO empty clears a manual override.
type ReisetagInput struct {
	LandISO                   string
	Satzort                   string
	Begruendung               string
	Fruehstueck               bool
	Mittag                    bool
	Abend                     bool
	ZuzahlungFruehstueck      int64
	ZuzahlungMittag           int64
	ZuzahlungAbend            int64
	Unterkunft                string
	VerpflegungAusgeschlossen bool
	AusschlussGrund           string
}

// FahrtInput creates or updates a Fahrt.
type FahrtInput struct {
	Datum         string
	Start         string
	Ziel          string
	Zweck         string
	Fahrzeugart   string
	Km            int64
	HinUndZurueck bool
	VorlageID     string
}

// ReiseBundle is a Reise with its legs, days and Fahrten.
type ReiseBundle struct {
	Reise   sqlitedb.Reise
	Legs    []sqlitedb.Ortswechsel
	Tage    []sqlitedb.Reisetag
	Fahrten []sqlitedb.Fahrt
}

// ReiseFilter selects a page of Reisen.
type ReiseFilter struct {
	Status        string
	ArbeitgeberID string
	Projekt       string
	Q             string
	Von           *time.Time
	Bis           *time.Time
	Cursor        *string
	Limit         int64
}

// CreateReise inserts a Reise, its Ortswechsel and the gapless Reisetage.
func (s *Store) CreateReise(ctx context.Context, nutzerID string, in ReiseInput, actor Actor) (ReiseBundle, error) {
	var out ReiseBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		row, legs, days, err := insertReise(ctx, q, nutzerID, in)
		if err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "reise.angelegt", "reise", row.ID, nil, mustJSON(row), nil); err != nil {
			return err
		}
		out = ReiseBundle{Reise: row, Legs: legs, Tage: days}
		return nil
	})
	return out, err
}

// UpdateReise replaces Ortswechsel and reconciles Reisetage so I4 holds.
func (s *Store) UpdateReise(ctx context.Context, nutzerID, reiseID string, version int64, in ReiseInput, actor Actor) (ReiseBundle, error) {
	var out ReiseBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Status == "gesperrt" {
			return &CodeError{Code: "reise_gesperrt"}
		}
		beginn, ende, err := checkReiseTimes(in)
		if err != nil {
			return err
		}
		if err := checkLegs(ctx, q, nutzerID, in.Ortswechsel); err != nil {
			return err
		}
		if _, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: in.ArbeitgeberID, NutzerID: nutzerID}); err != nil {
			if errors.Is(mapErr(err), ErrNotFound) {
				return &CodeError{Code: "arbeitgeber"}
			}
			return err
		}
		row, err := q.UpdateReise(ctx, sqlitedb.UpdateReiseParams{
			ArbeitgeberID: in.ArbeitgeberID, Anlass: strings.TrimSpace(in.Anlass), Projekt: emptyNil(in.Projekt),
			Beginn: beginn, BeginnZone: in.BeginnZone, Ende: ende, EndeZone: in.EndeZone,
			Notiz: emptyNil(in.Notiz), GeaendertAm: time.Now().UTC(), ID: reiseID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := q.DeleteOrtswechsel(ctx, reiseID); err != nil {
			return err
		}
		legs, err := insertLegs(ctx, q, reiseID, in.Ortswechsel)
		if err != nil {
			return err
		}
		prevDays, err := q.ListReisetage(ctx, reiseID)
		if err != nil {
			return err
		}
		dates, err := tripDates(in.BeginnLokal, in.BeginnZone, in.EndeLokal, in.EndeZone)
		if err != nil {
			return err
		}
		days, err := reconcileDays(ctx, q, reiseID, dates, prevDays, in.UnterkunftDefault)
		if err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "reise.geaendert", "reise", reiseID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		fahrten, err := q.ListFahrten(ctx, sqlitedb.ListFahrtenParams{ReiseID: reiseID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		out = ReiseBundle{Reise: row, Legs: legs, Tage: days, Fahrten: fahrten}
		return nil
	})
	return out, err
}

// GetReiseBundle loads one Reise for its Nutzer.
func (s *Store) GetReiseBundle(ctx context.Context, nutzerID, reiseID string) (ReiseBundle, error) {
	var out ReiseBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		var err error
		out, err = loadBundle(ctx, q, nutzerID, reiseID)
		return err
	})
	return out, err
}

// ListReisen returns a page of Reisen with legs and days.
func (s *Store) ListReisen(ctx context.Context, nutzerID string, f ReiseFilter) ([]ReiseBundle, error) {
	var out []ReiseBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		rows, err := q.ListReisen(ctx, sqlitedb.ListReisenParams{
			NutzerID: nutzerID, Status: f.Status, ArbeitgeberID: f.ArbeitgeberID, Projekt: f.Projekt,
			Q: f.Q, QLike: likePat(f.Q), Von: f.Von, Bis: f.Bis, Cursor: f.Cursor, LimitN: f.Limit,
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			b, err := loadParts(ctx, q, nutzerID, row)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return nil
	})
	return out, err
}

// ListReisenAll loads every Reise of a Nutzer for the day rule (4.5).
func (s *Store) ListReisenAll(ctx context.Context, nutzerID string) ([]ReiseBundle, error) {
	var out []ReiseBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		rows, err := q.ListReisenAll(ctx, nutzerID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			b, err := loadParts(ctx, q, nutzerID, row)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return nil
	})
	return out, err
}

// DeleteReise removes a Reise that is not gesperrt.
func (s *Store) DeleteReise(ctx context.Context, nutzerID, reiseID string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Status == "gesperrt" {
			return &CodeError{Code: "reise_gesperrt"}
		}
		n, err := q.DeleteReise(ctx, sqlitedb.DeleteReiseParams{ID: reiseID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return appendAudit(ctx, q, actor, "reise.geloescht", "reise", reiseID, mustJSON(prev), nil, nil)
	})
}

// PatchReisetag updates meals, lodging and an optional country override.
func (s *Store) PatchReisetag(ctx context.Context, nutzerID, reiseID, datum string, version int64, in ReisetagInput, actor Actor) (ReiseBundle, error) {
	var out ReiseBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Status == "gesperrt" {
			return &CodeError{Code: "reise_gesperrt"}
		}
		day, err := q.GetReisetag(ctx, sqlitedb.GetReisetagParams{ReiseID: reiseID, Datum: datum})
		if err != nil {
			return mapErr(err)
		}
		dates, err := tripDates(formatLocal(prev.Beginn, prev.BeginnZone), prev.BeginnZone, formatLocal(prev.Ende, prev.EndeZone), prev.EndeZone)
		if err != nil {
			return err
		}
		last := ""
		if len(dates) > 0 {
			last = dates[len(dates)-1]
		}
		u := in.Unterkunft
		if u == "" {
			u = "keine"
		}
		if datum == last {
			u = "keine"
		}
		if !unterkunftOK(u) {
			return &CodeError{Code: "unterkunft"}
		}
		if !centsOK(in.ZuzahlungFruehstueck) || !centsOK(in.ZuzahlungMittag) || !centsOK(in.ZuzahlungAbend) {
			return &CodeError{Code: "zuzahlung"}
		}
		land := strings.ToUpper(strings.TrimSpace(in.LandISO))
		var grund *string
		if land != "" {
			if strings.TrimSpace(in.Begruendung) == "" {
				return &CodeError{Code: "begruendung_fehlt"}
			}
			g := strings.TrimSpace(in.Begruendung)
			grund = &g
		}
		if in.VerpflegungAusgeschlossen && strings.TrimSpace(in.AusschlussGrund) == "" {
			return &CodeError{Code: "ausschluss_grund"}
		}
		updated, err := q.UpdateReisetag(ctx, sqlitedb.UpdateReisetagParams{
			LandManuell: emptyNil(land), SatzortManuell: emptyNil(in.Satzort), LandBegruendung: grund,
			FruehstueckGestellt: in.Fruehstueck, MittagGestellt: in.Mittag, AbendGestellt: in.Abend,
			ZuzahlungFruehstueck: in.ZuzahlungFruehstueck, ZuzahlungMittag: in.ZuzahlungMittag, ZuzahlungAbend: in.ZuzahlungAbend,
			MahlzeitQuelle: "manuell", Unterkunft: u, VerpflegungAusgeschlossen: in.VerpflegungAusgeschlossen,
			AusschlussGrund: emptyNil(in.AusschlussGrund), ReiseID: reiseID, Datum: datum,
		})
		if err != nil {
			return err
		}
		row, err := q.TouchReise(ctx, sqlitedb.TouchReiseParams{
			GeaendertAm: time.Now().UTC(), ID: reiseID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		auditGrund := grund
		if auditGrund == nil {
			auditGrund = emptyNil(in.AusschlussGrund)
		}
		if err := appendAudit(ctx, q, actor, "reisetag.geaendert", "reisetag", day.ID, mustJSON(day), mustJSON(updated), auditGrund); err != nil {
			return err
		}
		out, err = loadParts(ctx, q, nutzerID, row)
		return err
	})
	return out, err
}

// CreateFahrt adds a private-vehicle line to a Reise of the same Nutzer (I1).
func (s *Store) CreateFahrt(ctx context.Context, nutzerID, reiseID string, in FahrtInput, actor Actor) (sqlitedb.Fahrt, error) {
	var out sqlitedb.Fahrt
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		if _, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID}); err != nil {
			return mapErr(err)
		}
		if err := checkFahrt(in); err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := q.InsertFahrt(ctx, sqlitedb.InsertFahrtParams{
			ID: id.Must(), ReiseID: reiseID, NutzerID: nutzerID, Datum: in.Datum,
			StartOrt: strings.TrimSpace(in.Start), Ziel: strings.TrimSpace(in.Ziel), Zweck: emptyNil(in.Zweck),
			Fahrzeugart: in.Fahrzeugart, Km: in.Km, HinUndZurueck: in.HinUndZurueck, VorlageID: emptyNil(in.VorlageID),
			ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "fahrt.angelegt", "fahrt", row.ID, nil, mustJSON(row), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// UpdateFahrt replaces a Fahrt of this Nutzer.
func (s *Store) UpdateFahrt(ctx context.Context, nutzerID, fahrtID string, version int64, in FahrtInput, actor Actor) (sqlitedb.Fahrt, error) {
	var out sqlitedb.Fahrt
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetFahrt(ctx, sqlitedb.GetFahrtParams{ID: fahrtID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		reise, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: prev.ReiseID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if reise.Status == "gesperrt" {
			return &CodeError{Code: "reise_gesperrt"}
		}
		if err := checkFahrt(in); err != nil {
			return err
		}
		row, err := q.UpdateFahrt(ctx, sqlitedb.UpdateFahrtParams{
			Datum: in.Datum, StartOrt: strings.TrimSpace(in.Start), Ziel: strings.TrimSpace(in.Ziel),
			Zweck: emptyNil(in.Zweck), Fahrzeugart: in.Fahrzeugart, Km: in.Km, HinUndZurueck: in.HinUndZurueck,
			GeaendertAm: time.Now().UTC(), ID: fahrtID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "fahrt.geaendert", "fahrt", row.ID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// DeleteFahrt removes a Fahrt of this Nutzer.
func (s *Store) DeleteFahrt(ctx context.Context, nutzerID, fahrtID string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetFahrt(ctx, sqlitedb.GetFahrtParams{ID: fahrtID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		n, err := q.DeleteFahrt(ctx, sqlitedb.DeleteFahrtParams{ID: fahrtID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return appendAudit(ctx, q, actor, "fahrt.geloescht", "fahrt", fahrtID, mustJSON(prev), nil, nil)
	})
}

// ListFahrten lists Fahrten of one Reise.
func (s *Store) ListFahrten(ctx context.Context, nutzerID, reiseID string) ([]sqlitedb.Fahrt, error) {
	var out []sqlitedb.Fahrt
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		if _, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID}); err != nil {
			return mapErr(err)
		}
		var err error
		out, err = q.ListFahrten(ctx, sqlitedb.ListFahrtenParams{ReiseID: reiseID, NutzerID: nutzerID})
		return err
	})
	return out, err
}

// CreateVorlage stores an independent template document.
func (s *Store) CreateVorlage(ctx context.Context, nutzerID, name, art, daten string, actor Actor) (sqlitedb.Vorlage, error) {
	if art != "reise" && art != "fahrt" {
		return sqlitedb.Vorlage{}, &CodeError{Code: "vorlage_art"}
	}
	if strings.TrimSpace(name) == "" || !json.Valid([]byte(daten)) {
		return sqlitedb.Vorlage{}, &CodeError{Code: "validierung"}
	}
	var out sqlitedb.Vorlage
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		row, err := q.InsertVorlage(ctx, sqlitedb.InsertVorlageParams{
			ID: id.Must(), NutzerID: nutzerID, Name: strings.TrimSpace(name), Art: art, Daten: daten,
			ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "vorlage.angelegt", "vorlage", row.ID, nil, mustJSON(row), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// UpdateVorlage replaces the name and JSON of a Vorlage.
func (s *Store) UpdateVorlage(ctx context.Context, nutzerID, vorlageID string, version int64, name, daten string, actor Actor) (sqlitedb.Vorlage, error) {
	if strings.TrimSpace(name) == "" || !json.Valid([]byte(daten)) {
		return sqlitedb.Vorlage{}, &CodeError{Code: "validierung"}
	}
	var out sqlitedb.Vorlage
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetVorlage(ctx, sqlitedb.GetVorlageParams{ID: vorlageID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		row, err := q.UpdateVorlage(ctx, sqlitedb.UpdateVorlageParams{
			Name: strings.TrimSpace(name), Daten: daten, GeaendertAm: time.Now().UTC(),
			ID: vorlageID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "vorlage.geaendert", "vorlage", row.ID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// GetVorlage loads one Vorlage.
func (s *Store) GetVorlage(ctx context.Context, nutzerID, vorlageID string) (sqlitedb.Vorlage, error) {
	row, err := s.readQ().GetVorlage(ctx, sqlitedb.GetVorlageParams{ID: vorlageID, NutzerID: nutzerID})
	return row, mapErr(err)
}

// ListVorlagen lists Vorlagen, optionally by art.
func (s *Store) ListVorlagen(ctx context.Context, nutzerID, art string, cursor *string, limit int64) ([]sqlitedb.Vorlage, error) {
	return s.readQ().ListVorlagen(ctx, sqlitedb.ListVorlagenParams{
		NutzerID: nutzerID, Art: art, Cursor: cursor, LimitN: limit,
	})
}

// DeleteVorlage removes a Vorlage. Existing Reisen keep their copied data.
func (s *Store) DeleteVorlage(ctx context.Context, nutzerID, vorlageID string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetVorlage(ctx, sqlitedb.GetVorlageParams{ID: vorlageID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		n, err := q.DeleteVorlage(ctx, sqlitedb.DeleteVorlageParams{ID: vorlageID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return appendAudit(ctx, q, actor, "vorlage.geloescht", "vorlage", vorlageID, mustJSON(prev), nil, nil)
	})
}

// ListProjekte returns distinct project names, newest trip first.
func (s *Store) ListProjekte(ctx context.Context, nutzerID, q string, limit int64) ([]string, error) {
	pat := likePat(q)
	rows, err := s.readQ().ListProjekte(ctx, sqlitedb.ListProjekteParams{
		NutzerID: nutzerID, Q: q, QLike: &pat, LimitN: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Projekt != nil && *row.Projekt != "" {
			out = append(out, *row.Projekt)
		}
	}
	return out, nil
}

func insertReise(ctx context.Context, q *sqlitedb.Queries, nutzerID string, in ReiseInput) (sqlitedb.Reise, []sqlitedb.Ortswechsel, []sqlitedb.Reisetag, error) {
	beginn, ende, err := checkReiseTimes(in)
	if err != nil {
		return sqlitedb.Reise{}, nil, nil, err
	}
	if err := checkLegs(ctx, q, nutzerID, in.Ortswechsel); err != nil {
		return sqlitedb.Reise{}, nil, nil, err
	}
	if _, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: in.ArbeitgeberID, NutzerID: nutzerID}); err != nil {
		if errors.Is(mapErr(err), ErrNotFound) {
			return sqlitedb.Reise{}, nil, nil, &CodeError{Code: "arbeitgeber"}
		}
		return sqlitedb.Reise{}, nil, nil, err
	}
	now := time.Now().UTC()
	row, err := q.InsertReise(ctx, sqlitedb.InsertReiseParams{
		ID: id.Must(), NutzerID: nutzerID, ArbeitgeberID: in.ArbeitgeberID, Anlass: strings.TrimSpace(in.Anlass),
		Projekt: emptyNil(in.Projekt), Beginn: beginn, BeginnZone: in.BeginnZone, Ende: ende, EndeZone: in.EndeZone,
		Notiz: emptyNil(in.Notiz), VorlageID: emptyNil(in.VorlageID), Status: "offen", ErstelltAm: now, GeaendertAm: now,
	})
	if err != nil {
		return sqlitedb.Reise{}, nil, nil, err
	}
	legs, err := insertLegs(ctx, q, row.ID, in.Ortswechsel)
	if err != nil {
		return sqlitedb.Reise{}, nil, nil, err
	}
	dates, err := tripDates(in.BeginnLokal, in.BeginnZone, in.EndeLokal, in.EndeZone)
	if err != nil {
		return sqlitedb.Reise{}, nil, nil, err
	}
	days, err := reconcileDays(ctx, q, row.ID, dates, nil, in.UnterkunftDefault)
	return row, legs, days, err
}

func loadBundle(ctx context.Context, q *sqlitedb.Queries, nutzerID, reiseID string) (ReiseBundle, error) {
	row, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID})
	if err != nil {
		return ReiseBundle{}, mapErr(err)
	}
	return loadParts(ctx, q, nutzerID, row)
}

func loadParts(ctx context.Context, q *sqlitedb.Queries, nutzerID string, row sqlitedb.Reise) (ReiseBundle, error) {
	legs, err := q.ListOrtswechsel(ctx, row.ID)
	if err != nil {
		return ReiseBundle{}, err
	}
	days, err := q.ListReisetage(ctx, row.ID)
	if err != nil {
		return ReiseBundle{}, err
	}
	fahrten, err := q.ListFahrten(ctx, sqlitedb.ListFahrtenParams{ReiseID: row.ID, NutzerID: nutzerID})
	if err != nil {
		return ReiseBundle{}, err
	}
	return ReiseBundle{Reise: row, Legs: legs, Tage: days, Fahrten: fahrten}, nil
}

func insertLegs(ctx context.Context, q *sqlitedb.Queries, reiseID string, legs []OrtswechselInput) ([]sqlitedb.Ortswechsel, error) {
	out := make([]sqlitedb.Ortswechsel, 0, len(legs))
	for i, leg := range legs {
		ankunft, err := parseLocal(leg.AnkunftLokal, leg.AnkunftZone)
		if err != nil {
			return nil, err
		}
		var abfahrt *time.Time
		var abZone *string
		if leg.AbfahrtLokal != "" {
			t, err := parseLocal(leg.AbfahrtLokal, leg.AbfahrtZone)
			if err != nil {
				return nil, err
			}
			abfahrt = &t
			z := leg.AbfahrtZone
			abZone = &z
		}
		row, err := q.InsertOrtswechsel(ctx, sqlitedb.InsertOrtswechselParams{
			ID: id.Must(), ReiseID: reiseID, Reihenfolge: int64(i + 1), Abfahrt: abfahrt, AbfahrtZone: abZone,
			Ankunft: ankunft, AnkunftZone: leg.AnkunftZone, Verkehrsmittel: leg.Verkehrsmittel,
			LandIso: strings.ToUpper(leg.LandISO), Satzort: leg.Satzort, Ort: leg.Ort,
			TaetigkeitsstaetteID:            emptyNil(leg.TaetigkeitsstaetteID),
			ZwischenlandungMitUebernachtung: leg.ZwischenlandungMitUebernachtung,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func reconcileDays(ctx context.Context, q *sqlitedb.Queries, reiseID string, dates []string, existing []sqlitedb.Reisetag, fallback string) ([]sqlitedb.Reisetag, error) {
	have := map[string]sqlitedb.Reisetag{}
	for _, day := range existing {
		have[day.Datum] = day
	}
	keep := map[string]bool{}
	last := ""
	if len(dates) > 0 {
		last = dates[len(dates)-1]
	}
	if !unterkunftOK(fallback) {
		fallback = "keine"
	}
	for _, datum := range dates {
		keep[datum] = true
		if row, ok := have[datum]; ok {
			if datum == last && row.Unterkunft != "keine" {
				row.Unterkunft = "keine"
				if _, err := q.UpdateReisetag(ctx, dayParams(row)); err != nil {
					return nil, err
				}
			}
			continue
		}
		u := "keine"
		if datum != last {
			u = fallback
		}
		if _, err := q.InsertReisetag(ctx, sqlitedb.InsertReisetagParams{
			ID: id.Must(), ReiseID: reiseID, Datum: datum, MahlzeitQuelle: "", Unterkunft: u,
		}); err != nil {
			return nil, err
		}
	}
	for _, row := range existing {
		if !keep[row.Datum] {
			if err := q.DeleteReisetag(ctx, sqlitedb.DeleteReisetagParams{ReiseID: reiseID, Datum: row.Datum}); err != nil {
				return nil, err
			}
		}
	}
	return q.ListReisetage(ctx, reiseID)
}

func dayParams(row sqlitedb.Reisetag) sqlitedb.UpdateReisetagParams {
	return sqlitedb.UpdateReisetagParams{
		LandManuell: row.LandManuell, SatzortManuell: row.SatzortManuell, LandBegruendung: row.LandBegruendung,
		FruehstueckGestellt: row.FruehstueckGestellt, MittagGestellt: row.MittagGestellt, AbendGestellt: row.AbendGestellt,
		ZuzahlungFruehstueck: row.ZuzahlungFruehstueck, ZuzahlungMittag: row.ZuzahlungMittag, ZuzahlungAbend: row.ZuzahlungAbend,
		MahlzeitQuelle: row.MahlzeitQuelle, Unterkunft: row.Unterkunft,
		VerpflegungAusgeschlossen: row.VerpflegungAusgeschlossen, AusschlussGrund: row.AusschlussGrund,
		ReiseID: row.ReiseID, Datum: row.Datum,
	}
}

func checkReiseTimes(in ReiseInput) (time.Time, time.Time, error) {
	if strings.TrimSpace(in.Anlass) == "" || len(in.Anlass) > 500 {
		return time.Time{}, time.Time{}, &CodeError{Code: "anlass"}
	}
	beginn, err := parseLocal(in.BeginnLokal, in.BeginnZone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	ende, err := parseLocal(in.EndeLokal, in.EndeZone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !ende.After(beginn) {
		return time.Time{}, time.Time{}, &CodeError{Code: "ende_vor_beginn"}
	}
	return beginn, ende, nil
}

func checkLegs(ctx context.Context, q *sqlitedb.Queries, nutzerID string, legs []OrtswechselInput) error {
	if len(legs) == 0 {
		return &CodeError{Code: "ortswechsel_fehlt"}
	}
	for _, leg := range legs {
		if !verkehrOK(leg.Verkehrsmittel) {
			return &CodeError{Code: "verkehrsmittel"}
		}
		if len(leg.LandISO) != 2 {
			return &CodeError{Code: "land"}
		}
		if _, err := parseLocal(leg.AnkunftLokal, leg.AnkunftZone); err != nil {
			return err
		}
		if leg.TaetigkeitsstaetteID != "" {
			if _, err := q.GetTaetigkeitsstaette(ctx, sqlitedb.GetTaetigkeitsstaetteParams{
				ID: leg.TaetigkeitsstaetteID, NutzerID: nutzerID,
			}); err != nil {
				if errors.Is(mapErr(err), ErrNotFound) {
					return &CodeError{Code: "taetigkeitsstaette"}
				}
				return err
			}
		}
	}
	return nil
}

func checkFahrt(in FahrtInput) error {
	if _, err := time.Parse("2006-01-02", in.Datum); err != nil {
		return &CodeError{Code: "datum"}
	}
	if in.Fahrzeugart != "kraftwagen" && in.Fahrzeugart != "anderes_motorfahrzeug" {
		return &CodeError{Code: "fahrzeugart"}
	}
	if in.Km <= 0 || in.Km > 100_000 {
		return &CodeError{Code: "km"}
	}
	if strings.TrimSpace(in.Start) == "" || strings.TrimSpace(in.Ziel) == "" {
		return &CodeError{Code: "ort"}
	}
	return nil
}

func tripDates(beginn, bZone, ende, eZone string) ([]string, error) {
	return berechnung.Dates(berechnung.Reise{
		Beginn: berechnung.Zeitpunkt{Lokal: trimLocal(beginn), Zone: bZone},
		Ende:   berechnung.Zeitpunkt{Lokal: trimLocal(ende), Zone: eZone},
	})
}

func parseLocal(local, zone string) (time.Time, error) {
	local = trimLocal(local)
	if zone == "" {
		return time.Time{}, &CodeError{Code: "zeitzone"}
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, &CodeError{Code: "zeitzone"}
	}
	t, err := time.ParseInLocation("2006-01-02T15:04:05", local, loc)
	if err != nil {
		return time.Time{}, &CodeError{Code: "zeit"}
	}
	return t.UTC(), nil
}

func trimLocal(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 19 {
		return s[:19]
	}
	return s
}

func formatLocal(t time.Time, zone string) string {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return t.UTC().Format("2006-01-02T15:04:05")
	}
	return t.In(loc).Format("2006-01-02T15:04:05")
}

func verkehrOK(s string) bool {
	switch s {
	case "pkw", "bahn", "flug", "schiff", "bus", "sonstiges":
		return true
	default:
		return false
	}
}

func unterkunftOK(s string) bool {
	switch s {
	case "", "keine", "beleg", "pauschale", "gestellt", "verkehrsmittel":
		return true
	default:
		return false
	}
}

func centsOK(n int64) bool { return n >= 0 && n <= 100_000_000 }

func likePat(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	if q == "" {
		return "%"
	}
	return "%" + q + "%"
}

func emptyNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
