package berechnung

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"
)

var errNoYear = errors.New("B05")

// CalcError is a calculation refusal such as an unsupported Konstellation.
type CalcError struct{ Code string }

func (e *CalcError) Error() string { return e.Code }

const eightHours = 480

// Berechne runs SPEC 4 for one Nutzer. It does not touch the database.
func Berechne(in Eingabe) (Ergebnis, error) {
	for _, r := range in.Reisen {
		if r.Konstellation != "" && r.Konstellation != "arbeitgebererstattung" {
			return Ergebnis{}, &CalcError{Code: "konstellation_nicht_unterstuetzt"}
		}
	}
	prepared := make([]prep, len(in.Reisen))
	for i := range in.Reisen {
		p, err := prepareReise(&in, i)
		if err != nil {
			return Ergebnis{}, err
		}
		prepared[i] = p
	}
	assign(prepared)
	var out Ergebnis
	for i := range prepared {
		re := rollup(&in, &prepared[i])
		out.Reisen = append(out.Reisen, re)
		if len(re.Blocker) == 0 {
			out.Erstattung += re.Summe
		}
	}
	out.VorschussVerrechnet, out.Auszahlung = Auszahlung(out.Erstattung, in.Vorschuesse)
	return out, nil
}

type prep struct {
	idx       int
	reise     *Reise
	start     time.Time
	end       time.Time
	days      []dayCalc
	blocker   []string
	ausgaben  []AusgabeErgebnis
	fahrt     int64
	ausgabe   map[string]int64
	bewirt    bewirtSum
	vorsteuer int64
	warnings  []string
}

type bewirtSum struct {
	erstattung int64
	b          int64
	abziehbar  int64
	nicht      int64
}

type dayCalc struct {
	datum     civil
	zone      string
	art       string
	land      ort
	hit       satz.Hit
	sleep     ort
	sleepHit  satz.Hit
	absence   int
	base      int64
	pctF      int64
	pctH      int64
	sachFrueh int64
	sachHaupt int64
	vmaRegel  string
	meals     mealFlags
	excluded  bool
	ownP      int64
	pauschale int64
	kuerz     []Kuerzung
	ergebnis  int64
	ueb       int64
	hinweise  []Hinweis
	warnungen []string
	regeln    []string
	gotP      bool
}

type mealFlags struct {
	frueh, mittag, abend    bool
	zFrueh, zMittag, zAbend int64
}

func tagInput(r Reise, datum string) TagEingabe {
	if r.Tage == nil {
		return TagEingabe{}
	}
	return r.Tage[datum]
}

