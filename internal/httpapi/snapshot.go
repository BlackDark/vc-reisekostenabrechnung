package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/api"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/berechnung"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/export"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

type claimReview struct {
	pruefung    api.AbrechnungPruefung
	blockerKeys []string
	warnKeys    []string
	snap        export.Snapshot
}

func (a *App) reviewAbrechnung(r *http.Request, n sqlitedb.Nutzer, id string) (claimReview, error) {
	b, err := a.store.GetAbrechnungBundle(r.Context(), n.ID, id)
	if err != nil {
		return claimReview{}, err
	}
	ag, err := a.store.GetArbeitgeber(r.Context(), n.ID, b.Row.ArbeitgeberID)
	if err != nil {
		return claimReview{}, err
	}
	var bundles []store.ReiseBundle
	for _, reiseID := range b.ReiseIDs {
		one, err := a.store.GetReiseBundle(r.Context(), n.ID, reiseID)
		if err != nil {
			return claimReview{}, err
		}
		// Locked trips stay out of the live day auction. The export uses the
		// amounts from this review, so treat them as still in the draft.
		one.Reise.Status = "in_entwurf"
		bundles = append(bundles, one)
	}
	result, _, _, err := a.calculate(r, bundles)
	if err != nil {
		return claimReview{}, err
	}
	advances, err := a.advancesOf(r, n.ID, b.VorschussIDs)
	if err != nil {
		return claimReview{}, err
	}
	var advanceCents []int64
	var advanceSum int64
	var vorschuesse []export.Vorschuss
	for _, v := range advances {
		advanceCents = append(advanceCents, v.Betrag)
		advanceSum += v.Betrag
		notiz := ""
		if v.Notiz != nil {
			notiz = *v.Notiz
		}
		vorschuesse = append(vorschuesse, export.Vorschuss{ID: v.ID, Datum: v.Datum, Cent: v.Betrag, Notiz: notiz})
	}
	var payout int64
	var erstattung int64
	var blocker, warns []api.PruefPunkt
	var blockerKeys, warnKeys []string
	var summen export.Summen
	reisen := make([]export.Reise, 0, len(bundles))
	seenBeleg := map[string]struct{}{}
	var belege []export.Beleg
	for i, bundle := range bundles {
		re := result[bundle.Reise.ID]
		erstattung += re.Summe
		summen.FahrtkostenCent += re.Fahrtkosten
		summen.VerpflegungCent += re.Verpflegung
		summen.UebernachtungCent += re.Uebernachtung
		summen.ReisenebenkostenCent += re.Reisenebenkosten
		summen.BewirtungCent += re.Bewirtung
		addPoints(&blocker, &blockerKeys, &warns, &warnKeys, bundle, re)
		trip, km, pausch, ueb := a.tripSnapshot(r, bundle, re, i+1)
		summen.DavonKilometerCent += km
		summen.DavonPauschalenCent += pausch
		summen.DavonUebernachtungPauschCent += ueb
		reisen = append(reisen, trip)
		for _, item := range bundle.Ausgaben {
			for _, link := range item.Belege {
				if _, ok := seenBeleg[link.ID]; ok {
					continue
				}
				seenBeleg[link.ID] = struct{}{}
				doc, err := a.belegSnapshot(r.Context(), n.ID, link.ID)
				if err != nil {
					return claimReview{}, err
				}
				belege = append(belege, doc)
			}
		}
	}
	_, payout = berechnung.Auszahlung(erstattung, advanceCents)
	years, err := a.rateSnapshot(r, bundles)
	if err != nil {
		return claimReview{}, err
	}
	events, err := a.store.ListProtokoll(r.Context(), "abrechnung", id, 200)
	if err != nil {
		return claimReview{}, err
	}
	protokoll := make([]export.Ereignis, 0, len(events))
	for _, ev := range events {
		grund := ""
		if ev.Grund != nil {
			grund = *ev.Grund
		}
		protokoll = append(protokoll, export.Ereignis{
			Zeitpunkt: ev.Zeitpunkt.UTC().Format(time.RFC3339), AkteurArt: ev.AkteurArt,
			Aktion: ev.Aktion, ObjektTyp: ev.ObjektTyp, ObjektID: ev.ObjektID, Grund: grund,
		})
	}
	nummer := ""
	if b.Row.Abrechnungsnummer != nil {
		nummer = *b.Row.Abrechnungsnummer
	}
	version := int(bounded(b.Row.AktuelleExportVersion))
	if version < 1 {
		version = 1
	}
	personal := ""
	if n.Personalnummer != nil {
		personal = *n.Personalnummer
	}
	appVersion := a.Version
	if appVersion == "" {
		appVersion = "dev"
	}
	snap := export.Snapshot{
		SchemaVersion: export.SchemaVersion, AppVersion: appVersion,
		ErzeugtAm: time.Now().UTC().Format(time.RFC3339), ExportSprache: b.Row.ExportSprache,
		Abrechnung: export.Kopf{
			Nummer: nummer, Version: version, ZeitraumArt: b.Row.ZeitraumArt, Von: b.Row.Von, Bis: b.Row.Bis,
			Titel: b.Row.Titel, Status: b.Row.Status, Summen: summen, ErstattungCent: erstattung,
			Vorschuesse: vorschuesse, VorschussCent: advanceSum, AuszahlungCent: payout,
		},
		Nutzer:             export.Person{Name: n.Anzeigename, Personalnummer: personal},
		Arbeitgeber:        export.Arbeitgeber{Name: ag.Name, Anschrift: ag.Anschrift},
		Satztabellen:       years,
		Reisen:             reisen,
		Belege:             belege,
		WarnungenQuittiert: nil,
		Protokoll:          protokoll,
	}
	if b.Row.EingereichtAm != nil {
		snap.Abrechnung.EingereichtAm = b.Row.EingereichtAm.UTC().Format(time.RFC3339)
	}
	return claimReview{
		pruefung: api.AbrechnungPruefung{
			Blocker: orPoints(blocker), Warnungen: orPoints(warns),
			ErstattungCent: erstattung, VorschussCent: advanceSum, AuszahlungCent: payout,
		},
		blockerKeys: blockerKeys, warnKeys: warnKeys, snap: snap,
	}, nil
}

