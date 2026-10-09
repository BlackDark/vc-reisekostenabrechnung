package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/fx"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// Teilnehmer is one guest on a Bewirtung.
type Teilnehmer struct {
	Name  string
	Firma string
}

// BewirtungInput is the entertainment record stored as JSON.
type BewirtungInput struct {
	Anlass        string
	Ort           string
	Bewirtender   string
	Trinkgeld     int64
	Teilnehmer    []Teilnehmer
	BestaetigtAm  string
	BestaetigtVon string
}

// AnteilInput is one VAT share in receipt currency and EUR.
type AnteilInput struct {
	Satz       int64
	Steuerland string
	Brutto     int64
	Netto      *int64
	Steuer     *int64
	NettoEUR   int64
	SteuerEUR  int64
	BruttoEUR  int64
}

// AusgabeInput creates or replaces an Ausgabe. Amounts are cents.
type AusgabeInput struct {
	Kostenart              string
	Datum                  string
	Leistender             string
	Beschreibung           string
	Waehrung               string
	Betrag                 int64
	Kurs                   string
	KursQuelle             string
	KursDatum              string
	KursGrund              string
	BetragEUR              int64
	Rechnungsart           string
	RechnungAufArbeitgeber bool
	Empfaenger             string
	Verkehrsmittel         string
	Naechte                []string
	Fruehstueck            bool
	Mahlzeit               string
	TSE                    bool
	Bewirtung              *BewirtungInput
	BelegIDs               []string
	Anteile                []AnteilInput
}

// AusgabeBundle is an Ausgabe with its shares, receipts and optional Eigenbeleg.
type AusgabeBundle struct {
	Row     sqlitedb.Ausgabe
	Anteile []sqlitedb.Steueranteil
	Belege  []sqlitedb.ListBelegeForAusgabeRow
	Eigen   *sqlitedb.Eigenbeleg
}

// EigenbelegInput is the substitute receipt (SPEC 3.2).
type EigenbelegInput struct {
	Grund              string
	Zahlungsempfaenger string
	Art                string
}

// VorschussInput is an unallocated advance.
type VorschussInput struct {
	ArbeitgeberID string
	Datum         string
	Betrag        int64
	Notiz         string
}

// CreateAusgabe inserts an Ausgabe, its Steueranteile and optional Beleg links.
func (s *Store) CreateAusgabe(ctx context.Context, nutzerID, reiseID string, in AusgabeInput, actor Actor) (AusgabeBundle, error) {
	var out AusgabeBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		if err := ensureReiseOffen(ctx, q, nutzerID, reiseID); err != nil {
			return err
		}
		if err := prepareAusgabe(&in); err != nil {
			return err
		}
		if err := checkBelege(ctx, q, nutzerID, in.BelegIDs); err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := q.InsertAusgabe(ctx, insertAusgabeParams(id.Must(), nutzerID, reiseID, in, now))
		if err != nil {
			return err
		}
		anteile, err := replaceAnteile(ctx, q, row.ID, in.Anteile)
		if err != nil {
			return err
		}
		if err := replaceBelege(ctx, q, row.ID, in.BelegIDs); err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "ausgabe.angelegt", "ausgabe", row.ID, nil, mustJSON(row), nil); err != nil {
			return err
		}
		out, err = loadAusgabe(ctx, q, nutzerID, row)
		if err != nil {
			return err
		}
		out.Anteile = anteile
		return nil
	})
	return out, err
}

// UpdateAusgabe replaces the line, its shares and, when BelegIDs is non-nil, the links.
func (s *Store) UpdateAusgabe(ctx context.Context, nutzerID, ausgabeID string, version int64, in AusgabeInput, actor Actor) (AusgabeBundle, error) {
	var out AusgabeBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAusgabe(ctx, sqlitedb.GetAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if err := ensureReiseOffen(ctx, q, nutzerID, prev.ReiseID); err != nil {
			return err
		}
		if err := prepareAusgabe(&in); err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := q.UpdateAusgabe(ctx, updateAusgabeParams(ausgabeID, nutzerID, version, in, now))
		if err != nil {
			return mapUpdateErr(err)
		}
		if _, err := replaceAnteile(ctx, q, row.ID, in.Anteile); err != nil {
			return err
		}
		if in.BelegIDs != nil {
			if err := checkBelege(ctx, q, nutzerID, in.BelegIDs); err != nil {
				return err
			}
			if err := replaceBelege(ctx, q, row.ID, in.BelegIDs); err != nil {
				return err
			}
		}
		if err := appendAudit(ctx, q, actor, "ausgabe.geaendert", "ausgabe", row.ID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out, err = loadAusgabe(ctx, q, nutzerID, row)
		return err
	})
	return out, err
}