func prepareReise(in *Eingabe, idx int) (prep, error) {
	r := &in.Reisen[idx]
	p := prep{idx: idx, reise: r, ausgabe: map[string]int64{}}
	start, err := r.Beginn.instant()
	if err != nil {
		return p, err
	}
	end, err := r.Ende.instant()
	if err != nil {
		return p, err
	}
	p.start, p.end = start, end
	if !end.After(start) {
		p.blocker = append(p.blocker, "B06")
		return p, nil
	}
	from, err := r.Beginn.civil()
	if err != nil {
		return p, err
	}
	to, err := r.Ende.civil()
	if err != nil {
		return p, err
	}
	dates := eachDate(from, to)
	if len(dates) == 0 {
		p.blocker = append(p.blocker, "B06")
		return p, nil
	}
	p.days = make([]dayCalc, len(dates))
	overnight := false
	for i, d := range dates {
		inTag := tagInput(*r, d.String())
		u := inTag.Unterkunft
		if u == "" {
			u = "keine"
		}
		if i == len(dates)-1 {
			u = "keine"
		} else if u == "beleg" || u == "pauschale" || u == "gestellt" || u == "verkehrsmittel" {
			overnight = true
		}
		p.days[i] = dayCalc{
			datum: d,
			zone:  dayZone(*r, d),
			meals: mealFlags{
				frueh: inTag.Fruehstueck, mittag: inTag.Mittag, abend: inTag.Abend,
				zFrueh: inTag.ZuzahlungFruehstueck, zMittag: inTag.ZuzahlungMittag, zAbend: inTag.ZuzahlungAbend,
			},
			excluded: inTag.VerpflegungAusgeschlossen,
		}
		_ = u
	}
	n := len(dates)
	switch {
	case n == 1:
		p.days[0].art = "eintaegig"
	case n >= 2 && overnight:
		p.days[0].art = "anreisetag"
		p.days[n-1].art = "abreisetag"
		for i := 1; i < n-1; i++ {
			p.days[i].art = "zwischentag"
		}
	case n == 2:
		p.days[0].art = "ueber_nacht"
		p.days[1].art = "ueber_nacht"
	default:
		p.blocker = append(p.blocker, "B06")
		return p, nil
	}
	for i := range p.days {
		dc := &p.days[i]
		year, berr := in.year(dc.datum.Y)
		if berr != nil {
			p.blocker = appendUnique(p.blocker, "B05")
			continue
		}
		mins, err := absenceMinutes(dc.datum, dc.zone, start, end)
		if err != nil {
			return p, err
		}
		dc.absence = mins
		dc.land = massgeblichesLand(*r, dc.datum, dc.art, year)
		hit, err := lookup(year, dc.land)
		if err != nil {
			p.blocker = appendUnique(p.blocker, "B05")
			continue
		}
		dc.hit = hit
		dc.pctF = year.Inland.KuerzungFruehstueckPct
		dc.pctH = year.Inland.KuerzungHauptmahlzeitPct
		dc.sachFrueh = year.Inland.SachbezugFruehstueck
		dc.sachHaupt = year.Inland.SachbezugHauptmahlzeit
		dc.sleep = ortRegel4(*r, dc.datum)
		if dc.sleep.Land == "" {
			dc.sleep.Land = "DE"
		}
		shit, err := lookup(year, dc.sleep)
		if err != nil {
			p.blocker = appendUnique(p.blocker, "B05")
			continue
		}
		dc.sleepHit = shit
		dc.regeln = append(dc.regeln, dc.land.Regel)
	}
	if hasBlocker(p.blocker, "B05") {
		return p, nil
	}
	applyBase(&p)
	applyOvernight(in, &p)
	applyFahrten(in, &p)
	applyAusgaben(in, &p)
	return p, nil
}

func (in *Eingabe) year(y int) (*satz.Year, error) {
	yr := in.Jahre[y]
	if yr == nil || yr.Status != "aktiv" {
		return nil, errNoYear
	}
	return yr, nil
}

func dayZone(r Reise, day civil) string {
	zone := r.Beginn.Zone
	for _, leg := range r.Ortswechsel {
		arrived, err := leg.Ankunft.civil()
		if err != nil || leg.Ankunft.Zone == "" {
			continue
		}
		if !arrived.after(day) {
			zone = leg.Ankunft.Zone
		}
	}
	if end, err := r.Ende.civil(); err == nil && end.equal(day) && r.Ende.Zone != "" {
		return r.Ende.Zone
	}
	return zone
}

func applyBase(p *prep) {
	for i := range p.days {
		dc := &p.days[i]
		switch dc.art {
		case "eintaegig":
			if dc.absence > eightHours {
				dc.base = dc.hit.VMA8h
				dc.vmaRegel = "VMA-EINTAG"
			}
		case "anreisetag":
			dc.base = dc.hit.VMA8h
			dc.vmaRegel = "VMA-ANREISE"
		case "abreisetag":
			dc.base = dc.hit.VMA8h
			dc.vmaRegel = "VMA-ABREISE"
		case "zwischentag":
			dc.base = dc.hit.VMA24h
			dc.vmaRegel = "VMA-ZWISCHEN"
		}
		if dc.vmaRegel != "" {
			dc.regeln = append(dc.regeln, dc.vmaRegel)
		}
	}
	if len(p.days) == 2 && p.days[0].art == "ueber_nacht" {
		sum := p.days[0].absence + p.days[1].absence
		win := 1
		if p.days[0].absence > p.days[1].absence {
			win = 0
		}
		if sum > eightHours {
			p.days[win].base = p.days[win].hit.VMA8h
			p.days[win].vmaRegel = "VMA-UEBER-NACHT"
			p.days[win].regeln = append(p.days[win].regeln, "VMA-UEBER-NACHT")
		}
	}
	for i := range p.days {
		p.days[i].ownP = p.days[i].base
	}
}