func addPoints(blocker *[]api.PruefPunkt, blockerKeys *[]string, warns *[]api.PruefPunkt, warnKeys *[]string, bundle store.ReiseBundle, re berechnung.ReiseErgebnis) {
	seen := map[string]struct{}{}
	push := func(code, objekt string, block bool) {
		key := store.WarnKey(code, objekt)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		point := api.PruefPunkt{Code: code, ObjektId: objekt, ReiseId: bundle.Reise.ID, Anlass: strPtr(bundle.Reise.Anlass)}
		if block {
			*blocker = append(*blocker, point)
			*blockerKeys = append(*blockerKeys, code)
			return
		}
		*warns = append(*warns, point)
		*warnKeys = append(*warnKeys, key)
	}
	for _, code := range re.Blocker {
		push(code, bundle.Reise.ID, true)
	}
	for _, line := range re.Ausgaben {
		for _, code := range line.Warnungen {
			push(code, line.ID, submitBlocks(code))
		}
	}
	for _, day := range re.Tage {
		for _, code := range day.Warnungen {
			push(code, bundle.Reise.ID+"/"+day.Datum, submitBlocks(code))
		}
	}
	for _, code := range re.Warnungen {
		push(code, bundle.Reise.ID, submitBlocks(code))
	}
}

func submitBlocks(code string) bool {
	switch code {
	case "B01", "B02", "B03", "B04", "B05", "B06", "W06", "uebernachtung_doppelt":
		return true
	default:
		return strings.HasPrefix(code, "B")
	}
}

