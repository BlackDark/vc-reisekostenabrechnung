package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

func (a *App) GetReisen(w http.ResponseWriter, r *http.Request, params api.GetReisenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListReisen(r.Context(), n.ID, store.ReiseFilter{
		Status: deref(params.Status), ArbeitgeberID: deref(params.ArbeitgeberId), Projekt: deref(params.Projekt),
		Q: deref(params.Q), Von: dayBound(deref(params.Von), false), Bis: dayBound(deref(params.Bis), true),
		Cursor: params.Cursor, Limit: limitOf(params.Limit) + 1,
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	limit := int(limitOf(params.Limit))
	var next *string
	if len(rows) > limit {
		id := rows[limit-1].Reise.ID
		next = &id
		rows = rows[:limit]
	}
	items := make([]api.Reise, 0, len(rows))
	for _, row := range rows {
		items = append(items, toReise(row))
	}
	writeJSON(w, http.StatusOK, api.ReiseListe{Items: items, NextCursor: next})
}

func (a *App) PostReise(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.ReiseWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, ok := reiseFromWrite(body)
	if !ok {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check the trip", "")
		return
	}
	row, err := a.store.CreateReise(r.Context(), n.ID, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusCreated, row.Reise.Version, toReise(row))
}

func (a *App) GetReise(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetReiseBundle(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Reise.Version, toReise(row))
}

func (a *App) PatchReise(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchReiseParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.ReiseWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	in, ok := reiseFromWrite(body)
	if !ok {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check the trip", "")
		return
	}
	row, err := a.store.UpdateReise(r.Context(), n.ID, id, version, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Reise.Version, toReise(row))
}

func (a *App) DeleteReise(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if writeStoreErr(w, a.store.DeleteReise(r.Context(), n.ID, id, a.actor(r, n))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetReiseBerechnung(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	view, _, err := a.preview(r, n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (a *App) PatchReisetag(w http.ResponseWriter, r *http.Request, id api.Id, datum string, params api.PatchReisetagParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.ReisetagPatch
	if !decodeJSON(w, r, &body) {
		return
	}
	in := store.ReisetagInput{
		LandISO: deref(body.LandIso), Satzort: deref(body.Satzort), Begruendung: deref(body.Begruendung),
		Fruehstueck: body.FruehstueckGestellt, Mittag: body.MittagGestellt, Abend: body.AbendGestellt,
		ZuzahlungFruehstueck: val64(body.ZuzahlungFruehstueck), ZuzahlungMittag: val64(body.ZuzahlungMittag), ZuzahlungAbend: val64(body.ZuzahlungAbend),
		Unterkunft: body.Unterkunft, VerpflegungAusgeschlossen: body.VerpflegungAusgeschlossen != nil && *body.VerpflegungAusgeschlossen,
		AusschlussGrund: deref(body.AusschlussGrund),
	}
	row, err := a.store.PatchReisetag(r.Context(), n.ID, id, datum, version, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Reise.Version, toReise(row))
}

func (a *App) GetFahrten(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListFahrten(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	items := make([]api.Fahrt, 0, len(rows))
	for _, row := range rows {
		items = append(items, a.toFahrt(r, row))
	}
	writeJSON(w, http.StatusOK, api.FahrtListe{Items: items})
}

func (a *App) PostFahrt(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.FahrtWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.CreateFahrt(r.Context(), n.ID, id, fahrtFromWrite(body), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusCreated, row.Version, a.toFahrt(r, row))
}

func (a *App) PatchFahrt(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchFahrtParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.FahrtWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	row, err := a.store.UpdateFahrt(r.Context(), n.ID, id, version, fahrtFromWrite(body), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, a.toFahrt(r, row))
}

func (a *App) DeleteFahrt(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if writeStoreErr(w, a.store.DeleteFahrt(r.Context(), n.ID, id, a.actor(r, n))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) GetVorlagen(w http.ResponseWriter, r *http.Request, params api.GetVorlagenParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	rows, err := a.store.ListVorlagen(r.Context(), n.ID, deref(params.Art), params.Cursor, limitOf(params.Limit)+1)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	limit := int(limitOf(params.Limit))
	var next *string
	if len(rows) > limit {
		id := rows[limit-1].ID
		next = &id
		rows = rows[:limit]
	}
	items := make([]api.Vorlage, 0, len(rows))
	for _, row := range rows {
		items = append(items, toVorlage(row))
	}
	writeJSON(w, http.StatusOK, api.VorlageListe{Items: items, NextCursor: next})
}

func (a *App) PostVorlage(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var body api.VorlageWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	raw, err := json.Marshal(body.Daten)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check the template", "")
		return
	}
	row, err := a.store.CreateVorlage(r.Context(), n.ID, body.Name, body.Art, string(raw), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusCreated, row.Version, toVorlage(row))
}

func (a *App) GetVorlage(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	row, err := a.store.GetVorlage(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, toVorlage(row))
}

func (a *App) PatchVorlage(w http.ResponseWriter, r *http.Request, id api.Id, params api.PatchVorlageParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	version, present, valid := ifMatchVersion(params.IfMatch)
	if !present {
		writeProblem(w, http.StatusPreconditionRequired, "if_match", "If-Match is required", "")
		return
	}
	if !valid {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return
	}
	var body api.VorlageWrite
	if !decodeJSON(w, r, &body) {
		return
	}
	raw, err := json.Marshal(body.Daten)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Check the template", "")
		return
	}
	row, err := a.store.UpdateVorlage(r.Context(), n.ID, id, version, body.Name, string(raw), a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	writeObject(w, http.StatusOK, row.Version, toVorlage(row))
}

func (a *App) DeleteVorlage(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if writeStoreErr(w, a.store.DeleteVorlage(r.Context(), n.ID, id, a.actor(r, n))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) PostVorlageAnwenden(w http.ResponseWriter, r *http.Request, id api.Id) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	vorlage, err := a.store.GetVorlage(r.Context(), n.ID, id)
	if writeStoreErr(w, err) {
		return
	}
	var body api.VorlageAnwenden
	if !decodeJSON(w, r, &body) {
		return
	}
	var daten map[string]any
	if err := json.Unmarshal([]byte(vorlage.Daten), &daten); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validierung", "Template is not readable", "")
		return
	}
	if vorlage.Art == "fahrt" {
		fahrt, err := a.applyFahrt(r, n, vorlage.ID, daten, body)
		if writeStoreErr(w, err) {
			return
		}
		view := a.toFahrt(r, fahrt)
		writeJSON(w, http.StatusCreated, api.VorlageAnwendung{Fahrt: &view})
		return
	}
	in := reiseFromTemplate(daten, body, vorlage.ID)
	row, err := a.store.CreateReise(r.Context(), n.ID, in, a.actor(r, n))
	if writeStoreErr(w, err) {
		return
	}
	view := toReise(row)
	writeJSON(w, http.StatusCreated, api.VorlageAnwendung{Reise: &view})
}

func (a *App) GetProjekte(w http.ResponseWriter, r *http.Request, params api.GetProjekteParams) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	items, err := a.store.ListProjekte(r.Context(), n.ID, deref(params.Q), 20)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	if items == nil {
		items = []string{}
	}
	writeJSON(w, http.StatusOK, api.ProjektListe{Items: items})
}

func (a *App) GetWarnungen(w http.ResponseWriter, r *http.Request) {
	n, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	items, err := a.warnings(r, n.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
		return
	}
	writeJSON(w, http.StatusOK, api.WarnungListe{Items: items})
}

func (a *App) warnings(r *http.Request, nutzerID string) ([]api.Warnung, error) {
	bundles, err := a.store.ListReisenAll(r.Context(), nutzerID)
	if err != nil {
		return nil, err
	}
	result, frist, names, err := a.calculate(r, bundles)
	if err != nil {
		return nil, err
	}
	var items []api.Warnung
	for _, b := range bundles {
		re := result[b.Reise.ID]
		seen := map[string]bool{}
		for _, line := range re.Ausgaben {
			id := line.ID
			for _, code := range line.Warnungen {
				items = append(items, api.Warnung{Code: code, ReiseId: b.Reise.ID, Anlass: b.Reise.Anlass, AusgabeId: &id})
				seen[code] = true
			}
		}
		for _, code := range append(append([]string{}, re.Warnungen...), re.Blocker...) {
			if seen[code] {
				continue
			}
			seen[code] = true
			items = append(items, api.Warnung{Code: code, ReiseId: b.Reise.ID, Anlass: b.Reise.Anlass})
		}
		cover := stayCover(b, names)
		for _, w := range frist {
			if w.Code != "W03" || !cover[w.Datum][w.Staette] {
				continue
			}
			for _, day := range b.Tage {
				if day.Datum == w.Datum {
					d, name := w.Datum, w.Staette
					items = append(items, api.Warnung{Code: "W03", ReiseId: b.Reise.ID, Anlass: b.Reise.Anlass, Datum: &d, Staette: &name})
				}
			}
		}
	}
	dups, err := a.store.ListDuplikatBelege(r.Context(), nutzerID)
	if err != nil {
		return nil, err
	}
	for _, d := range dups {
		if d.DuplikatVon == nil {
			continue
		}
		existing := *d.DuplikatVon
		items = append(items, api.Warnung{Code: "W04", ReiseId: "", Anlass: d.ID, BelegId: &existing})
	}
	items = append(items, fuzzyAusgaben(bundles)...)
	if items == nil {
		items = []api.Warnung{}
	}
	return items, nil
}

func (a *App) preview(r *http.Request, nutzerID, reiseID string) (api.Berechnung, []berechnung.FristWarnung, error) {
	bundles, err := a.store.ListReisenAll(r.Context(), nutzerID)
	if err != nil {
		return api.Berechnung{}, nil, err
	}
	var target *store.ReiseBundle
	for i := range bundles {
		if bundles[i].Reise.ID == reiseID {
			target = &bundles[i]
		}
	}
	if target == nil {
		return api.Berechnung{}, nil, store.ErrNotFound
	}
	result, frist, names, err := a.calculate(r, bundles)
	if err != nil {
		return api.Berechnung{}, nil, err
	}
	cover := stayCover(*target, names)
	re := result[reiseID]
	view := api.Berechnung{
		ReiseId: reiseID, FahrtkostenCent: re.Fahrtkosten, VerpflegungCent: re.Verpflegung,
		UebernachtungCent: re.Uebernachtung, ReisenebenkostenCent: re.Reisenebenkosten,
		BewirtungCent: re.Bewirtung, SummeCent: re.Summe,
		VorsteuerCent: &re.Vorsteuer, BewirtungAbziehbarCent: &re.BewirtungAbziehbar,
		BewirtungNichtAbziehbarCent: &re.BewirtungNichtAbziehbar,
	}
	if len(re.Blocker) > 0 {
		view.Blocker = &re.Blocker
	}
	warns := append([]string{}, re.Warnungen...)
	for _, day := range re.Tage {
		var kuerz int64
		for _, k := range day.Kuerzungen {
			kuerz += k.Betrag
		}
		tag := api.BerechnungTag{
			Datum: day.Datum, Tagesart: day.Tagesart, LandIso: day.LandISO, ErgebnisCent: day.Ergebnis,
			PauschaleCent: day.Pauschale, UebernachtungCent: day.Uebernachtung, KuerzungCent: &kuerz,
			AbwesenheitMinuten: &day.AbwesenheitMin,
		}
		if day.Satzort != "" {
			tag.Satzort = &day.Satzort
		}
		if day.LandRegel != "" {
			tag.LandRegel = &day.LandRegel
		}
		if len(day.RegelIDs) > 0 {
			tag.RegelIds = &day.RegelIDs
		}
		if len(day.Hinweise) > 0 {
			codes := make([]string, 0, len(day.Hinweise))
			for _, h := range day.Hinweise {
				codes = append(codes, h.Code)
			}
			tag.Hinweise = &codes
		}
		var dayWarn []string
		dayWarn = append(dayWarn, day.Warnungen...)
		for _, w := range frist {
			if w.Datum == day.Datum && cover[day.Datum][w.Staette] {
				dayWarn = append(dayWarn, "W03")
				warns = append(warns, "W03")
			}
		}
		if len(dayWarn) > 0 {
			tag.Warnungen = &dayWarn
		}
		view.Tage = append(view.Tage, tag)
	}
	if len(warns) > 0 {
		view.Warnungen = &warns
	}
	if view.Tage == nil {
		view.Tage = []api.BerechnungTag{}
	}
	return view, frist, nil
}

func (a *App) calculate(r *http.Request, bundles []store.ReiseBundle) (map[string]berechnung.ReiseErgebnis, []berechnung.FristWarnung, map[string]string, error) {
	years := map[int]*satz.Year{}
	in := berechnung.Eingabe{Jahre: years}
	names := map[string]string{}
	var visits []berechnung.Visit
	for _, b := range bundles {
		trip := bundleToCalc(b)
		in.Reisen = append(in.Reisen, trip)
		for _, day := range b.Tage {
			y := yearOf(day.Datum)
			if _, ok := years[y]; ok || y == 0 {
				continue
			}
			loaded, err := a.store.EffectiveYear(r.Context(), y)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, nil, nil, err
			}
			years[y] = loaded
		}
		for _, f := range b.Fahrten {
			if err := a.ensureYear(r, years, yearOf(f.Datum)); err != nil {
				return nil, nil, nil, err
			}
		}
		for _, item := range b.Ausgaben {
			if err := a.ensureYear(r, years, yearOf(item.Row.Datum)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := a.rememberStaetten(r, b, names); err != nil {
			return nil, nil, nil, err
		}
		visits = append(visits, stays(b, names)...)
	}
	nutzerID := ""
	if len(bundles) > 0 {
		nutzerID = bundles[0].Reise.NutzerID
	}
	if err := a.attachMoney(r, nutzerID, &in); err != nil {
		return nil, nil, nil, err
	}
	got, err := berechnung.Berechne(in)
	if err != nil {
		return nil, nil, nil, err
	}
	out := map[string]berechnung.ReiseErgebnis{}
	for _, re := range got.Reisen {
		out[re.ID] = re
	}
	return out, berechnung.Dreimonatsfrist(visits), names, nil
}

func (a *App) rememberStaetten(r *http.Request, b store.ReiseBundle, names map[string]string) error {
	for _, leg := range b.Legs {
		if leg.TaetigkeitsstaetteID == nil {
			continue
		}
		id := *leg.TaetigkeitsstaetteID
		if _, ok := names[id]; ok {
			continue
		}
		st, err := a.store.GetTaetigkeit(r.Context(), b.Reise.NutzerID, id)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		names[st.ID] = st.Bezeichnung
	}
	return nil
}

// stays lists every calendar day from a fixed Tätigkeitsstätte's arrival
// through the next Ortswechsel (or the trip end). Flight and ship legs are
// not a fixed place (SPEC 4.14).
func stays(b store.ReiseBundle, names map[string]string) []berechnung.Visit {
	var out []berechnung.Visit
	tripEnd := dateIn(b.Reise.Ende, b.Reise.EndeZone)
	for i, leg := range b.Legs {
		if leg.TaetigkeitsstaetteID == nil || leg.Verkehrsmittel == "flug" || leg.Verkehrsmittel == "schiff" {
			continue
		}
		name := names[*leg.TaetigkeitsstaetteID]
		if name == "" {
			name = *leg.TaetigkeitsstaetteID
		}
		from := dateIn(leg.Ankunft, leg.AnkunftZone)
		to := tripEnd
		if i+1 < len(b.Legs) {
			to = dateIn(b.Legs[i+1].Ankunft, b.Legs[i+1].AnkunftZone)
		}
		if from == "" {
			continue
		}
		if to == "" || from > to {
			to = from
		}
		for d := from; ; d = nextCivil(d) {
			out = append(out, berechnung.Visit{Staette: name, Datum: d})
			if d >= to {
				break
			}
		}
	}
	return out
}

func stayCover(b store.ReiseBundle, names map[string]string) map[string]map[string]bool {
	cover := map[string]map[string]bool{}
	for _, v := range stays(b, names) {
		if cover[v.Datum] == nil {
			cover[v.Datum] = map[string]bool{}
		}
		cover[v.Datum][v.Staette] = true
	}
	return cover
}

func dateIn(t time.Time, zone string) string {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return ""
	}
	return t.In(loc).Format("2006-01-02")
}

func nextCivil(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

func bundleToCalc(b store.ReiseBundle) berechnung.Reise {
	return store.CalcReise(b)
}

func (a *App) applyFahrt(r *http.Request, n sqlitedb.Nutzer, vorlageID string, daten map[string]any, body api.VorlageAnwenden) (sqlitedb.Fahrt, error) {
	reiseID := deref(body.ReiseId)
	datum := deref(body.Datum)
	if reiseID == "" || datum == "" {
		return sqlitedb.Fahrt{}, &store.CodeError{Code: "validierung"}
	}
	return a.store.CreateFahrt(r.Context(), n.ID, reiseID, store.FahrtInput{
		Datum: datum, Start: strAny(daten["start"]), Ziel: strAny(daten["ziel"]), Zweck: strAny(daten["zweck"]),
		Fahrzeugart: strAny(daten["fahrzeugart"]), Km: int64Any(daten["km"]), HinUndZurueck: boolAny(daten["hin_und_zurueck"]),
		VorlageID: vorlageID,
	}, a.actor(r, n))
}

func reiseFromTemplate(daten map[string]any, body api.VorlageAnwenden, vorlageID string) store.ReiseInput {
	zone := deref(body.BeginnZone)
	if zone == "" {
		zone = "Europe/Berlin"
	}
	beginn := deref(body.Beginn)
	if beginn == "" {
		beginn = time.Now().In(mustLoc(zone)).Format("2006-01-02T15:04:05")
	}
	ende := deref(body.Ende)
	endeZone := deref(body.EndeZone)
	if endeZone == "" {
		endeZone = zone
	}
	if ende == "" {
		start, err := time.ParseInLocation("2006-01-02T15:04:05", trimSec(beginn), mustLoc(zone))
		if err != nil {
			start = time.Now()
		}
		minutes := int64Any(daten["dauer_minuten"])
		if minutes <= 0 {
			minutes = 8 * 60
		}
		ende = start.Add(time.Duration(minutes) * time.Minute).Format("2006-01-02T15:04:05")
	}
	in := store.ReiseInput{
		ArbeitgeberID: strAny(daten["arbeitgeber_id"]), Anlass: strAny(daten["anlass"]), Projekt: strAny(daten["projekt"]),
		BeginnLokal: beginn, BeginnZone: zone, EndeLokal: ende, EndeZone: endeZone,
		Notiz: strAny(daten["notiz"]), VorlageID: vorlageID, UnterkunftDefault: strAny(daten["unterkunft"]),
	}
	start, _ := time.ParseInLocation("2006-01-02T15:04:05", trimSec(beginn), mustLoc(zone))
	if legs, ok := daten["ortswechsel"].([]any); ok {
		for _, raw := range legs {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			offset := int64Any(m["offset_minuten"])
			ankunft := start.Add(time.Duration(offset) * time.Minute).Format("2006-01-02T15:04:05")
			in.Ortswechsel = append(in.Ortswechsel, store.OrtswechselInput{
				AnkunftLokal: ankunft, AnkunftZone: zone, Verkehrsmittel: strAny(m["verkehrsmittel"]),
				LandISO: strAny(m["land_iso"]), Satzort: strAny(m["satzort"]), Ort: strAny(m["ort"]),
				TaetigkeitsstaetteID: strAny(m["taetigkeitsstaette_id"]),
			})
		}
	}
	return in
}

func (a *App) toFahrt(r *http.Request, row sqlitedb.Fahrt) api.Fahrt {
	betrag := int64(0)
	if y := yearOf(row.Datum); y != 0 {
		if loaded, err := a.store.EffectiveYear(r.Context(), y); err == nil && loaded.Status == "aktiv" {
			rate := loaded.Inland.KmAnderes
			if row.Fahrzeugart == "kraftwagen" {
				rate = loaded.Inland.KmKraftwagen
			}
			km := row.Km
			if row.HinUndZurueck {
				km *= 2
			}
			betrag = km * rate
		}
	}
	return api.Fahrt{
		Id: row.ID, ReiseId: row.ReiseID, Datum: row.Datum, Start: row.StartOrt, Ziel: row.Ziel,
		Zweck: row.Zweck, Fahrzeugart: row.Fahrzeugart, Km: row.Km, HinUndZurueck: row.HinUndZurueck,
		BetragCent: betrag, Version: row.Version,
	}
}

func toReise(b store.ReiseBundle) api.Reise {
	legs := make([]api.Ortswechsel, 0, len(b.Legs))
	for _, leg := range b.Legs {
		item := api.Ortswechsel{
			Id: leg.ID, Reihenfolge: leg.Reihenfolge, Ankunft: localStamp(leg.Ankunft, leg.AnkunftZone),
			AnkunftZone: leg.AnkunftZone, Verkehrsmittel: leg.Verkehrsmittel, LandIso: leg.LandIso,
			Satzort: strPtr(leg.Satzort), Ort: strPtr(leg.Ort), TaetigkeitsstaetteId: leg.TaetigkeitsstaetteID,
			ZwischenlandungMitUebernachtung: &leg.ZwischenlandungMitUebernachtung,
		}
		if leg.Abfahrt != nil && leg.AbfahrtZone != nil {
			s := localStamp(*leg.Abfahrt, *leg.AbfahrtZone)
			item.Abfahrt = &s
			item.AbfahrtZone = leg.AbfahrtZone
		}
		legs = append(legs, item)
	}
	days := make([]api.Reisetag, 0, len(b.Tage))
	for _, day := range b.Tage {
		ex := day.VerpflegungAusgeschlossen
		item := api.Reisetag{
			Id: day.ID, Datum: day.Datum, Unterkunft: day.Unterkunft,
			FruehstueckGestellt: day.FruehstueckGestellt, MittagGestellt: day.MittagGestellt, AbendGestellt: day.AbendGestellt,
			LandManuell: day.LandManuell, SatzortManuell: day.SatzortManuell, Begruendung: day.LandBegruendung,
			ZuzahlungFruehstueck: &day.ZuzahlungFruehstueck, ZuzahlungMittag: &day.ZuzahlungMittag, ZuzahlungAbend: &day.ZuzahlungAbend,
			VerpflegungAusgeschlossen: &ex, AusschlussGrund: day.AusschlussGrund,
		}
		days = append(days, item)
	}
	return api.Reise{
		Id: b.Reise.ID, ArbeitgeberId: b.Reise.ArbeitgeberID, Anlass: b.Reise.Anlass, Projekt: b.Reise.Projekt,
		Beginn: localStamp(b.Reise.Beginn, b.Reise.BeginnZone), BeginnZone: b.Reise.BeginnZone,
		Ende: localStamp(b.Reise.Ende, b.Reise.EndeZone), EndeZone: b.Reise.EndeZone,
		Notiz: b.Reise.Notiz, VorlageId: b.Reise.VorlageID, Status: b.Reise.Status, Version: b.Reise.Version,
		Ortswechsel: legs, Reisetage: days,
	}
}

func toVorlage(row sqlitedb.Vorlage) api.Vorlage {
	daten := map[string]any{}
	_ = json.Unmarshal([]byte(row.Daten), &daten)
	return api.Vorlage{Id: row.ID, Name: row.Name, Art: row.Art, Daten: daten, Version: row.Version}
}

func reiseFromWrite(body api.ReiseWrite) (store.ReiseInput, bool) {
	in := store.ReiseInput{
		ArbeitgeberID: body.ArbeitgeberId, Anlass: body.Anlass, Projekt: deref(body.Projekt),
		BeginnLokal: body.Beginn, BeginnZone: body.BeginnZone, EndeLokal: body.Ende, EndeZone: body.EndeZone,
		Notiz: deref(body.Notiz), UnterkunftDefault: deref(body.Unterkunft),
	}
	for _, leg := range body.Ortswechsel {
		in.Ortswechsel = append(in.Ortswechsel, store.OrtswechselInput{
			AbfahrtLokal: deref(leg.Abfahrt), AbfahrtZone: deref(leg.AbfahrtZone),
			AnkunftLokal: leg.Ankunft, AnkunftZone: leg.AnkunftZone, Verkehrsmittel: leg.Verkehrsmittel,
			LandISO: leg.LandIso, Satzort: deref(leg.Satzort), Ort: deref(leg.Ort),
			TaetigkeitsstaetteID:            deref(leg.TaetigkeitsstaetteId),
			ZwischenlandungMitUebernachtung: leg.ZwischenlandungMitUebernachtung != nil && *leg.ZwischenlandungMitUebernachtung,
		})
	}
	if strings.TrimSpace(in.Anlass) == "" || in.ArbeitgeberID == "" || len(in.Ortswechsel) == 0 {
		return store.ReiseInput{}, false
	}
	return in, true
}

func fahrtFromWrite(body api.FahrtWrite) store.FahrtInput {
	return store.FahrtInput{
		Datum: body.Datum, Start: body.Start, Ziel: body.Ziel, Zweck: deref(body.Zweck),
		Fahrzeugart: body.Fahrzeugart, Km: body.Km, HinUndZurueck: body.HinUndZurueck,
	}
}

func writeStoreErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var code *store.CodeError
	if errors.As(err, &code) {
		status := http.StatusUnprocessableEntity
		switch code.Code {
		case "reise_gesperrt", "beleg_fest", "vorschuss_verrechnet", "reise_belegt", "vorschuss_belegt", "einreichung_laeuft", "uebergang":
			status = http.StatusConflict
		}
		writeProblem(w, status, code.Code, "Check the input", "")
		return true
	}
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "nicht_gefunden", "Not found", "")
		return true
	}
	if errors.Is(err, store.ErrConflict) {
		writeProblem(w, http.StatusPreconditionFailed, "version", "Version conflict", "")
		return true
	}
	var calc *berechnung.CalcError
	if errors.As(err, &calc) {
		writeProblem(w, http.StatusUnprocessableEntity, calc.Code, "Calculation refused", "")
		return true
	}
	writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")
	return true
}

func dayBound(s string, end bool) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, mustLoc("Europe/Berlin"))
	if err != nil {
		return nil
	}
	if end {
		t = t.Add(24*time.Hour - time.Second)
	}
	u := t.UTC()
	return &u
}

func localStamp(t time.Time, zone string) string {
	return t.In(mustLoc(zone)).Format("2006-01-02T15:04:05")
}

func mustLoc(zone string) *time.Location {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (a *App) ensureYear(r *http.Request, years map[int]*satz.Year, y int) error {
	if y == 0 {
		return nil
	}
	if _, ok := years[y]; ok {
		return nil
	}
	loaded, err := a.store.EffectiveYear(r.Context(), y)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	years[y] = loaded
	return nil
}

func yearOf(datum string) int {
	if len(datum) < 4 {
		return 0
	}
	n := 0
	for _, c := range datum[:4] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func val64(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}

func strAny(v any) string {
	s, _ := v.(string)
	return s
}

func int64Any(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func boolAny(v any) bool {
	b, _ := v.(bool)
	return b
}

func trimSec(s string) string {
	if len(s) >= 19 {
		return s[:19]
	}
	return s
}