// GetAusgabe loads one Ausgabe owned by the Nutzer.
func (s *Store) GetAusgabe(ctx context.Context, nutzerID, ausgabeID string) (AusgabeBundle, error) {
	row, err := s.readQ().GetAusgabe(ctx, sqlitedb.GetAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
	if err != nil {
		return AusgabeBundle{}, mapErr(err)
	}
	return loadAusgabe(ctx, s.readQ(), nutzerID, row)
}

// ListAusgaben lists the Ausgaben of one Reise.
func (s *Store) ListAusgaben(ctx context.Context, nutzerID, reiseID string) ([]AusgabeBundle, error) {
	if _, err := s.readQ().GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID}); err != nil {
		return nil, mapErr(err)
	}
	return loadAusgabeBundles(ctx, s.readQ(), nutzerID, reiseID)
}

// DeleteAusgabe removes an Ausgabe from an open Reise.
func (s *Store) DeleteAusgabe(ctx context.Context, nutzerID, ausgabeID string, version int64, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAusgabe(ctx, sqlitedb.GetAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Version != version {
			return ErrConflict
		}
		if err := ensureReiseOffen(ctx, q, nutzerID, prev.ReiseID); err != nil {
			return err
		}
		n, err := q.DeleteAusgabe(ctx, sqlitedb.DeleteAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return appendAudit(ctx, q, actor, "ausgabe.geloescht", "ausgabe", ausgabeID, mustJSON(prev), nil, nil)
	})
}

// SetAusgabeBelege replaces the n:m links. One Beleg may cover several Ausgaben.
func (s *Store) SetAusgabeBelege(ctx context.Context, nutzerID, ausgabeID string, version int64, belegIDs []string, actor Actor) (AusgabeBundle, error) {
	var out AusgabeBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAusgabe(ctx, sqlitedb.GetAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Version != version {
			return ErrConflict
		}
		if err := ensureReiseOffen(ctx, q, nutzerID, prev.ReiseID); err != nil {
			return err
		}
		if err := checkBelege(ctx, q, nutzerID, belegIDs); err != nil {
			return err
		}
		if err := replaceBelege(ctx, q, ausgabeID, belegIDs); err != nil {
			return err
		}
		row, err := q.SetRechnungsart(ctx, sqlitedb.SetRechnungsartParams{
			Rechnungsart: prev.Rechnungsart, GeaendertAm: time.Now().UTC(), ID: ausgabeID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "ausgabe.belege", "ausgabe", ausgabeID, nil, mustJSON(belegIDs), nil); err != nil {
			return err
		}
		out, err = loadAusgabe(ctx, q, nutzerID, row)
		return err
	})
	return out, err
}

// PutEigenbeleg stores the substitute receipt and marks the Ausgabe as Eigenbeleg.
func (s *Store) PutEigenbeleg(ctx context.Context, nutzerID, ausgabeID string, version int64, in EigenbelegInput, actor Actor) (AusgabeBundle, error) {
	if strings.TrimSpace(in.Grund) == "" || strings.TrimSpace(in.Zahlungsempfaenger) == "" || strings.TrimSpace(in.Art) == "" {
		return AusgabeBundle{}, &CodeError{Code: "eigenbeleg"}
	}
	var out AusgabeBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAusgabe(ctx, sqlitedb.GetAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if err := ensureReiseOffen(ctx, q, nutzerID, prev.ReiseID); err != nil {
			return err
		}
		now := time.Now().UTC()
		who := ""
		if actor.NutzerID != nil {
			who = *actor.NutzerID
		}
		if _, err := q.UpsertEigenbeleg(ctx, sqlitedb.UpsertEigenbelegParams{
			AusgabeID: ausgabeID, Grund: strings.TrimSpace(in.Grund), Zahlungsempfaenger: strings.TrimSpace(in.Zahlungsempfaenger),
			Art: strings.TrimSpace(in.Art), ErstelltAm: now, BestaetigtAm: &now, BestaetigtVon: emptyNil(who),
		}); err != nil {
			return err
		}
		row, err := q.SetRechnungsart(ctx, sqlitedb.SetRechnungsartParams{
			Rechnungsart: "eigenbeleg", GeaendertAm: now, ID: ausgabeID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "eigenbeleg.angelegt", "ausgabe", ausgabeID, nil, mustJSON(in), nil); err != nil {
			return err
		}
		out, err = loadAusgabe(ctx, q, nutzerID, row)
		return err
	})
	return out, err
}