func (a *App) tripSnapshot(r *http.Request, bundle store.ReiseBundle, re berechnung.ReiseErgebnis, nr int) (export.Reise, int64, int64, int64) {
	days := map[string]sqlitedb.Reisetag{}
	for _, day := range bundle.Tage {
		days[day.Datum] = day
	}
	var km, pausch, ueb int64
	trip := export.Reise{
		Nr: nr, ID: bundle.Reise.ID, Anlass: bundle.Reise.Anlass, SummeCent: re.Summe,
		Beginn: localStamp(bundle.Reise.Beginn, bundle.Reise.BeginnZone), BeginnZone: bundle.Reise.BeginnZone,
		Ende: localStamp(bundle.Reise.Ende, bundle.Reise.EndeZone), EndeZone: bundle.Reise.EndeZone,
	}
	if bundle.Reise.Projekt != nil {
		trip.Projekt = *bundle.Reise.Projekt
	}
	for _, leg := range bundle.Legs {
		trip.Ortswechsel = append(trip.Ortswechsel, export.Ort{
			Ankunft: localStamp(leg.Ankunft, leg.AnkunftZone), Verkehrsmittel: leg.Verkehrsmittel,
			LandISO: leg.LandIso, Ort: leg.Ort,
		})
	}
	byID := map[string]berechnung.AusgabeErgebnis{}
	for _, line := range re.Ausgaben {
		byID[line.ID] = line
	}
	for _, item := range bundle.Ausgaben {
		line := byID[item.Row.ID]
		out := ausgabeSnapshot(item, line)
		if out.Kostenart == "bewirtung" {
			ab, nicht := bewirtSplit(line)
			out.BewirtungAbziehbarCent = ab
			out.BewirtungNichtCent = nicht
		}
		trip.Ausgaben = append(trip.Ausgaben, out)
	}
	for _, day := range re.Tage {
		stored := days[day.Datum]
		var frueh, mittag, abend int64
		for _, k := range day.Kuerzungen {
			switch k.Art {
			case "fruehstueck":
				frueh += k.Betrag
			case "mittag":
				mittag += k.Betrag
			case "abend":
				abend += k.Betrag
			}
		}
		var hints []string
		for _, h := range day.Hinweise {
			hints = append(hints, h.Code)
		}
		row := export.Tag{
			Datum: day.Datum, Tagesart: day.Tagesart, AbwesenheitMin: day.AbwesenheitMin,
			LandISO: day.LandISO, Satzort: day.Satzort, PauschaleCent: day.Pauschale,
			KuerzungFruehCent: frueh, KuerzungMittagCent: mittag, KuerzungAbendCent: abend,
			ZuzahlungenCent: stored.ZuzahlungFruehstueck + stored.ZuzahlungMittag + stored.ZuzahlungAbend,
			VerpflegungCent: day.Ergebnis, Unterkunft: stored.Unterkunft, UebernachtungCent: day.Uebernachtung,
			RegelIDs: day.RegelIDs, Hinweise: hints,
		}
		if row.Satz24hCent == 0 {
			row.Satz24hCent = a.rate24(r, yearOf(day.Datum), day.LandISO, day.Satzort)
		}
		pausch += day.Ergebnis
		if stored.Unterkunft == "pauschale" {
			ueb += day.Uebernachtung
		}
		trip.Reisetage = append(trip.Reisetage, row)
	}
	for _, f := range bundle.Fahrten {
		total := f.Km
		if f.HinUndZurueck {
			total *= 2
		}
		rate := a.kmRate(r, yearOf(f.Datum), f.Fahrzeugart)
		betrag := total * rate
		km += betrag
		zweck := ""
		if f.Zweck != nil {
			zweck = *f.Zweck
		}
		trip.Fahrten = append(trip.Fahrten, export.Fahrt{
			Datum: f.Datum, Start: f.StartOrt, Ziel: f.Ziel, Zweck: zweck, Fahrzeugart: f.Fahrzeugart,
			Km: f.Km, HinUndZurueck: f.HinUndZurueck, KmGesamt: total, SatzCent: rate, BetragCent: betrag,
		})
	}
	return trip, km, pausch, ueb
}