func applyOvernight(in *Eingabe, p *prep) {
	for i := range p.days {
		if i == len(p.days)-1 {
			continue
		}
		dc := &p.days[i]
		u := unterkunft(*p.reise, dc.datum.String())
		switch u {
		case "beleg":
			sum := covered(*p.reise, dc.datum.String())
			if sum == 0 {
				dc.warnungen = append(dc.warnungen, "W01")
			}
			dc.ueb = sum
			dc.regeln = append(dc.regeln, "UEB-BELEG")
		case "pauschale":
			if covered(*p.reise, dc.datum.String()) > 0 {
				p.blocker = appendUnique(p.blocker, "uebernachtung_doppelt")
				p.blocker = appendUnique(p.blocker, "B06")
			}
			dc.ueb = p.days[i].sleepHit.Uebernachtung
			dc.regeln = append(dc.regeln, "UEB-PAUSCHALE")
		}
	}
	_ = in
}

func unterkunft(r Reise, datum string) string {
	u := tagInput(r, datum).Unterkunft
	if u == "" {
		return "keine"
	}
	return u
}

func covered(r Reise, night string) int64 {
	var sum int64
	for _, a := range r.Ausgaben {
		if a.Kostenart != "uebernachtung" {
			continue
		}
		for _, n := range a.Naechte {
			if n == night {
				sum += eurOf(a)
			}
		}
	}
	return sum
}

func eurOf(a Ausgabe) int64 {
	if a.Waehrung == "" || a.Waehrung == "EUR" || a.KursQuelle == "belastung" {
		if a.BetragEUR > 0 {
			return a.BetragEUR
		}
		return a.Betrag
	}
	return a.BetragEUR
}

type groupKey struct {
	scope string
	datum string
}

type ref struct{ pi, di int }

