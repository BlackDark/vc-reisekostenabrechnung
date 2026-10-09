package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/fx"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) GetAusgaben(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListAusgaben(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	lines := a.lineIndex(r, n.ID)
	items := make([]api.Ausgabe, 0, len(rows))
	for _, row := range rows {
		items = append(items, presentAusgabe(row, lines[row.Row.ID]))
	}
	writeJSON(w, http.StatusOK, api.AusgabeListe{Items: items})
}

func (a *App) PostAusgabe(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.AusgabeWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, err := a.ausgabeFromWrite(r.Context(), body)
	if writeStoreErr(w, err) {
		return
	}
	row, err := a.store.CreateAusgabe(r.Context(), n.ID, id, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	_ = a.store.ApplyMealSuggestions(r.Context(), n.ID, id, a.actor(r, n))
	writeObject(w, http.StatusCreated, row.Row.Version, presentAusgabe(row, a.lineIndex(r, n.ID)[row.Row.ID]))
}

func (a *App) GetAusgabe(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetAusgabe(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Row.Version, presentAusgabe(row, a.lineIndex(r, n.ID)[row.Row.ID]))
}

func (a *App) PatchAusgabe(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchAusgabeParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.AusgabeWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, err := a.ausgabeFromWrite(r.Context(), body)
	if writeStoreErr(w, err) {
		return
	}
	row, err := a.store.UpdateAusgabe(r.Context(), n.ID, id, version, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	_ = a.store.ApplyMealSuggestions(r.Context(), n.ID, row.Row.ReiseID, a.actor(r, n))
	writeObject(w, http.StatusOK, row.Row.Version, presentAusgabe(row, a.lineIndex(r, n.ID)[row.Row.ID]))
}

func (a *App) DeleteAusgabe(w http.ResponseWriter, r *http.Request, id api.Id, params api.DeleteAusgabeParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	prev, err := a.store.GetAusgabe(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	if writeStoreErr(w, a.store.DeleteAusgabe(r.Context(), n.ID, id, version, a.actor(r, n))) {
		return
	}
	_ = a.store.ApplyMealSuggestions(r.Context(), n.ID, prev.Row.ReiseID, a.actor(r, n))
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) PutAusgabeBelege(w http.ResponseWriter, r *http.Request, id api.Id, params api.PutAusgabeBelegeParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.BelegZuordnung
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.SetAusgabeBelege(r.Context(), n.ID, id, version, body.BelegIds, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Row.Version, presentAusgabe(row, a.lineIndex(r, n.ID)[row.Row.ID]))
}

func (a *App) PutEigenbeleg(w http.ResponseWriter, r *http.Request, id api.Id, params api.PutEigenbelegParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.EigenbelegWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.PutEigenbeleg(r.Context(), n.ID, id, version, store.EigenbelegInput{
		Grund: body.Grund, Zahlungsempfaenger: body.Zahlungsempfaenger, Art: body.Art,
	}, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Row.Version, presentAusgabe(row, a.lineIndex(r, n.ID)[row.Row.ID]))
}

func (a *App) PostBewirtungBestaetigen(w http.ResponseWriter, r *http.Request, id api.Id, params api.PostBewirtungBestaetigenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	row, err := a.store.ConfirmBewirtung(r.Context(), n.ID, id, version, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Row.Version, presentAusgabe(row, a.lineIndex(r, n.ID)[row.Row.ID]))
}

func (a *App) GetWechselkurs(w http.ResponseWriter, r *http.Request, params api.GetWechselkursParams) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	waehrung := strings.ToUpper(strings.TrimSpace(params.Waehrung))
	kurs, used, err := a.ecbRate(r.Context(), waehrung, params.Datum)
	if writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, api.Wechselkurs{Waehrung: waehrung, Datum: params.Datum, Kurs: kurs, KursDatum: used, Quelle: "ezb"})
}

func (a *App) GetUstKurs(w http.ResponseWriter, r *http.Request, params api.GetUstKursParams) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	waehrung := strings.ToUpper(strings.TrimSpace(params.Waehrung))
	kurs, ok, err := a.ustRate(r.Context(), waehrung, params.Jahr, params.Monat)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	if !ok {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return
	}
	writeJSON(w, http.StatusOK, api.UstKurs{Waehrung: waehrung, Jahr: params.Jahr, Monat: params.Monat, Kurs: kurs})
}