func ausgabeSnapshot(item store.AusgabeBundle, line berechnung.AusgabeErgebnis) export.Ausgabe {
	row := item.Row
	out := export.Ausgabe{
		ID: row.ID, Datum: row.Datum, Kostenart: row.Kostenart, Leistender: row.Leistender,
		Rechnungsart: row.Rechnungsart, RechnungAufArbeitgeber: row.RechnungAufArbeitgeber,
		Waehrung: row.Waehrung, BetragCent: row.Betrag, BetragEURCent: line.BetragEUR,
	}
	if row.Beschreibung != nil {
		out.Beschreibung = *row.Beschreibung
	}
	if row.Verkehrsmittel != nil {
		out.Verkehrsmittel = *row.Verkehrsmittel
	}
	if row.Kurs != nil {
		out.Kurs = *row.Kurs
	}
	if line.Kurs != "" {
		out.Kurs = line.Kurs
	}
	if row.KursQuelle != nil {
		out.KursQuelle = *row.KursQuelle
	}
	if row.KursDatum != nil {
		out.KursDatum = *row.KursDatum
	}
	if line.KursDatum != "" {
		out.KursDatum = line.KursDatum
	}
	if line.BetragEUR != 0 {
		out.BetragEURCent = line.BetragEUR
	} else {
		out.BetragEURCent = row.BetragEur
	}
	for i, sh := range item.Anteile {
		res := berechnung.SteuerErgebnis{}
		if i < len(line.Anteile) {
			res = line.Anteile[i]
		}
		out.Anteile = append(out.Anteile, export.Anteil{
			Nr: i + 1, Land: sh.Steuerland, Satz: sh.Satz,
			NettoCent: res.NettoEUR, UstCent: res.SteuerEUR, BruttoCent: res.BruttoEUR, Vorsteuer: res.Vorsteuer,
		})
	}
	if doc := bewirtDoc(deref(row.Bewirtung)); doc != nil {
		var names []string
		for _, p := range doc.Teilnehmer {
			if p.Name != "" {
				names = append(names, p.Name)
			}
		}
		out.Bewirtung = &export.Bewirt{
			Anlass: doc.Anlass, Ort: doc.Ort, Teilnehmer: strings.Join(names, ", "),
			Bewirtender: doc.Bewirtender, Bestaetigt: doc.BestaetigtAm,
		}
	}
	return out
}

func bewirtDoc(raw string) *store.BewirtungInput {
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return nil
	}
	var doc store.BewirtungInput
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil
	}
	return &doc
}

func bewirtSplit(line berechnung.AusgabeErgebnis) (int64, int64) {
	var basis int64
	for _, sh := range line.Anteile {
		if sh.Steuerland == "DE" && sh.Satz > 0 {
			basis += sh.NettoEUR
		} else {
			basis += sh.BruttoEUR
		}
	}
	if basis == 0 {
		basis = line.BetragEUR
	}
	ab := (basis*70 + 50) / 100
	return ab, basis - ab
}

func (a *App) belegSnapshot(ctx context.Context, nutzerID, belegID string) (export.Beleg, error) {
	row, err := a.store.GetBeleg(ctx, nutzerID, belegID)
	if err != nil {
		return export.Beleg{}, err
	}
	files, err := a.store.ListBelegdateien(ctx, belegID)
	if err != nil {
		return export.Beleg{}, err
	}
	nummer := belegID
	if row.Belegnummer != nil && *row.Belegnummer != "" {
		nummer = *row.Belegnummer
	}
	doc := export.Beleg{
		ID: row.ID, Nummer: nummer, Typ: row.Typ, Status: row.Status,
		SHA256: row.Sha256Original, PipelineVersion: row.PipelineVersion,
	}
	if row.BestaetigtAm != nil {
		doc.BestaetigtAm = row.BestaetigtAm.UTC().Format(time.RFC3339)
	}
	for _, f := range files {
		ext := "bin"
		switch {
		case strings.Contains(f.Mime, "jpeg"):
			ext = "jpg"
		case strings.Contains(f.Mime, "pdf"):
			ext = "pdf"
		case strings.Contains(f.Mime, "xml"):
			ext = "xml"
		case strings.Contains(f.Mime, "png"):
			ext = "png"
		}
		doc.Dateien = append(doc.Dateien, export.Datei{
			Variante: f.Variante, Seite: int(bounded(f.Seite)), MIME: f.Mime, SHA256: f.Sha256,
			Name: nummer + "_" + f.Variante + "_s" + itoa(f.Seite) + "." + ext,
		})
	}
	texts, err := a.store.ListBelegtexte(ctx, nutzerID, belegID)
	if err != nil {
		return export.Beleg{}, err
	}
	var best *sqlitedb.Belegtext
	for i := range texts {
		if texts[i].BestaetigtAm == nil {
			continue
		}
		if best == nil || texts[i].Version > best.Version {
			best = &texts[i]
		}
	}
	if best != nil {
		doc.Text = best.Volltext
	}
	return doc, nil
}