func assign(all []prep) {
	groups := map[groupKey][]ref{}
	var keys []groupKey
	for pi := range all {
		if len(all[pi].blocker) > 0 {
			continue
		}
		scope := all[pi].reise.ArbeitgeberID
		if VerpflegungScope() == "nutzer" {
			scope = ""
		}
		for di := range all[pi].days {
			k := groupKey{scope: scope, datum: all[pi].days[di].datum.String()}
			if _, ok := groups[k]; !ok {
				keys = append(keys, k)
			}
			groups[k] = append(groups[k], ref{pi, di})
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].datum != keys[j].datum {
			return keys[i].datum < keys[j].datum
		}
		return keys[i].scope < keys[j].scope
	})
	for _, k := range keys {
		refs := groups[k]
		var maxCand int64
		var win dayCalc
		have := false
		var combined int
		foreign := -1
		for i, rf := range refs {
			dc := &all[rf.pi].days[rf.di]
			if !have || dc.base > maxCand {
				maxCand = dc.base
				win = *dc
				have = true
			}
			if dc.art == "eintaegig" || dc.art == "ueber_nacht" {
				combined += dc.absence
			}
			if !dc.land.inland() && (foreign < 0 || later(*all[rf.pi].reise, *all[refs[foreign].pi].reise)) {
				foreign = i
			}
		}
		if combined > eightHours && len(refs) > 0 {
			src := refs[0]
			if foreign >= 0 {
				src = refs[foreign]
			}
			rate := all[src.pi].days[src.di].hit.VMA8h
			if rate > maxCand {
				maxCand = rate
				win = all[src.pi].days[src.di]
				have = true
			}
		}
		meals := unionMeals(all, refs)
		s24 := int64(0)
		pctF, pctH := int64(2000), int64(4000)
		if have {
			s24 = win.hit.VMA24h
			if win.pctF > 0 {
				pctF = win.pctF
			}
			if win.pctH > 0 {
				pctH = win.pctH
			}
		}
		kF := kuerzAmount(meals.frueh, s24, pctF, meals.zFrueh)
		kM := kuerzAmount(meals.mittag, s24, pctH, meals.zMittag)
		kA := kuerzAmount(meals.abend, s24, pctH, meals.zAbend)
		var lines []Kuerzung
		if meals.frueh && s24 > 0 {
			lines = append(lines, Kuerzung{Art: "fruehstueck", Betrag: kF, Regel: "KUERZ-FRUEH"})
		}
		if meals.mittag && s24 > 0 {
			lines = append(lines, Kuerzung{Art: "mittag", Betrag: kM, Regel: "KUERZ-MITTAG"})
		}
		if meals.abend && s24 > 0 {
			lines = append(lines, Kuerzung{Art: "abend", Betrag: kA, Regel: "KUERZ-ABEND"})
		}
		result := maxCand - kF - kM - kA
		if result < 0 {
			result = 0
		}
		var granted int64
		for _, rf := range refs {
			if all[rf.pi].reise.Status == "gesperrt" {
				granted += all[rf.pi].reise.Snapshot[k.datum]
			}
		}
		rest := result - granted
		if rest < 0 {
			rest = 0
			for _, rf := range refs {
				all[rf.pi].days[rf.di].warnungen = append(all[rf.pi].days[rf.di].warnungen, "W12")
			}
		}
		open := openRefs(all, refs)
		winner := -1
		if len(open) > 0 && maxCand > 0 {
			winner = 0
			for i := range open {
				if later(*all[open[i].pi].reise, *all[open[winner].pi].reise) {
					winner = i
				}
			}
		}
		for _, rf := range refs {
			dc := &all[rf.pi].days[rf.di]
			if all[rf.pi].reise.Status == "gesperrt" {
				dc.ergebnis = all[rf.pi].reise.Snapshot[k.datum]
				dc.pauschale = dc.ergebnis
				dc.gotP = dc.ergebnis > 0
				continue
			}
			owned := winner >= 0 && rf == open[winner]
			if owned {
				dc.pauschale = maxCand
				dc.kuerz = lines
				dc.ergebnis = rest
				dc.gotP = true
				for _, line := range lines {
					if line.Betrag > 0 {
						dc.regeln = append(dc.regeln, line.Regel)
					}
				}
				if dc.excluded {
					dc.ergebnis = 0
					dc.regeln = append(dc.regeln, "VMA-AUSGESCHLOSSEN")
				}
				continue
			}
			dc.ergebnis = 0
			if len(refs) > 1 && (winner >= 0 || granted > 0) {
				other := *all[refs[0].pi].reise
				if winner >= 0 {
					other = *all[open[winner].pi].reise
				}
				if other.ID != all[rf.pi].reise.ID {
					dc.hinweise = append(dc.hinweise, Hinweis{
						Code: "H-TAG-VERRECHNET", ReiseID: other.ID, Anlass: other.Anlass, Datum: k.datum,
					})
				}
			}
		}
		if maxCand == 0 {
			for _, rf := range refs {
				dc := &all[rf.pi].days[rf.di]
				if dc.meals.frueh || dc.meals.mittag || dc.meals.abend {
					betrag := dc.sachHaupt
					if dc.meals.frueh && !dc.meals.mittag && !dc.meals.abend {
						betrag = dc.sachFrueh
					}
					dc.hinweise = append(dc.hinweise, Hinweis{Code: "H-SACHBEZUG", Betrag: betrag, Datum: dc.datum.String()})
				}
			}
		}
	}
}

