package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "embed"
)

//go:embed templates/abrechnung.typ
var template []byte

// Source is one receipt file the PDF and ZIP should carry.
type Source struct {
	Name   string
	MIME   string
	Bytes  []byte
	Art    string
	Nummer string
	Seite  int
	Kopf   string
	Text   string
}

// Result is one rendered export.
type Result struct {
	PDF  []byte
	ZIP  []byte
	JSON []byte
}

// Options tunes the Typst pipeline. Zero fields fall back to the SPEC defaults:
// a 60s timeout, JPEG quality 70 and PDF standard a-3b.
type Options struct {
	Timeout     time.Duration
	JPEGQuality int
	PDFStandard string
}

// Render builds the export with the SPEC defaults. It is a convenience wrapper
// around RenderWith for callers that do not need to tune the pipeline.
func Render(typst string, snap Snapshot, sources []Source, draft bool) (Result, error) {
	return RenderWith(typst, snap, sources, draft, Options{})
}

// RenderWith builds a PDF/A-3b and the ZIP described in SPEC 7.2, tuning the
// pipeline with opt. Zero fields in opt fall back to the SPEC defaults.
func RenderWith(typst string, snap Snapshot, sources []Source, draft bool, opt Options) (Result, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = 60 * time.Second
	}
	if opt.JPEGQuality <= 0 {
		opt.JPEGQuality = 70
	}
	if opt.PDFStandard == "" {
		opt.PDFStandard = "a-3b"
	}
	body, err := canonical(snap)
	if err != nil {
		return Result{}, err
	}
	dir, err := os.MkdirTemp("", "rk-export-")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "abrechnung.typ"), template, 0o644); err != nil {
		return Result{}, err
	}
	ctx, cancel := withTimeout(opt.Timeout)
	defer cancel()
	var extra []memFile
	var bilder []map[string]any
	var attached []map[string]any
	for i, src := range sources {
		base := safeName(src.Name)
		if base == "datei" {
			base = fmt.Sprintf("datei-%d", i+1)
		}
		switch src.Art {
		case "pdf":
			pdfName := "src-" + base
			if !strings.HasSuffix(pdfName, ".pdf") {
				pdfName += ".pdf"
			}
			if err := os.WriteFile(filepath.Join(dir, pdfName), src.Bytes, 0o644); err != nil {
				return Result{}, err
			}
			pages, err := rasterPDF(ctx, typst, dir, pdfName, opt.JPEGQuality)
			if err != nil {
				return Result{}, err
			}
			for n, jpg := range pages {
				imgName := fmt.Sprintf("img-%d-%d.jpg", i+1, n+1)
				if err := os.WriteFile(filepath.Join(dir, imgName), jpg, 0o644); err != nil {
					return Result{}, err
				}
				bilder = append(bilder, map[string]any{
					"art": "bild", "pfad": imgName, "alt": src.Nummer,
					"kopf": fmt.Sprintf("%s · %d/%d", src.Kopf, n+1, len(pages)), "text": "",
				})
			}
			attached = append(attached, map[string]any{
				"pfad": pdfName, "beschreibung": src.Nummer, "mime": "application/pdf",
			})
			extra = append(extra, memFile{name: base, body: src.Bytes})
		case "xml":
			xmlName := "src-" + base
			if err := os.WriteFile(filepath.Join(dir, xmlName), src.Bytes, 0o644); err != nil {
				return Result{}, err
			}
			text := src.Text
			if text == "" {
				text = src.Nummer
			}
			bilder = append(bilder, map[string]any{
				"art": "text", "pfad": "", "alt": src.Nummer, "kopf": src.Kopf, "text": text,
			})
			mime := src.MIME
			if mime == "" {
				mime = "application/xml"
			}
			attached = append(attached, map[string]any{
				"pfad": xmlName, "beschreibung": src.Nummer, "mime": mime,
			})
			extra = append(extra, memFile{name: base, body: src.Bytes})
		default:
			jpg, err := asJPEG(src.Bytes, opt.JPEGQuality)
			if err != nil {
				return Result{}, err
			}
			imgName := fmt.Sprintf("img-%d.jpg", i+1)
			if err := os.WriteFile(filepath.Join(dir, imgName), jpg, 0o644); err != nil {
				return Result{}, err
			}
			kopf := src.Kopf
			if kopf == "" {
				kopf = src.Nummer
			}
			bilder = append(bilder, map[string]any{
				"art": "bild", "pfad": imgName, "alt": src.Nummer, "kopf": kopf, "text": "",
			})
			zipName := base
			if !strings.Contains(zipName, ".") {
				zipName += ".jpg"
			}
			extra = append(extra, memFile{name: zipName, body: jpg})
		}
	}
	view := viewModel(snap, bilder, attached, draft)
	raw, err := json.Marshal(view)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "render.json"), raw, 0o644); err != nil {
		return Result{}, err
	}
	pdf, err := compilePDF(ctx, typst, dir, snap.Stamp(), opt.PDFStandard)
	if err != nil {
		return Result{}, err
	}
	pack, err := buildZIP(snap, pdf, extra)
	if err != nil {
		return Result{}, err
	}
	return Result{PDF: pdf, ZIP: pack, JSON: body}, nil
}

