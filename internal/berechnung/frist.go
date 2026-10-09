package berechnung

import "sort"

// Dreimonatsfrist warns (W03) once a Tätigkeitsstätte series passes three months.
// It never zeroes a Pauschale by itself (SPEC 4.14).
func Dreimonatsfrist(visits []Visit) []FristWarnung {
	by := map[string][]civil{}
	for _, v := range visits {
		d, err := parseCivil(v.Datum)
		if err != nil || v.Staette == "" {
			continue
		}
		by[v.Staette] = append(by[v.Staette], d)
	}
	var out []FristWarnung
	names := make([]string, 0, len(by))
	for name := range by {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		days := uniqueSorted(by[name])
		for _, series := range splitSeries(days) {
			out = append(out, warnSeries(name, series)...)
		}
	}
	return out
}

func uniqueSorted(days []civil) []civil {
	sort.Slice(days, func(i, j int) bool { return days[j].after(days[i]) })
	out := days[:0]
	for _, d := range days {
		if len(out) == 0 || !out[len(out)-1].equal(d) {
			out = append(out, d)
		}
	}
	return out
}

func splitSeries(days []civil) [][]civil {
	if len(days) == 0 {
		return nil
	}
	var series [][]civil
	cur := []civil{days[0]}
	for i := 1; i < len(days); i++ {
		if days[i-1].daysUntil(days[i]) >= 28 {
			series = append(series, cur)
			cur = nil
		}
		cur = append(cur, days[i])
	}
	series = append(series, cur)
	return series
}

func warnSeries(name string, days []civil) []FristWarnung {
	weeks := map[string][]civil{}
	var order []string
	for _, d := range days {
		mon := mondayOf(d).String()
		if _, ok := weeks[mon]; !ok {
			order = append(order, mon)
		}
		weeks[mon] = append(weeks[mon], d)
	}
	var start civil
	found := false
	for _, mon := range order {
		if len(weeks[mon]) >= 3 {
			var err error
			start, err = parseCivil(mon)
			if err != nil {
				return nil
			}
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	ende := addMonths(start, 3).add(-1)
	var out []FristWarnung
	for _, mon := range order {
		if len(weeks[mon]) < 3 {
			continue
		}
		for _, d := range weeks[mon] {
			if !d.after(ende) {
				continue
			}
			out = append(out, FristWarnung{
				Staette: name, Datum: d.String(), Fristende: ende.String(), Code: "W03",
			})
		}
	}
	return out
}

// SuggestMeals proposes Gestellte Mahlzeiten from Ausgaben (SPEC 4.7).
func SuggestMeals(r Reise) []Vorschlag {
	var out []Vorschlag
	for _, a := range r.Ausgaben {
		switch a.Kostenart {
		case "uebernachtung":
			if !a.FruehstueckEnthalten {
				continue
			}
			for _, night := range a.Naechte {
				d, err := parseCivil(night)
				if err != nil {
					continue
				}
				out = append(out, Vorschlag{
					Datum: d.next().String(), Mahlzeit: "fruehstueck", Quelle: "uebernachtung:" + a.ID,
				})
			}
		case "verpflegung":
			if a.Mahlzeit == "" {
				continue
			}
			out = append(out, Vorschlag{Datum: a.Datum, Mahlzeit: a.Mahlzeit, Quelle: "verpflegung:" + a.ID})
		case "bewirtung":
			meal := a.Mahlzeit
			if meal == "" {
				meal = "abend"
			}
			out = append(out, Vorschlag{Datum: a.Datum, Mahlzeit: meal, Quelle: "bewirtung:" + a.ID})
		}
	}
	return out
}

// Auszahlung is Erstattung minus advances (SPEC 4.15). Negative means a repayment.
func Auszahlung(erstattung int64, vorschuesse []int64) (summe, auszahlung int64) {
	for _, v := range vorschuesse {
		summe += v
	}
	return summe, erstattung - summe
}