func unionMeals(all []prep, refs []ref) mealFlags {
	var meals mealFlags
	for _, rf := range refs {
		m := all[rf.pi].days[rf.di].meals
		if m.frueh {
			meals.frueh = true
			if m.zFrueh > meals.zFrueh {
				meals.zFrueh = m.zFrueh
			}
		}
		if m.mittag {
			meals.mittag = true
			if m.zMittag > meals.zMittag {
				meals.zMittag = m.zMittag
			}
		}
		if m.abend {
			meals.abend = true
			if m.zAbend > meals.zAbend {
				meals.zAbend = m.zAbend
			}
		}
	}
	return meals
}

func kuerzAmount(gestellt bool, s, pct, zuzahlung int64) int64 {
	if !gestellt || s <= 0 {
		return 0
	}
	k := roundDiv(s*pct, 10000) - zuzahlung
	if k < 0 {
		return 0
	}
	return k
}

func later(a, b Reise) bool {
	ae, ea := a.Ende.instant()
	be, eb := b.Ende.instant()
	if ea != nil || eb != nil {
		return a.ID > b.ID
	}
	if ae.Equal(be) {
		return a.ID > b.ID
	}
	return ae.After(be)
}

func openRefs(all []prep, refs []ref) []ref {
	var out []ref
	for _, rf := range refs {
		st := all[rf.pi].reise.Status
		if st == "" || st == "offen" || st == "in_entwurf" {
			out = append(out, rf)
		}
	}
	return out
}

func applyFahrten(in *Eingabe, p *prep) {
	for _, f := range p.reise.Fahrten {
		d, err := parseCivil(f.Datum)
		if err != nil {
			p.blocker = appendUnique(p.blocker, "B05")
			continue
		}
		year, err := in.year(d.Y)
		if err != nil {
			p.blocker = appendUnique(p.blocker, "B05")
			continue
		}
		rate := year.Inland.KmAnderes
		if f.Fahrzeugart == "kraftwagen" {
			rate = year.Inland.KmKraftwagen
		}
		km := f.Km
		if f.HinUndZurueck {
			km *= 2
		}
		if km < 0 {
			km = 0
		}
		p.fahrt += km * rate
	}
}

func applyAusgaben(in *Eingabe, p *prep) {
	for _, a := range p.reise.Ausgaben {
		ae, warns, block := convertAusgabe(in, *p.reise, a)
		p.ausgaben = append(p.ausgaben, ae)
		p.warnings = append(p.warnings, warns...)
		for _, b := range block {
			p.blocker = appendUnique(p.blocker, b)
		}
		if len(block) > 0 {
			continue
		}
		p.ausgabe[a.Kostenart] += ae.BetragEUR
		for _, sh := range ae.Anteile {
			if sh.Vorsteuer {
				p.vorsteuer += sh.SteuerEUR
			}
		}
		if a.Kostenart == "bewirtung" {
			addBewirt(in, &p.bewirt, ae)
		}
	}
	if p.fahrt > 0 {
		for _, a := range p.reise.Ausgaben {
			if a.Kostenart == "fahrtkosten" && a.Verkehrsmittel == "dienstwagen_kraftstoff" {
				p.warnings = append(p.warnings, "W09")
			}
		}
		for _, f := range p.reise.Fahrten {
			if f.Fahrzeugart == "kraftwagen" {
				for _, a := range p.reise.Ausgaben {
					if a.Kostenart == "fahrtkosten" && a.Verkehrsmittel == "dienstwagen_kraftstoff" {
						p.warnings = appendUnique(p.warnings, "W09")
					}
				}
			}
		}
	}
}