func (a *App) GetVorschuesse(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListVorschuesse(r.Context(), n.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	items := make([]api.Vorschuss, 0, len(rows))
	for _, row := range rows {
		items = append(items, toVorschuss(row))
	}
	writeJSON(w, http.StatusOK, api.VorschussListe{Items: items})
}

func (a *App) PostVorschuss(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.VorschussWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.CreateVorschuss(r.Context(), n.ID, vorschussFromWrite(body), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusCreated, row.Version, toVorschuss(row))
}

func (a *App) PatchVorschuss(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchVorschussParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	var body api.VorschussWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.UpdateVorschuss(r.Context(), n.ID, id, version, vorschussFromWrite(body), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, toVorschuss(row))
}

func (a *App) DeleteVorschuss(w http.ResponseWriter, r *http.Request, id api.Id, params api.DeleteVorschussParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, ok := a.matchVersion(w, params.IfMatch)
	if !ok {
		return
	}
	if writeStoreErr(w, a.store.DeleteVorschuss(r.Context(), n.ID, id, version, a.actor(r, n))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) PostMwstHelfer(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var body api.MwstHelfer
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.BetragCent <= 0 {
		writeProblem(w, http.StatusUnprocessableEntity, "betrag", "Check the amount", "")
		return
	}
	year := time.Now().Year()
	if body.Jahr != nil {
		year = *body.Jahr
	}
	var shares []berechnung.SteuerInput
	switch body.Art {
	case "gastronomie":
		shares = berechnung.SplitGastronomie(body.BetragCent)
	case "hotel":
		shares = berechnung.SplitHotel(body.BetragCent, year)
	default:
		writeProblem(w, http.StatusUnprocessableEntity, "art", "Check the helper", "")
		return
	}
	out := make([]api.SteueranteilWrite, 0, len(shares))
	for _, sh := range shares {
		out = append(out, api.SteueranteilWrite{Satz: sh.Satz, Steuerland: sh.Steuerland, BruttoCent: sh.Brutto})
	}
	writeJSON(w, http.StatusOK, api.MwstHelferAntwort{Anteile: out})
}

func (a *App) matchVersion(w http.ResponseWriter, header *string) (int64, bool) {
	version, present, valid := ifMatchVersion(header)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return 0, false
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return 0, false
	}
	return version, true
}

func (a *App) ausgabeFromWrite(ctx context.Context, body api.AusgabeWrite) (store.AusgabeInput, error) {
	in := store.AusgabeInput{
		Kostenart: body.Kostenart, Datum: body.Datum, Waehrung: strings.ToUpper(body.Waehrung), Betrag: body.BetragCent,
		Leistender: deref(body.Leistender), Beschreibung: deref(body.Beschreibung), Kurs: deref(body.Kurs),
		KursQuelle: deref(body.KursQuelle), KursGrund: deref(body.KursGrund), Rechnungsart: deref(body.Rechnungsart),
		Empfaenger: deref(body.Empfaenger), Verkehrsmittel: deref(body.Verkehrsmittel), Mahlzeit: deref(body.Mahlzeit),
		RechnungAufArbeitgeber: boolVal(body.RechnungAufArbeitgeber), Fruehstueck: boolVal(body.FruehstueckEnthalten),
		TSE: boolVal(body.TseBeleg),
	}
	if body.BetragEurCent != nil {
		in.BetragEUR = *body.BetragEurCent
	}
	if body.UebernachtungNaechte != nil {
		in.Naechte = *body.UebernachtungNaechte
	}
	if body.BelegIds != nil {
		in.BelegIDs = *body.BelegIds
	}
	if body.Anteile != nil {
		for _, sh := range *body.Anteile {
			in.Anteile = append(in.Anteile, store.AnteilInput{Satz: sh.Satz, Steuerland: sh.Steuerland, Brutto: sh.BruttoCent, Netto: sh.NettoCent, Steuer: sh.SteuerCent})
		}
	}
	if body.Bewirtung != nil {
		in.Bewirtung = bewirtungFromAPI(body.Bewirtung)
	}
	if err := a.fillMoney(ctx, &in); err != nil {
		return store.AusgabeInput{}, err
	}
	return in, nil
}

func bewirtungFromAPI(b *api.Bewirtung) *store.BewirtungInput {
	in := &store.BewirtungInput{
		Anlass: deref(b.Anlass), Ort: deref(b.Ort), Bewirtender: deref(b.Bewirtender),
		BestaetigtAm: deref(b.BestaetigtAm),
	}
	if b.TrinkgeldCent != nil {
		in.Trinkgeld = *b.TrinkgeldCent
	}
	if b.Teilnehmer != nil {
		for _, p := range *b.Teilnehmer {
			in.Teilnehmer = append(in.Teilnehmer, store.Teilnehmer{Name: p.Name, Firma: deref(p.Firma)})
		}
	}
	return in
}

func (a *App) fillMoney(ctx context.Context, in *store.AusgabeInput) error {
	if in.Waehrung == "" || in.Waehrung == "EUR" {
		in.Waehrung = "EUR"
		in.BetragEUR = in.Betrag
		in.Kurs, in.KursQuelle, in.KursDatum = "", "", ""
		return nil
	}
	switch in.KursQuelle {
	case "belastung":
		if in.BetragEUR <= 0 {
			return &store.CodeError{Code: "B04"}
		}
		in.Kurs = berechnung.ImplicitKurs(in.Betrag, in.BetragEUR)
		in.KursDatum = in.Datum
	case "manuell":
		if in.Kurs == "" {
			return &store.CodeError{Code: "B04"}
		}
		eur, err := berechnung.EURFromKurs(in.Betrag, in.Kurs)
		if err != nil {
			return &store.CodeError{Code: "B04"}
		}
		in.BetragEUR = eur
		in.KursDatum = in.Datum
	default:
		in.KursQuelle = "ezb"
		kurs, used, err := a.ecbRate(ctx, in.Waehrung, in.Datum)
		if err != nil {
			return err
		}
		in.Kurs, in.KursDatum = kurs, used
		eur, err := berechnung.EURFromKurs(in.Betrag, kurs)
		if err != nil {
			return &store.CodeError{Code: "B04"}
		}
		in.BetragEUR = eur
		_ = a.store.RememberRates(ctx, []fx.Rate{{Waehrung: in.Waehrung, Datum: used, Kurs: kurs}})
	}
	return nil
}

func (a *App) ecbRate(ctx context.Context, waehrung, datum string) (string, string, error) {
	if !fx.ValidCurrency(waehrung) {
		return "", "", &store.CodeError{Code: "waehrung"}
	}
	cached, err := a.store.Rates(ctx, waehrung)
	if err != nil {
		return "", "", err
	}
	all := append(append([]fx.Rate{}, cached...), fx.EmbeddedECB()...)
	if kurs, used, ok := fx.Lookback(all, waehrung, datum); ok {
		return kurs, used, nil
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	fetched, ferr := fx.FetchECB(fetchCtx, nil, waehrung, fx.ShiftDate(datum, -7), datum)
	if ferr == nil && len(fetched) > 0 {
		_ = a.store.RememberRates(ctx, fetched)
		all = append(all, fetched...)
		if kurs, used, ok := fx.Lookback(all, waehrung, datum); ok {
			return kurs, used, nil
		}
	}
	return "", "", &store.CodeError{Code: "B04"}
}

func (a *App) ustRate(ctx context.Context, waehrung string, jahr, monat int) (string, bool, error) {
	kurs, ok, err := a.store.MonthRate(ctx, waehrung, jahr, monat)
	if err != nil || ok {
		return kurs, ok, err
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	rows, ferr := fx.FetchBMF(fetchCtx, nil)
	if ferr != nil {
		return "", false, nil
	}
	for _, row := range rows {
		_ = a.store.RememberMonth(ctx, row.Jahr, row.Monat, row.Waehrung, row.Kurs)
		if row.Waehrung == waehrung && row.Jahr == jahr && row.Monat == monat {
			kurs, ok = row.Kurs, true
		}
	}
	return kurs, ok, nil
}

func (a *App) attachMoney(r *http.Request, nutzerID string, in *berechnung.Eingabe) error {
	rates, err := a.store.AllRates(r.Context())
	if err != nil {
		return err
	}
	for _, rate := range rates {
		in.Kurse = append(in.Kurse, berechnung.Kurs{Waehrung: rate.Waehrung, Datum: rate.Datum, Kurs: rate.Kurs})
	}
	for _, rate := range fx.EmbeddedECB() {
		in.Kurse = append(in.Kurse, berechnung.Kurs{Waehrung: rate.Waehrung, Datum: rate.Datum, Kurs: rate.Kurs})
	}
	employers := map[string]string{}
	for i := range in.Reisen {
		trip := &in.Reisen[i]
		if nutzerID != "" && trip.ArbeitgeberID != "" {
			if name, ok := employers[trip.ArbeitgeberID]; ok {
				trip.ArbeitgeberName = name
			} else if ag, err := a.store.GetArbeitgeber(r.Context(), nutzerID, trip.ArbeitgeberID); err == nil {
				employers[trip.ArbeitgeberID] = ag.Name
				trip.ArbeitgeberName = ag.Name
			}
		}
		for j := range trip.Ausgaben {
			line := &trip.Ausgaben[j]
			if line.Waehrung != "" && line.Waehrung != "EUR" && line.Kurs != "" && line.KursDatum != "" {
				in.Kurse = append(in.Kurse, berechnung.Kurs{Waehrung: line.Waehrung, Datum: line.KursDatum, Kurs: line.Kurs})
			}
			if line.Waehrung == "" || line.Waehrung == "EUR" {
				continue
			}
			when, err := time.Parse("2006-01-02", line.Datum)
			if err != nil {
				continue
			}
			kurs, ok, err := a.store.MonthRate(r.Context(), line.Waehrung, when.Year(), int(when.Month()))
			if err != nil {
				return err
			}
			if ok {
				line.Monatskurs = kurs
			}
		}
	}
	return nil
}

func (a *App) lineIndex(r *http.Request, nutzerID string) map[string]berechnung.AusgabeErgebnis {
	bundles, err := a.store.ListReisenAll(r.Context(), nutzerID)
	if err != nil {
		return nil
	}
	result, _, _, err := a.calculate(r, bundles)
	if err != nil {
		return nil
	}
	out := map[string]berechnung.AusgabeErgebnis{}
	for _, re := range result {
		for _, line := range re.Ausgaben {
			out[line.ID] = line
		}
	}
	return out
}

func presentAusgabe(item store.AusgabeBundle, calc berechnung.AusgabeErgebnis) api.Ausgabe {
	row := item.Row
	view := api.Ausgabe{
		Id: row.ID, ReiseId: row.ReiseID, Kostenart: row.Kostenart, Datum: row.Datum, Waehrung: row.Waehrung,
		BetragCent: row.Betrag, BetragEurCent: row.BetragEur, Rechnungsart: row.Rechnungsart,
		RechnungAufArbeitgeber: row.RechnungAufArbeitgeber, Version: row.Version,
		Leistender: strPtr(row.Leistender), Beschreibung: row.Beschreibung, Kurs: row.Kurs, KursQuelle: row.KursQuelle,
		KursDatum: row.KursDatum, KursGrund: row.KursGrund, Empfaenger: row.Empfaenger, Verkehrsmittel: row.Verkehrsmittel,
		Mahlzeit: row.Mahlzeit, TseBeleg: &row.TseBeleg, FruehstueckEnthalten: &row.FruehstueckEnthalten,
	}
	var nights []string
	_ = json.Unmarshal([]byte(row.UebernachtungNaechte), &nights)
	if nights == nil {
		nights = []string{}
	}
	view.UebernachtungNaechte = &nights
	ids := make([]string, 0, len(item.Belege))
	for _, beleg := range item.Belege {
		ids = append(ids, beleg.ID)
	}
	view.BelegIds = ids
	view.Anteile = make([]api.Steueranteil, 0, len(item.Anteile))
	for i, sh := range item.Anteile {
		part := api.Steueranteil{
			Satz: sh.Satz, Steuerland: sh.Steuerland, BruttoCent: sh.Brutto, NettoCent: sh.Netto, SteuerCent: sh.Steuer,
			BruttoEurCent: sh.BruttoEur, NettoEurCent: sh.NettoEur, SteuerEurCent: sh.SteuerEur,
		}
		if i < len(calc.Anteile) {
			flag := calc.Anteile[i].Vorsteuer
			part.Vorsteuer = &flag
			part.BruttoEurCent = calc.Anteile[i].BruttoEUR
			part.NettoEurCent = calc.Anteile[i].NettoEUR
			part.SteuerEurCent = calc.Anteile[i].SteuerEUR
		}
		view.Anteile = append(view.Anteile, part)
	}
	if calc.ID != "" {
		if calc.Monatskurs != "" {
			view.UstKurs = &calc.Monatskurs
		}
		if len(calc.Warnungen) > 0 {
			view.Warnungen = &calc.Warnungen
		}
		if calc.BetragEUR > 0 {
			view.BetragEurCent = calc.BetragEUR
		}
		if calc.Kurs != "" {
			view.Kurs = &calc.Kurs
			view.KursDatum = strPtr(calc.KursDatum)
		}
	}
	if doc := decodeBewirtungView(row.Bewirtung); doc != nil {
		view.Bewirtung = doc
	}
	if item.Eigen != nil {
		view.Eigenbeleg = &api.Eigenbeleg{Grund: item.Eigen.Grund, Zahlungsempfaenger: item.Eigen.Zahlungsempfaenger, Art: item.Eigen.Art}
		if item.Eigen.BestaetigtAm != nil {
			s := item.Eigen.BestaetigtAm.UTC().Format(time.RFC3339)
			view.Eigenbeleg.BestaetigtAm = &s
		}
	}
	return view
}

func decodeBewirtungView(raw *string) *api.Bewirtung {
	if raw == nil || *raw == "" {
		return nil
	}
	var doc struct {
		Anlass       string `json:"anlass"`
		Ort          string `json:"ort"`
		Bewirtender  string `json:"bewirtender"`
		Trinkgeld    int64  `json:"trinkgeld"`
		BestaetigtAm string `json:"bestaetigt_am"`
		Teilnehmer   []struct {
			Name  string `json:"name"`
			Firma string `json:"firma"`
		} `json:"teilnehmer"`
	}
	if err := json.Unmarshal([]byte(*raw), &doc); err != nil {
		return nil
	}
	out := &api.Bewirtung{}
	if doc.Anlass != "" {
		out.Anlass = &doc.Anlass
	}
	if doc.Ort != "" {
		out.Ort = &doc.Ort
	}
	if doc.Bewirtender != "" {
		out.Bewirtender = &doc.Bewirtender
	}
	if doc.BestaetigtAm != "" {
		out.BestaetigtAm = &doc.BestaetigtAm
	}
	if doc.Trinkgeld != 0 {
		out.TrinkgeldCent = &doc.Trinkgeld
	}
	people := make([]api.Teilnehmer, 0, len(doc.Teilnehmer))
	for _, p := range doc.Teilnehmer {
		person := api.Teilnehmer{Name: p.Name}
		if p.Firma != "" {
			person.Firma = &p.Firma
		}
		people = append(people, person)
	}
	out.Teilnehmer = &people
	return out
}

func toVorschuss(row sqlitedb.Vorschuss) api.Vorschuss {
	done := row.AbrechnungID != nil
	return api.Vorschuss{
		Id: row.ID, ArbeitgeberId: row.ArbeitgeberID, Datum: row.Datum, BetragCent: row.Betrag,
		Notiz: row.Notiz, Verrechnet: &done, Version: row.Version,
	}
}

func vorschussFromWrite(body api.VorschussWrite) store.VorschussInput {
	return store.VorschussInput{ArbeitgeberID: body.ArbeitgeberId, Datum: body.Datum, Betrag: body.BetragCent, Notiz: deref(body.Notiz)}
}

func fuzzyAusgaben(bundles []store.ReiseBundle) []api.Warnung {
	type key struct {
		datum, name string
		betrag      int64
	}
	seen := map[key]string{}
	var out []api.Warnung
	for _, b := range bundles {
		for _, item := range b.Ausgaben {
			name := strings.ToLower(strings.TrimSpace(item.Row.Leistender))
			if name == "" {
				continue
			}
			k := key{item.Row.Datum, name, item.Row.Betrag}
			if prev, ok := seen[k]; ok && prev != item.Row.ID {
				id := item.Row.ID
				out = append(out, api.Warnung{Code: "W04", ReiseId: b.Reise.ID, Anlass: b.Reise.Anlass, AusgabeId: &id})
				continue
			}
			seen[k] = item.Row.ID
		}
	}
	return out
}

func boolVal(v *bool) bool { return v != nil && *v }