func canonical(s Snapshot) ([]byte, error) {
	if s.SchemaVersion == "" {
		s.SchemaVersion = SchemaVersion
	}
	if s.Satztabellen == nil {
		s.Satztabellen = []SatzJahr{}
	}
	if s.Reisen == nil {
		s.Reisen = []Reise{}
	}
	if s.Belege == nil {
		s.Belege = []Beleg{}
	}
	if s.WarnungenQuittiert == nil {
		s.WarnungenQuittiert = []Quittung{}
	}
	if s.Protokoll == nil {
		s.Protokoll = []Ereignis{}
	}
	if s.Abrechnung.Vorschuesse == nil {
		s.Abrechnung.Vorschuesse = []Vorschuss{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func asJPEG(raw []byte, quality int) ([]byte, error) {
	if len(raw) > 2 && raw[0] == 0xFF && raw[1] == 0xD8 {
		return raw, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type tableData struct {
	Head []string   `json:"head"`
	Rows [][]string `json:"rows"`
	Foot []string   `json:"foot"`
}

func newTable(head []string, rows [][]string, foot []string) tableData {
	if rows == nil {
		rows = [][]string{}
	}
	if foot == nil {
		foot = []string{}
	}
	return tableData{Head: head, Rows: rows, Foot: foot}
}

type sumRow struct {
	Label  string `json:"label"`
	Betrag string `json:"betrag"`
	Sub    bool   `json:"sub"`
	Strong bool   `json:"strong"`
}

func viewModel(s Snapshot, bilder, attached []map[string]any, draft bool) map[string]any {
	de := s.ExportSprache != "en"
	t := func(a, b string) string {
		if de {
			return a
		}
		return b
	}
	sum := s.Abrechnung.Summen
	var summen []sumRow
	main := func(label string, cents int64) {
		summen = append(summen, sumRow{Label: label, Betrag: euroLabel(cents)})
	}
	sub := func(label string, cents int64) {
		if cents != 0 {
			summen = append(summen, sumRow{Label: label, Betrag: euroLabel(cents), Sub: true})
		}
	}
	main(t("Fahrtkosten", "Transport"), sum.FahrtkostenCent)
	sub(t("davon Kilometerpauschale", "of which mileage allowance"), sum.DavonKilometerCent)
	main(t("Verpflegungsmehraufwand", "Meal allowances"), sum.VerpflegungCent)
	sub(t("davon Pauschalen", "of which flat rates"), sum.DavonPauschalenCent)
	main(t("Übernachtung", "Lodging"), sum.UebernachtungCent)
	sub(t("davon Pauschalen", "of which flat rates"), sum.DavonUebernachtungPauschCent)
	main(t("Reisenebenkosten", "Incidentals"), sum.ReisenebenkostenCent)
	main(t("Bewirtung", "Entertainment"), sum.BewirtungCent)
	summen = append(summen, sumRow{Label: t("Erstattungsbetrag", "Reimbursement"), Betrag: euroLabel(s.Abrechnung.ErstattungCent), Strong: true})
	for _, v := range s.Abrechnung.Vorschuesse {
		note := v.Notiz
		if note == "" {
			note = dateLabel(v.Datum, de)
		} else {
			note += " (" + dateLabel(v.Datum, de) + ")"
		}
		summen = append(summen, sumRow{Label: t("abzüglich Vorschuss ", "less advance ") + note, Betrag: euroLabel(-v.Cent)})
	}
	payLabel := t("Auszahlungsbetrag", "Amount payable")
	if s.Abrechnung.AuszahlungCent < 0 {
		payLabel = t("Rückzahlung an den Arbeitgeber", "Repayment to the employer")
	}

	var overview [][]string
	var trips []map[string]any
	var bewirt [][]string
	var eigen [][]string
	vatDE := map[int64]*vatAcc{}
	vatForeign := map[string]*vatAcc{}
	var totalKm, bewAbz, bewNicht int64
	kmRates := map[int64]bool{}
	for _, reise := range s.Reisen {
		zeitraum := stampLabel(reise.Beginn, de) + " – " + stampLabel(reise.Ende, de)
		overview = append(overview, []string{strconv.Itoa(reise.Nr), reise.Anlass, zeitraum, euroLabel(reise.SummeCent)})
		var orte []string
		for _, o := range reise.Ortswechsel {
			orte = append(orte, strings.TrimSpace(o.LandISO+" "+o.Ort))
		}
		var days [][]string
		var rules []string
		seenRule := map[string]bool{}
		var sPausch, sKuerz, sVerpf, sUebern int64
		for _, day := range reise.Reisetage {
			kuerz := day.KuerzungFruehCent + day.KuerzungMittagCent + day.KuerzungAbendCent
			sPausch += day.PauschaleCent
			sKuerz += kuerz
			sVerpf += day.VerpflegungCent
			sUebern += day.UebernachtungCent
			for _, id := range day.RegelIDs {
				if !seenRule[id] {
					seenRule[id] = true
					rules = append(rules, id)
				}
			}
			days = append(days, []string{
				dateLabel(day.Datum, de), tagesartLabel(day.Tagesart, de),
				strings.TrimSpace(day.LandISO + " " + day.Satzort), hoursLabel(day.AbwesenheitMin),
				euroLabel(day.Satz24hCent), euroLabel(day.PauschaleCent), minusLabel(kuerz),
				euroLabel(day.VerpflegungCent), euroLabel(day.UebernachtungCent),
			})
		}
		var rides [][]string
		var rideKm, rideSum int64
		for _, f := range reise.Fahrten {
			totalKm += f.KmGesamt
			rideKm += f.KmGesamt
			rideSum += f.BetragCent
			kmRates[f.SatzCent] = true
			way := f.Start + " – " + f.Ziel
			if f.HinUndZurueck {
				way += " (" + t("hin und zurück", "return trip") + ")"
			}
			rides = append(rides, []string{
				dateLabel(f.Datum, de), way, fahrzeugLabel(f.Fahrzeugart, de),
				strconv.FormatInt(f.KmGesamt, 10), centsPerKm(f.SatzCent), euroLabel(f.BetragCent),
			})
		}
		var expenses [][]string
		var eNetto, eUst, eBrutto int64
		for _, a := range reise.Ausgaben {
			var rates []string
			var netto, ust int64
			for _, p := range a.Anteile {
				addVAT(vatDE, vatForeign, p)
				netto += p.NettoCent
				ust += p.UstCent
				if r := percentShort(p.Satz, p.Land, de); !contains(rates, r) {
					rates = append(rates, r)
				}
			}
			eNetto += netto
			eUst += ust
			eBrutto += a.BetragEURCent
			who := a.Leistender
			if a.Beschreibung != "" {
				if who != "" {
					who += " · "
				}
				who += a.Beschreibung
			}
			if a.Rechnungsart == "eigenbeleg" {
				who += " [" + t("Eigenbeleg", "substitute receipt") + "]"
				eigen = append(eigen, []string{dateLabel(a.Datum, de), a.Leistender, a.Beschreibung, a.Belegnummern, euroLabel(a.BetragEURCent)})
			}
			if a.Waehrung != "" && a.Waehrung != "EUR" {
				who += fmt.Sprintf(" (%s %s @ %s)", euroCSV(a.BetragCent), a.Waehrung, commaDecimal(a.Kurs))
			}
			expenses = append(expenses, []string{
				dateLabel(a.Datum, de), kostenartLabel(a.Kostenart, de), who, a.Belegnummern,
				strings.Join(rates, ", "), euroLabel(netto), euroLabel(ust), euroLabel(a.BetragEURCent),
			})
			if a.Bewirtung != nil {
				bewAbz += a.BewirtungAbziehbarCent
				bewNicht += a.BewirtungNichtCent
				bewirt = append(bewirt, []string{
					a.Belegnummern, dateLabel(a.Datum, de), a.Bewirtung.Anlass, a.Bewirtung.Ort,
					a.Bewirtung.Teilnehmer, a.Bewirtung.Bewirtender,
					euroLabel(a.BewirtungAbziehbarCent), euroLabel(a.BewirtungNichtCent),
				})
			}
		}
		trip := map[string]any{
			"kopf":     fmt.Sprintf("%d · %s", reise.Nr, reise.Anlass),
			"projekt":  reise.Projekt,
			"zeitraum": zeitraum,
			"orte":     strings.Join(orte, " → "),
			"regeln":   strings.Join(rules, ", "),
			"summe":    euroLabel(reise.SummeCent),
			"tage": newTable([]string{
				t("Datum", "Date"), t("Tagesart", "Day type"), t("Ort", "Place"), t("Dauer", "Duration"),
				t("Satz 24 h", "24 h rate"), t("Pauschale", "Allowance"), t("Kürzung", "Deduction"),
				t("Verpflegung", "Meals"), t("Übernachtung", "Lodging"),
			}, days, []string{t("Summe", "Total"), "", "", "", "", euroLabel(sPausch), minusLabel(sKuerz), euroLabel(sVerpf), euroLabel(sUebern)}),
			"fahrten": newTable([]string{
				t("Datum", "Date"), t("Strecke", "Route"), t("Fahrzeug", "Vehicle"), "km", t("Satz", "Rate"), t("Betrag", "Amount"),
			}, rides, []string{t("Summe", "Total"), "", "", strconv.FormatInt(rideKm, 10), "", euroLabel(rideSum)}),
			"ausgaben": newTable([]string{
				t("Datum", "Date"), t("Kostenart", "Type"), t("Leistender / Beschreibung", "Vendor / description"), t("Beleg", "Receipt"),
				t("USt-Satz", "VAT rate"), t("Netto", "Net"), t("USt", "VAT"), t("Brutto", "Gross"),
			}, expenses, []string{t("Summe", "Total"), "", "", "", "", euroLabel(eNetto), euroLabel(eUst), euroLabel(eBrutto)}),
		}
		trips = append(trips, trip)
	}

	var protocol [][]string
	for _, ev := range s.Protokoll {
		protocol = append(protocol, []string{stampLabel(ev.Zeitpunkt, de), ev.Aktion, ev.Grund})
	}
	nutzer := s.Nutzer.Name
	if s.Nutzer.Personalnummer != "" {
		nutzer += " · " + t("Personalnr. ", "Staff no. ") + s.Nutzer.Personalnummer
	}
	meta := []string{
		s.Abrechnung.Nummer + " · v" + strconv.Itoa(s.Abrechnung.Version),
		dateLabel(s.Abrechnung.Von, de) + " – " + dateLabel(s.Abrechnung.Bis, de),
	}
	if bilder == nil {
		bilder = []map[string]any{}
	}
	if attached == nil {
		attached = []map[string]any{}
	}
	var warnings []string
	for _, w := range s.WarnungenQuittiert {
		warnings = append(warnings, w.Code+" · "+warnText(w.Code, de)+" · "+w.ObjektID)
	}
	if warnings == nil {
		warnings = []string{}
	}
	var sources []string
	for _, jahr := range s.Satztabellen {
		sources = append(sources, fmt.Sprintf("%d · %s", jahr.Jahr, jahr.Quelle))
	}
	if len(sources) == 0 {
		sources = []string{t("Keine Satztabelle im Snapshot.", "No rate table in the snapshot.")}
	}
	var vorsteuer int64
	for _, row := range vatDE {
		vorsteuer += row.vorst
	}
	var tax [][]string
	addTax := func(cents int64, label, basis string) {
		if cents != 0 {
			tax = append(tax, []string{label, euroLabel(cents), basis})
		}
	}
	addTax(sum.VerpflegungCent, t("Verpflegungspauschalen (steuerfrei)", "Meal allowances (tax-free)"), "§ 9 Abs. 4a, § 3 Nr. 16 EStG")
	addTax(sum.DavonUebernachtungPauschCent, t("Übernachtungspauschalen (steuerfrei)", "Lodging flat rates (tax-free)"), "§ 3 Nr. 16 EStG")
	if sum.DavonKilometerCent != 0 {
		var rates []string
		for _, c := range sortedKeys(kmRates) {
			rates = append(rates, centsPerKm(c))
		}
		addTax(sum.DavonKilometerCent,
			fmt.Sprintf("%s (%d km)", t("Kilometerpauschale (steuerfrei)", "Mileage allowance (tax-free)"), totalKm),
			strings.Join(rates, ", ")+" · § 9 Abs. 1 Satz 3 Nr. 4a EStG")
	}
	addTax(vorsteuer, t("Abziehbare Vorsteuer (Inland)", "Deductible input VAT (Germany)"), vatBasis(vatDE, de))
	addTax(bewAbz, t("Bewirtung, abziehbar", "Entertainment, deductible"), "§ 4 Abs. 5 Satz 1 Nr. 2 EStG")
	addTax(bewNicht, t("Bewirtung, nicht abziehbar", "Entertainment, non-deductible"), "§ 4 Abs. 5 Satz 1 Nr. 2 EStG")

	return map[string]any{
		"lang": ifLang(de), "entwurf": draft, "titel": s.Abrechnung.Titel,
		"nummer": s.Abrechnung.Nummer, "version": s.Abrechnung.Version,
		"fuss":        t("erstellt mit vc-reisekostenabrechnung", "created with vc-reisekostenabrechnung") + " " + s.AppVersion,
		"arbeitgeber": s.Arbeitgeber.Name, "anschrift": s.Arbeitgeber.Anschrift,
		"nutzer": s.Nutzer.Name, "nutzer_zeile": nutzer, "meta": meta,
		"label_arbeitnehmer": t("Reisende Person", "Traveller"),
		"label_arbeitgeber":  t("Arbeitgeber", "Employer"),
		"summen_titel":       t("Zusammenstellung", "Summary"),
		"summen":             nonNilSum(summen),
		"auszahlung_label":   payLabel, "auszahlung": euroLabel(s.Abrechnung.AuszahlungCent),
		"steuer_titel":     t("Steuerliche Übersicht", "Tax overview"),
		"steuer_head":      []string{t("Position", "Item"), t("Betrag", "Amount"), t("Grundlage", "Basis")},
		"steuer":           nonNilRows(tax),
		"hinweis":          t("Steuerfrei nach § 3 Nr. 16 EStG, soweit nicht anders gekennzeichnet.", "Tax-free under § 3 no. 16 EStG unless marked otherwise."),
		"bestaetigung":     t("Elektronisch eingereicht am ", "Submitted electronically on ") + stampLabel(s.Abrechnung.EingereichtAm, de),
		"eingereicht":      s.Abrechnung.EingereichtAm != "",
		"freigabe":         t("Freigabe Arbeitgeber (Datum, Unterschrift)", "Employer approval (date, signature)"),
		"uebersicht_titel": t("Reiseübersicht", "Trips"),
		"uebersicht_head":  []string{"#", t("Anlass", "Purpose"), t("Zeitraum", "Period"), t("Summe", "Total")},
		"uebersicht":       nonNilRows(overview),
		"reisen":           trips,
		"stationen_label":  t("Stationen", "Stops"),
		"projekt_label":    t("Projekt", "Project"),
		"regeln_label":     t("Angewandte Regeln", "Applied rules"),
		"tage_titel":       t("Tagesberechnung", "Daily calculation"),
		"fahrten_titel":    t("Fahrten", "Journeys"),
		"ausgaben_titel":   t("Ausgaben", "Expenses"),
		"summe_label":      t("Reisesumme", "Trip total"),
		"bewirtung_titel":  t("Bewirtungs-Eigenbelege", "Entertainment records"),
		"bewirtung_head": []string{
			t("Beleg", "Receipt"), t("Datum", "Date"), t("Anlass", "Occasion"), t("Ort", "Place"),
			t("Teilnehmer", "Guests"), t("Bewirtender", "Host"), t("abziehbar", "deductible"), t("nicht abziehbar", "non-deductible"),
		},
		"bewirtungen":       nonNilRows(bewirt),
		"eigenbeleg_titel":  t("Eigenbelege", "Substitute receipts"),
		"eigenbeleg_head":   []string{t("Datum", "Date"), t("Leistender", "Vendor"), t("Beschreibung", "Description"), t("Beleg", "Receipt"), t("Betrag", "Amount")},
		"eigenbelege":       nonNilRows(eigen),
		"ust_titel":         t("Umsatzsteuer", "VAT"),
		"ust_head":          []string{t("Satz", "Rate"), "Netto", "USt", "Brutto", t("davon Vorsteuer", "of which input VAT")},
		"ust":               vatRows(vatDE, de),
		"ust_ausland_titel": t("Ausländische Umsatzsteuer", "Foreign VAT"),
		"ust_ausland_head":  []string{t("Land", "Country"), t("Satz", "Rate"), "Netto", "USt", "Brutto"},
		"ust_ausland":       foreignRows(vatForeign, de),
		"keine_ust":         t("Keine Umsatzsteuer auf Einzelnachweisen.", "No VAT on individual receipts."),
		"ust_hinweis":       t("Ausländische Umsatzsteuer ist keine deutsche Vorsteuer (Vorsteuer-Vergütung).", "Foreign VAT is not German input VAT."),
		"ust_pauschale":     t("Pauschalen ohne Vorsteuer.", "Allowances do not include input VAT."),
		"hinweise_titel":    t("Hinweise und quittierte Warnungen", "Notes and acknowledged warnings"),
		"warnungen":         warnings,
		"keine_warnung":     t("Keine quittierten Warnungen.", "No acknowledged warnings."),
		"quellen_titel":     t("Rechtsgrundlagen und Satztabellen", "Legal basis and rate tables"),
		"quellen":           append([]string{t("§ 3 Nr. 16 EStG; BMF-Schreiben zu Reisekosten.", "§ 3 no. 16 EStG; BMF circulars on travel expenses.")}, sources...),
		"protokoll_titel":   t("Protokoll", "Protocol"),
		"protokoll_head":    []string{t("Zeitpunkt", "Time"), t("Ereignis", "Event"), t("Grund", "Reason")},
		"protokoll":         nonNilRows(protocol),
		"kein_protokoll":    t("Keine Protokolleinträge im Snapshot.", "No protocol entries in the snapshot."),
		"bilder":            bilder, "dateien": attached,
	}
}

func nonNilRows(rows [][]string) [][]string {
	if rows == nil {
		return [][]string{}
	}
	return rows
}

func nonNilSum(rows []sumRow) []sumRow {
	if rows == nil {
		return []sumRow{}
	}
	return rows
}

func contains(items []string, v string) bool {
	for _, i := range items {
		if i == v {
			return true
		}
	}
	return false
}

func sortedKeys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortInt(out)
	return out
}

// dateLabel renders the date part of an ISO stamp: 01.10.2026 (de) or 01/10/2026 (en).
func dateLabel(v string, de bool) string {
	if len(v) < 10 || v[4] != '-' || v[7] != '-' {
		return v
	}
	sep := "/"
	if de {
		sep = "."
	}
	return v[8:10] + sep + v[5:7] + sep + v[:4]
}

// stampLabel renders an ISO stamp as a date plus HH:MM when a time is present.
func stampLabel(v string, de bool) string {
	out := dateLabel(v, de)
	if len(v) >= 16 && (v[10] == 'T' || v[10] == ' ') {
		out += " " + v[11:16]
	}
	return out
}

func hoursLabel(minutes int) string {
	return fmt.Sprintf("%d:%02d h", minutes/60, minutes%60)
}

func minusLabel(cents int64) string {
	if cents == 0 {
		return euroLabel(0)
	}
	return euroLabel(-cents)
}

func centsPerKm(cents int64) string {
	return fmt.Sprintf("%d,%02d €/km", cents/100, cents%100)
}

// percentShort prints 1900 as "19 %" and 550 as "5,5 %". Foreign rates carry the country.
func percentShort(hundredths int64, land string, de bool) string {
	whole, frac := hundredths/100, hundredths%100
	out := strconv.FormatInt(whole, 10)
	if frac != 0 {
		out += "," + strings.TrimRight(fmt.Sprintf("%02d", frac), "0")
	}
	out += " %"
	if land != "" && land != "DE" {
		out = land + " " + out
	}
	return out
}

func tagesartLabel(v string, de bool) string {
	names := map[string][2]string{
		"anreise": {"Anreise", "Arrival"}, "anreisetag": {"Anreise", "Arrival"},
		"abreise": {"Abreise", "Departure"}, "abreisetag": {"Abreise", "Departure"},
		"zwischentag": {"Zwischentag", "Intermediate day"},
		"eintaegig":   {"Eintägig", "Single day"},
		"ueber_nacht": {"Über Nacht", "Overnight"},
	}
	return pick(names, v, de)
}

func kostenartLabel(v string, de bool) string {
	names := map[string][2]string{
		"fahrtkosten": {"Fahrtkosten", "Transport"}, "verpflegung": {"Verpflegung", "Meals"},
		"uebernachtung":    {"Übernachtung", "Lodging"},
		"reisenebenkosten": {"Reisenebenkosten", "Incidentals"}, "bewirtung": {"Bewirtung", "Entertainment"},
	}
	return pick(names, v, de)
}

func fahrzeugLabel(v string, de bool) string {
	names := map[string][2]string{
		"kraftwagen":            {"Kraftwagen", "Car"},
		"anderes_motorfahrzeug": {"Anderes Kfz", "Other vehicle"},
	}
	return pick(names, v, de)
}

func pick(names map[string][2]string, v string, de bool) string {
	n, ok := names[v]
	if !ok {
		return v
	}
	if de {
		return n[0]
	}
	return n[1]
}

func vatBasis(rows map[int64]*vatAcc, de bool) string {
	keys := make([]int64, 0, len(rows))
	for k, row := range rows {
		if row.vorst != 0 {
			keys = append(keys, k)
		}
	}
	sortInt(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, percentShort(k, "DE", de)+": "+euroLabel(rows[k].vorst))
	}
	return strings.Join(parts, " · ") + " · § 15 UStG"
}

type vatAcc struct {
	land          string
	satz          int64
	netto, ust    int64
	brutto, vorst int64
}

func addVAT(de map[int64]*vatAcc, foreign map[string]*vatAcc, p Anteil) {
	if p.Land == "" || p.Land == "DE" {
		row := de[p.Satz]
		if row == nil {
			row = &vatAcc{land: "DE", satz: p.Satz}
			de[p.Satz] = row
		}
		row.netto += p.NettoCent
		row.ust += p.UstCent
		row.brutto += p.BruttoCent
		if p.Vorsteuer {
			row.vorst += p.UstCent
		}
		return
	}
	key := p.Land + "/" + strconv.FormatInt(p.Satz, 10)
	row := foreign[key]
	if row == nil {
		row = &vatAcc{land: p.Land, satz: p.Satz}
		foreign[key] = row
	}
	row.netto += p.NettoCent
	row.ust += p.UstCent
	row.brutto += p.BruttoCent
}

func vatRows(rows map[int64]*vatAcc, de bool) [][]string {
	keys := make([]int64, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sortInt(keys)
	out := make([][]string, 0, len(keys))
	for _, k := range keys {
		row := rows[k]
		out = append(out, []string{
			percentShort(row.satz, "DE", de), euroLabel(row.netto), euroLabel(row.ust), euroLabel(row.brutto), euroLabel(row.vorst),
		})
	}
	return out
}

func foreignRows(rows map[string]*vatAcc, de bool) [][]string {
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([][]string, 0, len(keys))
	for _, k := range keys {
		row := rows[k]
		out = append(out, []string{
			row.land, percentShort(row.satz, "DE", de), euroLabel(row.netto), euroLabel(row.ust), euroLabel(row.brutto),
		})
	}
	return out
}

func sortInt(v []int64) {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
}

func warnText(code string, de bool) string {
	deText := map[string]string{
		"W01": "Fehlender Beleg",
		"W02": "Rechnung über 250 € lautet nicht auf den Arbeitgeber",
		"W03": "Dreimonatsfrist",
		"W04": "Möglicherweise doppelt erfasst",
		"W05": "Eigenbeleg, keine Vorsteuer",
		"W06": "Bewirtung ohne TSE-Bestätigung",
		"W07": "Gestellte Mahlzeit",
		"W08": "Keine übliche Mahlzeit",
		"W09": "Kraftstoff neben Kilometerpauschale",
		"W10": "Ausländische Umsatzsteuer",
		"W11": "Ausgabedatum außerhalb der Reise",
		"W12": "Mehr gewährt als zusteht",
		"W13": "Umsatzsteuer weicht ab",
		"W14": "Reise ohne Erstattungsbetrag",
	}
	enText := map[string]string{
		"W01": "Missing receipt",
		"W02": "Invoice over 250 € is not in the employer's name",
		"W03": "Three-month limit",
		"W04": "Possible duplicate",
		"W05": "Substitute receipt, no input VAT",
		"W06": "Entertainment without a TSE confirmation",
		"W07": "Provided meal",
		"W08": "Meal above the usual limit",
		"W09": "Fuel alongside the mileage allowance",
		"W10": "Foreign VAT",
		"W11": "Expense date outside the trip",
		"W12": "More was granted than is due",
		"W13": "VAT amount differs",
		"W14": "Trip without a reimbursement amount",
	}
	if de {
		if s, ok := deText[code]; ok {
			return s
		}
		return code
	}
	if s, ok := enText[code]; ok {
		return s
	}
	return code
}

func ifLang(de bool) string {
	if de {
		return "de"
	}
	return "en"
}