func convertAusgabe(in *Eingabe, r Reise, a Ausgabe) (AusgabeErgebnis, []string, []string) {
	ae := AusgabeErgebnis{ID: a.ID, Kostenart: a.Kostenart}
	var warns, block []string
	eur, kurs, kursDatum, b := moneyEUR(in, a)
	if b != "" {
		return ae, nil, []string{b}
	}
	ae.BetragEUR = eur
	ae.Kurs = kurs
	ae.KursDatum = kursDatum
	a.BetragEUR = eur
	d, err := parseCivil(a.Datum)
	if err != nil {
		return ae, nil, []string{"B05"}
	}
	year, err := in.year(d.Y)
	if err != nil {
		return ae, nil, []string{"B05"}
	}
	anteile := a.Anteile
	if len(anteile) == 0 && eur > 0 {
		anteile = []SteuerInput{{Satz: 0, Steuerland: "DE", Brutto: a.Betrag}}
	}
	var shares []SteuerErgebnis
	var sumBrutto int64
	for _, sh := range anteile {
		netto, steuer := int64(0), int64(0)
		if sh.Netto != nil && sh.Steuer != nil {
			netto, steuer = *sh.Netto, *sh.Steuer
			expect := roundDiv(sh.Brutto*sh.Satz, 10000+sh.Satz)
			diff := steuer - expect
			if diff < 0 {
				diff = -diff
			}
			if diff > 1 {
				warns = append(warns, "W13")
			}
		} else {
			steuer = roundDiv(sh.Brutto*sh.Satz, 10000+sh.Satz)
			netto = sh.Brutto - steuer
		}
		shares = append(shares, SteuerErgebnis{
			Satz: sh.Satz, Steuerland: sh.Steuerland, Netto: netto, Steuer: steuer, Brutto: sh.Brutto,
		})
		sumBrutto += sh.Brutto
	}
	if sumBrutto > 0 && sumBrutto != a.Betrag && a.Betrag > 0 {
		block = append(block, "B02")
	}
	// Convert shares to EUR. Rounding remainder lands on the largest share.
	if a.Betrag > 0 && (a.Waehrung != "" && a.Waehrung != "EUR") {
		var allocated int64
		largest := 0
		for i := range shares {
			shares[i].BruttoEUR = roundDiv(shares[i].Brutto*eur, a.Betrag)
			shares[i].SteuerEUR = roundDiv(shares[i].Steuer*eur, a.Betrag)
			allocated += shares[i].BruttoEUR
			if shares[i].Brutto > shares[largest].Brutto {
				largest = i
			}
		}
		shares[largest].BruttoEUR += eur - allocated
		for i := range shares {
			shares[i].NettoEUR = shares[i].BruttoEUR - shares[i].SteuerEUR
		}
	} else {
		for i := range shares {
			shares[i].BruttoEUR = shares[i].Brutto
			shares[i].SteuerEUR = shares[i].Steuer
			shares[i].NettoEUR = shares[i].Netto
		}
	}
	klein := year.Inland.Kleinbetrag
	for i := range shares {
		sh := &shares[i]
		sh.Vorsteuer = sh.Steuerland == "DE" && a.Rechnungsart != "eigenbeleg" &&
			(a.RechnungAufArbeitgeber || eur <= klein || a.Rechnungsart == "fahrausweis")
		if sh.Steuerland != "" && sh.Steuerland != "DE" && sh.Steuer > 0 {
			warns = appendUnique(warns, "W10")
		}
	}
	ae.Anteile = shares
	if a.Kostenart == "verpflegung" && eur > year.Inland.UeblicheMahlzeit {
		warns = append(warns, "W08")
	}
	if a.Rechnungsart != "fahrausweis" && !a.RechnungAufArbeitgeber && eur > klein {
		warns = append(warns, "W02")
	}
	if outsideTrip(r, d) {
		warns = append(warns, "W11")
	}
	if a.PruefeBeleg {
		if a.BelegAnzahl == 0 && !a.Eigenbeleg && a.Rechnungsart != "eigenbeleg" {
			warns = append(warns, "W01")
		}
		if a.BelegOffen {
			warns = append(warns, "B01")
		}
		if a.Eigenbeleg || a.Rechnungsart == "eigenbeleg" {
			warns = appendUnique(warns, "W05")
		}
	}
	if a.Kostenart == "bewirtung" && a.Bewirtung != nil {
		if bewirtungMissing(a, eur, klein) {
			warns = append(warns, "B03")
		}
		if !a.TSE {
			warns = append(warns, "W06")
		}
	}
	if a.Empfaenger != "" && r.ArbeitgeberName != "" && !strings.EqualFold(strings.TrimSpace(a.Empfaenger), strings.TrimSpace(r.ArbeitgeberName)) {
		warns = appendUnique(warns, "W02")
	}
	if a.Waehrung != "" && a.Waehrung != "EUR" && a.Monatskurs != "" && deTax(shares) {
		warns = appendUnique(warns, "H-UST-KURS")
		ae.Monatskurs = a.Monatskurs
	}
	ae.Warnungen = warns
	return ae, warns, block
}