func (a *App) rateSnapshot(r *http.Request, bundles []store.ReiseBundle) ([]export.SatzJahr, error) {
	years := map[int]*satz.Year{}
	for _, b := range bundles {
		for _, day := range b.Tage {
			if err := a.ensureYear(r, years, yearOf(day.Datum)); err != nil {
				return nil, err
			}
		}
		for _, f := range b.Fahrten {
			if err := a.ensureYear(r, years, yearOf(f.Datum)); err != nil {
				return nil, err
			}
		}
	}
	var out []export.SatzJahr
	for y, year := range years {
		if year == nil {
			continue
		}
		in := year.Inland
		row := export.SatzJahr{
			Jahr: y, Quelle: in.Quelle,
			Werte: map[string]int64{
				"vma_24h_cent": in.VMA24h, "vma_8h_cent": in.VMA8h, "uebernachtung_cent": in.Uebernachtung,
				"km_kraftwagen_cent": in.KmKraftwagen, "km_anderes_cent": in.KmAnderes,
			},
		}
		out = append(out, row)
	}
	if out == nil {
		out = []export.SatzJahr{}
	}
	return out, nil
}

func (a *App) rate24(r *http.Request, year int, land, ort string) int64 {
	loaded, err := a.store.EffectiveYear(r.Context(), year)
	if err != nil || loaded == nil {
		return 0
	}
	if land == "" {
		land = "DE"
	}
	hit, err := loaded.Satz(land, ort)
	if err != nil {
		return loaded.Inland.VMA24h
	}
	return hit.VMA24h
}

func (a *App) kmRate(r *http.Request, year int, art string) int64 {
	loaded, err := a.store.EffectiveYear(r.Context(), year)
	if err != nil || loaded == nil {
		return 0
	}
	if art == "kraftwagen" {
		return loaded.Inland.KmKraftwagen
	}
	return loaded.Inland.KmAnderes
}

func (a *App) advancesOf(r *http.Request, nutzerID string, ids []string) ([]sqlitedb.Vorschuss, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := a.store.ListVorschuesse(r.Context(), nutzerID)
	if err != nil {
		return nil, err
	}
	want := map[string]struct{}{}
	for _, id := range ids {
		want[id] = struct{}{}
	}
	var out []sqlitedb.Vorschuss
	for _, row := range rows {
		if _, ok := want[row.ID]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

func snapshotJSON(s export.Snapshot) ([]byte, error) {
	if s.Satztabellen == nil {
		s.Satztabellen = []export.SatzJahr{}
	}
	if s.Reisen == nil {
		s.Reisen = []export.Reise{}
	}
	if s.Belege == nil {
		s.Belege = []export.Beleg{}
	}
	if s.WarnungenQuittiert == nil {
		s.WarnungenQuittiert = []export.Quittung{}
	}
	if s.Protokoll == nil {
		s.Protokoll = []export.Ereignis{}
	}
	if s.Abrechnung.Vorschuesse == nil {
		s.Abrechnung.Vorschuesse = []export.Vorschuss{}
	}
	return json.Marshal(s)
}

func orPoints(items []api.PruefPunkt) []api.PruefPunkt {
	if items == nil {
		return []api.PruefPunkt{}
	}
	return items
}

func bounded(n int64) int64 {
	if n < 0 || n > 1_000_000 {
		return 0
	}
	return n
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