// ConfirmBewirtung stamps the digital confirmation (SPEC 4.12).
func (s *Store) ConfirmBewirtung(ctx context.Context, nutzerID, ausgabeID string, version int64, actor Actor) (AusgabeBundle, error) {
	var out AusgabeBundle
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetAusgabe(ctx, sqlitedb.GetAusgabeParams{ID: ausgabeID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Kostenart != "bewirtung" {
			return &CodeError{Code: "bewirtung"}
		}
		if err := ensureReiseOffen(ctx, q, nutzerID, prev.ReiseID); err != nil {
			return err
		}
		doc := decodeBewirtung(prev.Bewirtung)
		if doc == nil {
			doc = &BewirtungInput{}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		doc.BestaetigtAm = now
		if actor.NutzerID != nil {
			doc.BestaetigtVon = *actor.NutzerID
		}
		raw, err := json.Marshal(bewirtungJSON(*doc))
		if err != nil {
			return err
		}
		text := string(raw)
		row, err := q.SetBewirtung(ctx, sqlitedb.SetBewirtungParams{
			Bewirtung: &text, TseBeleg: prev.TseBeleg, GeaendertAm: time.Now().UTC(),
			ID: ausgabeID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "bewirtung.bestaetigt", "ausgabe", ausgabeID, nil, mustJSON(doc), nil); err != nil {
			return err
		}
		out, err = loadAusgabe(ctx, q, nutzerID, row)
		return err
	})
	return out, err
}

// ApplyMealSuggestions sets Gestellte Mahlzeiten from Ausgaben unless the day was edited by hand.
func (s *Store) ApplyMealSuggestions(ctx context.Context, nutzerID, reiseID string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		bundle, err := loadBundle(ctx, q, nutzerID, reiseID)
		if err != nil {
			return err
		}
		suggestions := berechnung.SuggestMeals(CalcReise(bundle))
		byDay := map[string][]berechnung.Vorschlag{}
		for _, v := range suggestions {
			byDay[v.Datum] = append(byDay[v.Datum], v)
		}
		for _, day := range bundle.Tage {
			if day.MahlzeitQuelle == "manuell" {
				continue
			}
			fr, mi, ab := day.FruehstueckGestellt, day.MittagGestellt, day.AbendGestellt
			prev := parseMealQuelle(day.MahlzeitQuelle)
			if _, ok := prev["fruehstueck"]; ok {
				fr = false
			}
			if _, ok := prev["mittag"]; ok {
				mi = false
			}
			if _, ok := prev["abend"]; ok {
				ab = false
			}
			next := map[string]string{}
			for _, v := range byDay[day.Datum] {
				switch v.Mahlzeit {
				case "fruehstueck":
					fr = true
				case "mittag":
					mi = true
				case "abend":
					ab = true
				default:
					continue
				}
				next[v.Mahlzeit] = v.Quelle
			}
			raw := ""
			if len(next) > 0 {
				b, err := json.Marshal(next)
				if err != nil {
					return err
				}
				raw = string(b)
			}
			if fr == day.FruehstueckGestellt && mi == day.MittagGestellt && ab == day.AbendGestellt && raw == day.MahlzeitQuelle {
				continue
			}
			updated, err := q.UpdateReisetagMeals(ctx, sqlitedb.UpdateReisetagMealsParams{
				FruehstueckGestellt: fr, MittagGestellt: mi, AbendGestellt: ab, MahlzeitQuelle: raw,
				ReiseID: reiseID, Datum: day.Datum,
			})
			if err != nil {
				return err
			}
			if err := appendAudit(ctx, q, actor, "reisetag.vorschlag", "reisetag", day.ID, mustJSON(day), mustJSON(updated), nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// RememberRates caches ECB rates.
func (s *Store) RememberRates(ctx context.Context, rates []fx.Rate) error {
	if len(rates) == 0 {
		return nil
	}
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		for _, rate := range rates {
			if rate.Kurs == "" || rate.Datum == "" || !fx.ValidCurrency(rate.Waehrung) {
				continue
			}
			if err := q.UpsertWechselkurs(ctx, sqlitedb.UpsertWechselkursParams{
				Datum: rate.Datum, Waehrung: rate.Waehrung, Kurs: rate.Kurs, Quelle: "ezb", AbgerufenAm: now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Rates lists cached daily rates for one currency.
func (s *Store) Rates(ctx context.Context, waehrung string) ([]fx.Rate, error) {
	rows, err := s.readQ().ListWechselkurse(ctx, waehrung)
	if err != nil {
		return nil, err
	}
	out := make([]fx.Rate, 0, len(rows))
	for _, row := range rows {
		out = append(out, fx.Rate{Waehrung: row.Waehrung, Datum: row.Datum, Kurs: row.Kurs})
	}
	return out, nil
}

// AllRates lists every cached daily rate.
func (s *Store) AllRates(ctx context.Context) ([]fx.Rate, error) {
	rows, err := s.readQ().ListWechselkurseAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]fx.Rate, 0, len(rows))
	for _, row := range rows {
		out = append(out, fx.Rate{Waehrung: row.Waehrung, Datum: row.Datum, Kurs: row.Kurs})
	}
	return out, nil
}

// RememberMonth caches one BMF monthly VAT rate.
func (s *Store) RememberMonth(ctx context.Context, jahr, monat int, waehrung, kurs string) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		return q.UpsertUstMonatskurs(ctx, sqlitedb.UpsertUstMonatskursParams{
			Jahr: int64(jahr), Monat: int64(monat), Waehrung: waehrung, Kurs: kurs, AbgerufenAm: time.Now().UTC(),
		})
	})
}

// MonthRate returns a cached BMF rate, then the embedded fixture.
func (s *Store) MonthRate(ctx context.Context, waehrung string, jahr, monat int) (string, bool, error) {
	row, err := s.readQ().GetUstMonatskurs(ctx, sqlitedb.GetUstMonatskursParams{Jahr: int64(jahr), Monat: int64(monat), Waehrung: waehrung})
	if err == nil {
		return row.Kurs, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if kurs, ok := fx.BMF(waehrung, jahr, monat); ok {
		return kurs, true, nil
	}
	return "", false, nil
}

// CreateVorschuss inserts an advance.
func (s *Store) CreateVorschuss(ctx context.Context, nutzerID string, in VorschussInput, actor Actor) (sqlitedb.Vorschuss, error) {
	if err := checkVorschuss(in); err != nil {
		return sqlitedb.Vorschuss{}, err
	}
	var out sqlitedb.Vorschuss
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		if _, err := q.GetArbeitgeber(ctx, sqlitedb.GetArbeitgeberParams{ID: in.ArbeitgeberID, NutzerID: nutzerID}); err != nil {
			if errors.Is(mapErr(err), ErrNotFound) {
				return &CodeError{Code: "arbeitgeber"}
			}
			return err
		}
		now := time.Now().UTC()
		row, err := q.InsertVorschuss(ctx, sqlitedb.InsertVorschussParams{
			ID: id.Must(), NutzerID: nutzerID, ArbeitgeberID: in.ArbeitgeberID, Datum: in.Datum, Betrag: in.Betrag,
			Notiz: emptyNil(in.Notiz), ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "vorschuss.angelegt", "vorschuss", row.ID, nil, mustJSON(row), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// ListVorschuesse lists advances of the Nutzer.
func (s *Store) ListVorschuesse(ctx context.Context, nutzerID string) ([]sqlitedb.Vorschuss, error) {
	return s.readQ().ListVorschuesse(ctx, nutzerID)
}

// UpdateVorschuss edits an advance that is not yet allocated to an Abrechnung.
func (s *Store) UpdateVorschuss(ctx context.Context, nutzerID, vorschussID string, version int64, in VorschussInput, actor Actor) (sqlitedb.Vorschuss, error) {
	if _, err := time.Parse("2006-01-02", in.Datum); err != nil || !centsOK(in.Betrag) || in.Betrag <= 0 {
		return sqlitedb.Vorschuss{}, &CodeError{Code: "vorschuss"}
	}
	var out sqlitedb.Vorschuss
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetVorschuss(ctx, sqlitedb.GetVorschussParams{ID: vorschussID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.AbrechnungID != nil {
			return &CodeError{Code: "vorschuss_verrechnet"}
		}
		row, err := q.UpdateVorschuss(ctx, sqlitedb.UpdateVorschussParams{
			Datum: in.Datum, Betrag: in.Betrag, Notiz: emptyNil(in.Notiz), GeaendertAm: time.Now().UTC(),
			ID: vorschussID, NutzerID: nutzerID, Version: version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if err := appendAudit(ctx, q, actor, "vorschuss.geaendert", "vorschuss", row.ID, mustJSON(prev), mustJSON(row), nil); err != nil {
			return err
		}
		out = row
		return nil
	})
	return out, err
}

// DeleteVorschuss removes an unallocated advance.
func (s *Store) DeleteVorschuss(ctx context.Context, nutzerID, vorschussID string, version int64, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetVorschuss(ctx, sqlitedb.GetVorschussParams{ID: vorschussID, NutzerID: nutzerID})
		if err != nil {
			return mapErr(err)
		}
		if prev.Version != version {
			return ErrConflict
		}
		if prev.AbrechnungID != nil {
			return &CodeError{Code: "vorschuss_verrechnet"}
		}
		n, err := q.DeleteVorschussOffen(ctx, sqlitedb.DeleteVorschussOffenParams{ID: vorschussID, NutzerID: nutzerID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return appendAudit(ctx, q, actor, "vorschuss.geloescht", "vorschuss", vorschussID, mustJSON(prev), nil, nil)
	})
}

func ensureReiseOffen(ctx context.Context, q *sqlitedb.Queries, nutzerID, reiseID string) error {
	row, err := q.GetReise(ctx, sqlitedb.GetReiseParams{ID: reiseID, NutzerID: nutzerID})
	if err != nil {
		return mapErr(err)
	}
	if row.Status == "gesperrt" {
		return &CodeError{Code: "reise_gesperrt"}
	}
	return nil
}

func prepareAusgabe(in *AusgabeInput) error {
	in.Kostenart = strings.TrimSpace(in.Kostenart)
	switch in.Kostenart {
	case "fahrtkosten", "verpflegung", "uebernachtung", "reisenebenkosten", "bewirtung":
	default:
		return &CodeError{Code: "kostenart"}
	}
	if _, err := time.Parse("2006-01-02", in.Datum); err != nil {
		return &CodeError{Code: "datum"}
	}
	if !centsOK(in.Betrag) || in.Betrag <= 0 || !centsOK(in.BetragEUR) {
		return &CodeError{Code: "betrag"}
	}
	in.Waehrung = strings.ToUpper(strings.TrimSpace(in.Waehrung))
	if in.Waehrung == "" {
		in.Waehrung = "EUR"
	}
	if !fx.ValidCurrency(in.Waehrung) {
		return &CodeError{Code: "waehrung"}
	}
	if in.Waehrung == "EUR" {
		in.Kurs, in.KursQuelle, in.KursDatum = "", "", ""
		if in.BetragEUR == 0 {
			in.BetragEUR = in.Betrag
		}
	} else if in.BetragEUR <= 0 {
		return &CodeError{Code: "B04"}
	}
	switch in.KursQuelle {
	case "", "ezb", "manuell", "belastung":
	default:
		return &CodeError{Code: "kurs_quelle"}
	}
	switch in.Rechnungsart {
	case "", "kleinbetragsrechnung":
		in.Rechnungsart = "kleinbetragsrechnung"
	case "rechnung", "fahrausweis", "e_rechnung", "eigenbeleg":
	default:
		return &CodeError{Code: "rechnungsart"}
	}
	if in.Kostenart != "fahrtkosten" {
		in.Verkehrsmittel = ""
	} else if in.Verkehrsmittel != "" && !verkehrAusgabeOK(in.Verkehrsmittel) {
		return &CodeError{Code: "verkehrsmittel"}
	}
	if in.Kostenart != "uebernachtung" {
		in.Naechte = nil
		in.Fruehstueck = false
	}
	if in.Kostenart != "verpflegung" && in.Kostenart != "bewirtung" {
		in.Mahlzeit = ""
	} else if in.Mahlzeit != "" && in.Mahlzeit != "fruehstueck" && in.Mahlzeit != "mittag" && in.Mahlzeit != "abend" {
		return &CodeError{Code: "mahlzeit"}
	}
	if in.Kostenart != "bewirtung" {
		in.Bewirtung = nil
		in.TSE = false
	}
	if len(in.Anteile) == 0 {
		in.Anteile = []AnteilInput{{Satz: 0, Steuerland: "DE", Brutto: in.Betrag, BruttoEUR: in.BetragEUR}}
	}
	var sum, sumEUR int64
	for i := range in.Anteile {
		sh := &in.Anteile[i]
		sh.Steuerland = strings.ToUpper(strings.TrimSpace(sh.Steuerland))
		if sh.Steuerland == "" {
			sh.Steuerland = "DE"
		}
		if sh.Steuerland == "DE" {
			if sh.Satz != 0 && sh.Satz != 700 && sh.Satz != 1900 {
				return &CodeError{Code: "ust_satz"}
			}
		} else if sh.Satz < 0 || sh.Satz > 3000 {
			return &CodeError{Code: "ust_satz"}
		}
		if sh.Brutto < 0 || !centsOK(sh.Brutto) {
			return &CodeError{Code: "betrag"}
		}
		if sh.Netto == nil || sh.Steuer == nil {
			netto, steuer := berechnung.Tax(sh.Brutto, sh.Satz)
			sh.Netto = &netto
			sh.Steuer = &steuer
		}
		if in.Waehrung == "EUR" {
			sh.BruttoEUR = sh.Brutto
			sh.NettoEUR = *sh.Netto
			sh.SteuerEUR = *sh.Steuer
		}
		sum += sh.Brutto
		sumEUR += sh.BruttoEUR
	}
	if sum != in.Betrag {
		return &CodeError{Code: "B02"}
	}
	if in.Waehrung != "EUR" && sumEUR == 0 {
		allocateEUR(in)
		sumEUR = 0
		for _, sh := range in.Anteile {
			sumEUR += sh.BruttoEUR
		}
	}
	if sumEUR != in.BetragEUR {
		return &CodeError{Code: "B02"}
	}
	return nil
}

func allocateEUR(in *AusgabeInput) {
	var allocated int64
	largest := 0
	for i := range in.Anteile {
		sh := &in.Anteile[i]
		sh.BruttoEUR = berechnungEUR(sh.Brutto, in.BetragEUR, in.Betrag)
		sh.SteuerEUR = berechnungEUR(*sh.Steuer, in.BetragEUR, in.Betrag)
		sh.NettoEUR = sh.BruttoEUR - sh.SteuerEUR
		allocated += sh.BruttoEUR
		if sh.Brutto > in.Anteile[largest].Brutto {
			largest = i
		}
	}
	in.Anteile[largest].BruttoEUR += in.BetragEUR - allocated
	in.Anteile[largest].NettoEUR = in.Anteile[largest].BruttoEUR - in.Anteile[largest].SteuerEUR
}

func berechnungEUR(part, eur, betrag int64) int64 {
	if betrag <= 0 {
		return 0
	}
	n, _, _ := splitRound(part*eur, betrag)
	return n
}

func splitRound(n, d int64) (int64, int64, bool) {
	if d == 0 {
		return 0, 0, false
	}
	return (n + d/2) / d, 0, true
}

func verkehrAusgabeOK(s string) bool {
	switch s {
	case "bahn", "flug", "oepnv", "taxi", "mietwagen", "dienstwagen_kraftstoff", "sonstiges":
		return true
	default:
		return false
	}
}

func checkVorschuss(in VorschussInput) error {
	if _, err := time.Parse("2006-01-02", in.Datum); err != nil || !centsOK(in.Betrag) || in.Betrag <= 0 || in.ArbeitgeberID == "" {
		return &CodeError{Code: "vorschuss"}
	}
	return nil
}

func insertAusgabeParams(ausgabeID, nutzerID, reiseID string, in AusgabeInput, now time.Time) sqlitedb.InsertAusgabeParams {
	return sqlitedb.InsertAusgabeParams{
		ID: ausgabeID, NutzerID: nutzerID, ReiseID: reiseID, Kostenart: in.Kostenart, Datum: in.Datum,
		Leistender: strings.TrimSpace(in.Leistender), Beschreibung: emptyNil(in.Beschreibung), Waehrung: in.Waehrung,
		Betrag: in.Betrag, Kurs: emptyNil(in.Kurs), KursQuelle: emptyNil(in.KursQuelle), KursDatum: emptyNil(in.KursDatum),
		KursGrund: emptyNil(in.KursGrund), BetragEur: in.BetragEUR, Rechnungsart: in.Rechnungsart,
		RechnungAufArbeitgeber: in.RechnungAufArbeitgeber, Empfaenger: emptyNil(in.Empfaenger),
		Verkehrsmittel: emptyNil(in.Verkehrsmittel), UebernachtungNaechte: mustNaechte(in.Naechte),
		FruehstueckEnthalten: in.Fruehstueck, Mahlzeit: emptyNil(in.Mahlzeit), Bewirtung: bewirtungPtr(in.Bewirtung),
		TseBeleg: in.TSE, ErstelltAm: now, GeaendertAm: now,
	}
}

func updateAusgabeParams(ausgabeID, nutzerID string, version int64, in AusgabeInput, now time.Time) sqlitedb.UpdateAusgabeParams {
	return sqlitedb.UpdateAusgabeParams{
		Kostenart: in.Kostenart, Datum: in.Datum, Leistender: strings.TrimSpace(in.Leistender),
		Beschreibung: emptyNil(in.Beschreibung), Waehrung: in.Waehrung, Betrag: in.Betrag,
		Kurs: emptyNil(in.Kurs), KursQuelle: emptyNil(in.KursQuelle), KursDatum: emptyNil(in.KursDatum),
		KursGrund: emptyNil(in.KursGrund), BetragEur: in.BetragEUR, Rechnungsart: in.Rechnungsart,
		RechnungAufArbeitgeber: in.RechnungAufArbeitgeber, Empfaenger: emptyNil(in.Empfaenger),
		Verkehrsmittel: emptyNil(in.Verkehrsmittel), UebernachtungNaechte: mustNaechte(in.Naechte),
		FruehstueckEnthalten: in.Fruehstueck, Mahlzeit: emptyNil(in.Mahlzeit), Bewirtung: bewirtungPtr(in.Bewirtung),
		TseBeleg: in.TSE, GeaendertAm: now, ID: ausgabeID, NutzerID: nutzerID, Version: version,
	}
}

func replaceAnteile(ctx context.Context, q *sqlitedb.Queries, ausgabeID string, anteile []AnteilInput) ([]sqlitedb.Steueranteil, error) {
	if err := q.DeleteSteueranteile(ctx, ausgabeID); err != nil {
		return nil, err
	}
	out := make([]sqlitedb.Steueranteil, 0, len(anteile))
	for i, sh := range anteile {
		netto, steuer := int64(0), int64(0)
		if sh.Netto != nil {
			netto = *sh.Netto
		}
		if sh.Steuer != nil {
			steuer = *sh.Steuer
		}
		row, err := q.InsertSteueranteil(ctx, sqlitedb.InsertSteueranteilParams{
			ID: id.Must(), AusgabeID: ausgabeID, Position: int64(i + 1), Satz: sh.Satz, Steuerland: sh.Steuerland,
			Netto: netto, Steuer: steuer, Brutto: sh.Brutto, NettoEur: sh.NettoEUR, SteuerEur: sh.SteuerEUR, BruttoEur: sh.BruttoEUR,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func replaceBelege(ctx context.Context, q *sqlitedb.Queries, ausgabeID string, ids []string) error {
	if err := q.DeleteAusgabeBelege(ctx, ausgabeID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, belegID := range ids {
		if belegID == "" || seen[belegID] {
			continue
		}
		seen[belegID] = true
		if err := q.InsertAusgabeBeleg(ctx, sqlitedb.InsertAusgabeBelegParams{AusgabeID: ausgabeID, BelegID: belegID}); err != nil {
			return err
		}
	}
	return nil
}

func checkBelege(ctx context.Context, q *sqlitedb.Queries, nutzerID string, ids []string) error {
	for _, belegID := range ids {
		if belegID == "" {
			continue
		}
		if _, err := q.GetBeleg(ctx, sqlitedb.GetBelegParams{ID: belegID, NutzerID: nutzerID}); err != nil {
			if errors.Is(mapErr(err), ErrNotFound) {
				return &CodeError{Code: "beleg"}
			}
			return err
		}
	}
	return nil
}

func loadAusgabeBundles(ctx context.Context, q *sqlitedb.Queries, nutzerID, reiseID string) ([]AusgabeBundle, error) {
	rows, err := q.ListAusgabenByReise(ctx, sqlitedb.ListAusgabenByReiseParams{ReiseID: reiseID, NutzerID: nutzerID})
	if err != nil {
		return nil, err
	}
	out := make([]AusgabeBundle, 0, len(rows))
	for _, row := range rows {
		item, err := loadAusgabe(ctx, q, nutzerID, row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func loadAusgabe(ctx context.Context, q *sqlitedb.Queries, nutzerID string, row sqlitedb.Ausgabe) (AusgabeBundle, error) {
	anteile, err := q.ListSteueranteile(ctx, row.ID)
	if err != nil {
		return AusgabeBundle{}, err
	}
	belege, err := q.ListBelegeForAusgabe(ctx, sqlitedb.ListBelegeForAusgabeParams{AusgabeID: row.ID, NutzerID: nutzerID})
	if err != nil {
		return AusgabeBundle{}, err
	}
	var eigen *sqlitedb.Eigenbeleg
	got, err := q.GetEigenbeleg(ctx, row.ID)
	if err == nil {
		eigen = &got
	} else if !errors.Is(err, sql.ErrNoRows) {
		return AusgabeBundle{}, err
	}
	if anteile == nil {
		anteile = []sqlitedb.Steueranteil{}
	}
	if belege == nil {
		belege = []sqlitedb.ListBelegeForAusgabeRow{}
	}
	return AusgabeBundle{Row: row, Anteile: anteile, Belege: belege, Eigen: eigen}, nil
}

func mustNaechte(days []string) string {
	if days == nil {
		days = []string{}
	}
	b, err := json.Marshal(days)
	if err != nil {
		return "[]"
	}
	return string(b)
}

type bewirtungStored struct {
	Anlass        string       `json:"anlass"`
	Teilnehmer    []Teilnehmer `json:"teilnehmer"`
	Ort           string       `json:"ort"`
	Bewirtender   string       `json:"bewirtender"`
	Trinkgeld     int64        `json:"trinkgeld"`
	BestaetigtAm  string       `json:"bestaetigt_am,omitempty"`
	BestaetigtVon string       `json:"bestaetigt_von,omitempty"`
}

func bewirtungJSON(in BewirtungInput) bewirtungStored {
	if in.Teilnehmer == nil {
		in.Teilnehmer = []Teilnehmer{}
	}
	return bewirtungStored{
		Anlass: in.Anlass, Teilnehmer: in.Teilnehmer, Ort: in.Ort, Bewirtender: in.Bewirtender,
		Trinkgeld: in.Trinkgeld, BestaetigtAm: in.BestaetigtAm, BestaetigtVon: in.BestaetigtVon,
	}
}

func bewirtungPtr(in *BewirtungInput) *string {
	if in == nil {
		return nil
	}
	b, err := json.Marshal(bewirtungJSON(*in))
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func decodeBewirtung(raw *string) *BewirtungInput {
	if raw == nil || *raw == "" {
		return nil
	}
	var doc bewirtungStored
	if err := json.Unmarshal([]byte(*raw), &doc); err != nil {
		return nil
	}
	return &BewirtungInput{
		Anlass: doc.Anlass, Ort: doc.Ort, Bewirtender: doc.Bewirtender, Trinkgeld: doc.Trinkgeld,
		Teilnehmer: doc.Teilnehmer, BestaetigtAm: doc.BestaetigtAm, BestaetigtVon: doc.BestaetigtVon,
	}
}

func parseMealQuelle(raw string) map[string]string {
	if raw == "" || raw == "manuell" || !strings.HasPrefix(raw, "{") {
		return map[string]string{}
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]string{}
	}
	return out
}