func deTax(shares []SteuerErgebnis) bool {
	for _, sh := range shares {
		if sh.Steuerland == "DE" && sh.Satz > 0 {
			return true
		}
	}
	return false
}

func bewirtungMissing(a Ausgabe, eur, klein int64) bool {
	b := a.Bewirtung
	if b == nil {
		return false
	}
	if strings.TrimSpace(b.Anlass) == "" || b.Teilnehmer < 1 || strings.TrimSpace(b.Ort) == "" || !b.Bestaetigt {
		return true
	}
	if eur > klein && strings.TrimSpace(b.Bewirtender) == "" {
		return true
	}
	return false
}

func moneyEUR(in *Eingabe, a Ausgabe) (eur int64, kurs, kursDatum, block string) {
	if a.Waehrung == "" || a.Waehrung == "EUR" {
		if a.BetragEUR > 0 {
			return a.BetragEUR, "", "", ""
		}
		return a.Betrag, "", "", ""
	}
	if a.KursQuelle == "belastung" {
		if a.BetragEUR <= 0 {
			return 0, "", "", "B04"
		}
		k := formatScaled(roundDiv(a.Betrag*1_000_000, a.BetragEUR), 6)
		return a.BetragEUR, k, a.KursDatum, ""
	}
	rate, used, ok := findKurs(in.Kurse, a)
	if !ok {
		return 0, "", "", "B04"
	}
	num, den, err := parseDecimal(rate)
	if err != nil || num <= 0 {
		return 0, "", "", "B04"
	}
	return roundDiv(a.Betrag*den, num), rate, used, ""
}

func findKurs(rates []Kurs, a Ausgabe) (string, string, bool) {
	if a.Kurs != "" && a.KursQuelle == "manuell" {
		d := a.KursDatum
		if d == "" {
			d = a.Datum
		}
		return a.Kurs, d, true
	}
	want, err := parseCivil(a.Datum)
	if err != nil {
		return "", "", false
	}
	var best Kurs
	bestDays := 8
	for _, k := range rates {
		if k.Waehrung != a.Waehrung {
			continue
		}
		kd, err := parseCivil(k.Datum)
		if err != nil || kd.after(want) {
			continue
		}
		gap := kd.daysUntil(want)
		if gap <= 7 && gap < bestDays {
			best = k
			bestDays = gap
		}
	}
	if best.Kurs == "" {
		return "", "", false
	}
	return best.Kurs, best.Datum, true
}

func outsideTrip(r Reise, d civil) bool {
	from, err1 := r.Beginn.civil()
	to, err2 := r.Ende.civil()
	if err1 != nil || err2 != nil {
		return false
	}
	return d.before(from.add(-30)) || d.after(to.add(30))
}

func addBewirt(in *Eingabe, b *bewirtSum, ae AusgabeErgebnis) {
	b.erstattung += ae.BetragEUR
	var basis int64
	for _, sh := range ae.Anteile {
		if sh.Steuerland == "DE" && sh.Satz > 0 {
			basis += sh.NettoEUR
		} else {
			basis += sh.BruttoEUR
		}
	}
	b.b += basis
	pct := int64(7000)
	if len(ae.Anteile) > 0 {
		_ = in
	}
	ab := roundDiv(basis*pct, 10000)
	b.abziehbar += ab
	b.nicht += basis - ab
}

func rollup(in *Eingabe, p *prep) ReiseErgebnis {
	re := ReiseErgebnis{
		ID: p.reise.ID, Blocker: append([]string{}, p.blocker...),
		Warnungen: append([]string{}, p.warnings...), Ausgaben: p.ausgaben,
		Fahrtkosten: p.fahrt, Bewirtung: p.bewirt.erstattung,
		BewirtungB: p.bewirt.b, BewirtungAbziehbar: p.bewirt.abziehbar,
		BewirtungNichtAbziehbar: p.bewirt.nicht, Vorsteuer: p.vorsteuer,
	}
	if len(p.blocker) > 0 {
		re.Warnungen = uniqueStrings(re.Warnungen)
		return re
	}
	var vma, pauschale int64
	for i, dc := range p.days {
		te := TagErgebnis{
			ReiseID: p.reise.ID, Datum: dc.datum.String(), Tagesart: dc.art,
			LandISO: dc.land.Land, Satzort: dc.land.Satzort, LandRegel: dc.land.Regel,
			AbwesenheitMin: dc.absence, Pauschale: dc.pauschale, Kuerzungen: dc.kuerz,
			Ergebnis: dc.ergebnis, Uebernachtung: dc.ueb, RegelIDs: uniqueStrings(dc.regeln),
			Hinweise: dc.hinweise, Warnungen: uniqueStrings(dc.warnungen),
		}
		for _, h := range dc.hinweise {
			if h.Code == "H-SACHBEZUG" {
				te.Warnungen = append(te.Warnungen, "W07")
			}
		}
		if te.LandISO == "" {
			te.LandISO = "DE"
		}
		re.Tage = append(re.Tage, te)
		vma += dc.ergebnis
		if i < len(p.days)-1 && unterkunft(*p.reise, dc.datum.String()) == "pauschale" {
			pauschale += dc.ueb
		}
		re.Warnungen = append(re.Warnungen, te.Warnungen...)
	}
	re.Verpflegung = vma + p.ausgabe["verpflegung"]
	re.Uebernachtung = pauschale + p.ausgabe["uebernachtung"]
	re.Fahrtkosten += p.ausgabe["fahrtkosten"]
	re.Reisenebenkosten = p.ausgabe["reisenebenkosten"]
	re.Bewirtung = p.bewirt.erstattung
	if re.Bewirtung == 0 {
		re.Bewirtung = p.ausgabe["bewirtung"]
	}
	re.Summe = re.Fahrtkosten + re.Verpflegung + re.Uebernachtung + re.Reisenebenkosten + re.Bewirtung
	if re.Summe == 0 {
		re.Warnungen = append(re.Warnungen, "W14")
	}
	re.Warnungen = uniqueStrings(re.Warnungen)
	_ = in
	return re
}

func hasBlocker(b []string, code string) bool {
	for _, c := range b {
		if c == code {
			return true
		}
	}
	return false
}

func appendUnique(ss []string, s string) []string {
	for _, e := range ss {
		if e == s {
			return ss
		}
	}
	return append(ss, s)
}

func uniqueStrings(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Dates returns the gapless local calendar days from Beginn through Ende (I4).
func Dates(r Reise) ([]string, error) {
	from, err := r.Beginn.civil()
	if err != nil {
		return nil, err
	}
	to, err := r.Ende.civil()
	if err != nil {
		return nil, err
	}
	days := eachDate(from, to)
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.String()
	}
	return out, nil
}

// SachbezugBetrag reports the year's breakfast and main-meal values in cents.
func SachbezugBetrag(year *satz.Year, meal string) int64 {
	if year == nil {
		return 0
	}
	if meal == "fruehstueck" {
		return year.Inland.SachbezugFruehstueck
	}
	return year.Inland.SachbezugHauptmahlzeit
}
