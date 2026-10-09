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

// Render builds a PDF/A-3b and the ZIP described in SPEC 7.2.
func Render(typst string, snap Snapshot, sources []Source, draft bool) (Result, error) {
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
	ctx, cancel := withTimeout()
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
			pages, err := rasterPDF(ctx, typst, dir, pdfName)
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
			jpg, err := asJPEG(src.Bytes)
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
	pdf, err := compilePDF(ctx, typst, dir, snap.Stamp())
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

func asJPEG(raw []byte) ([]byte, error) {
	if len(raw) > 2 && raw[0] == 0xFF && raw[1] == 0xD8 {
		return raw, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	rows := []map[string]string{
		line(t("Fahrtkosten", "Transport"), sum.FahrtkostenCent),
		line(t("davon Kilometerpauschale", "of which mileage"), sum.DavonKilometerCent),
		line(t("Verpflegung", "Meals"), sum.VerpflegungCent),
		line(t("davon Pauschalen", "of which allowances"), sum.DavonPauschalenCent),
		line(t("Übernachtung", "Lodging"), sum.UebernachtungCent),
		line(t("davon Pauschalen", "of which allowances"), sum.DavonUebernachtungPauschCent),
		line(t("Reisenebenkosten", "Incidentals"), sum.ReisenebenkostenCent),
		line(t("Bewirtung", "Entertainment"), sum.BewirtungCent),
		line(t("Erstattungsbetrag", "Reimbursement"), s.Abrechnung.ErstattungCent),
	}
	var advances []map[string]string
	for _, v := range s.Abrechnung.Vorschuesse {
		note := v.Notiz
		if note == "" {
			note = v.Datum
		}
		advances = append(advances, map[string]string{"label": t("Vorschuss ", "Advance ") + note, "betrag": euroLabel(v.Cent)})
	}
	payLabel := t("Auszahlungsbetrag", "Amount payable")
	if s.Abrechnung.AuszahlungCent < 0 {
		payLabel = t("Rückzahlung an den Arbeitgeber", "Repayment to the employer")
	}
	var overview []map[string]string
	var trips []map[string]any
	var bewirt []string
	var eigen []string
	vatDE := map[int64]*vatAcc{}
	vatForeign := map[string]*vatAcc{}
	for _, reise := range s.Reisen {
		overview = append(overview, map[string]string{
			"nr": strconv.Itoa(reise.Nr), "text": reise.Anlass + " · " + reise.Beginn + " – " + reise.Ende,
			"summe": euroLabel(reise.SummeCent),
		})
		var orte []string
		for _, o := range reise.Ortswechsel {
			orte = append(orte, strings.TrimSpace(o.LandISO+" "+o.Ort))
		}
		var days []string
		for _, day := range reise.Reisetage {
			days = append(days, fmt.Sprintf("%s %s %d min %s %s · %s %s · %s %s/%s/%s · %s %s · %s %s · %s",
				day.Datum, day.Tagesart, day.AbwesenheitMin, day.LandISO, day.Satzort,
				t("Pauschale", "Allowance"), euroLabel(day.PauschaleCent),
				t("Kürzung F/M/A", "Deduction B/L/D"), euroLabel(day.KuerzungFruehCent), euroLabel(day.KuerzungMittagCent), euroLabel(day.KuerzungAbendCent),
				t("Verpflegung", "Meals"), euroLabel(day.VerpflegungCent),
				t("Übernachtung", "Lodging"), euroLabel(day.UebernachtungCent),
				strings.Join(day.RegelIDs, ",")))
		}
		var rides []string
		for _, f := range reise.Fahrten {
			rides = append(rides, fmt.Sprintf("%s %s – %s %d km %s", f.Datum, f.Start, f.Ziel, f.KmGesamt, euroLabel(f.BetragCent)))
		}
		var expenses []string
		for _, a := range reise.Ausgaben {
			var shares []string
			for _, p := range a.Anteile {
				shares = append(shares, fmt.Sprintf("%s %s", p.Land, percentLabel(p.Satz)))
				addVAT(vatDE, vatForeign, p)
			}
			expenses = append(expenses, fmt.Sprintf("%s %s %s %s %s %s", a.Datum, a.Kostenart, a.Leistender, a.Belegnummern, euroLabel(a.BetragEURCent), strings.Join(shares, ", ")))
			if a.Rechnungsart == "eigenbeleg" {
				eigen = append(eigen, fmt.Sprintf("%s · %s · %s · %s · %s", a.Datum, a.Leistender, a.Beschreibung, a.Belegnummern, euroLabel(a.BetragEURCent)))
			}
			if a.Bewirtung != nil {
				bewirt = append(bewirt, fmt.Sprintf("%s · %s · %s · %s · %s · %s · %s %s · %s %s · %s",
					a.Belegnummern, a.Bewirtung.Anlass, a.Bewirtung.Ort, a.Datum, a.Bewirtung.Teilnehmer, a.Bewirtung.Bewirtender,
					t("abziehbar", "deductible"), euroLabel(a.BewirtungAbziehbarCent),
					t("nicht abziehbar", "non-deductible"), euroLabel(a.BewirtungNichtCent),
					a.Bewirtung.Bestaetigt))
			}
		}
		trips = append(trips, map[string]any{
			"kopf": fmt.Sprintf("%d %s", reise.Nr, reise.Anlass),
			"orte": strings.Join(orte, ", "),
			"tage": stringsOrEmpty(days), "fahrten": stringsOrEmpty(rides), "ausgaben": stringsOrEmpty(expenses),
			"summe": euroLabel(reise.SummeCent),
		})
	}
	var protocol []string
	for _, ev := range s.Protokoll {
		line := ev.Zeitpunkt + " " + ev.Aktion
		if ev.Grund != "" {
			line += " (" + ev.Grund + ")"
		}
		protocol = append(protocol, line)
	}
	nutzer := s.Nutzer.Name
	if s.Nutzer.Personalnummer != "" {
		nutzer += " · " + s.Nutzer.Personalnummer
	}
	meta := s.Abrechnung.Nummer + " · v" + strconv.Itoa(s.Abrechnung.Version) + " · " + s.Abrechnung.Von + " – " + s.Abrechnung.Bis
	if s.Abrechnung.EingereichtAm != "" {
		meta += " · " + s.Abrechnung.EingereichtAm
	}
	if bilder == nil {
		bilder = []map[string]any{}
	}
	if attached == nil {
		attached = []map[string]any{}
	}
	if advances == nil {
		advances = []map[string]string{}
	}
	if overview == nil {
		overview = []map[string]string{}
	}
	if trips == nil {
		trips = []map[string]any{}
	}
	if bewirt == nil {
		bewirt = []string{}
	}
	if eigen == nil {
		eigen = []string{}
	}
	if protocol == nil {
		protocol = []string{}
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
	summen := append(rows, advances...)
	return map[string]any{
		"lang": ifLang(de), "entwurf": draft, "titel": s.Abrechnung.Titel,
		"nummer": s.Abrechnung.Nummer, "version": s.Abrechnung.Version,
		"fuss":        t("erstellt mit vc-reisekostenabrechnung", "created with vc-reisekostenabrechnung") + " " + s.AppVersion,
		"arbeitgeber": s.Arbeitgeber.Name, "anschrift": s.Arbeitgeber.Anschrift,
		"nutzer": s.Nutzer.Name, "nutzer_zeile": nutzer, "meta": meta,
		"summen": summen, "auszahlung_label": payLabel, "auszahlung": euroLabel(s.Abrechnung.AuszahlungCent),
		"hinweis":           t("Steuerfrei nach § 3 Nr. 16 EStG, soweit nicht anders gekennzeichnet.", "Tax-free under § 3 no. 16 EStG unless marked otherwise."),
		"bestaetigung":      t("Elektronisch eingereicht.", "Submitted electronically.") + " " + s.Abrechnung.EingereichtAm,
		"freigabe":          t("Freigabe Arbeitgeber", "Employer approval"),
		"uebersicht_titel":  t("Reiseübersicht", "Trips"),
		"uebersicht":        overview,
		"tage_titel":        t("Tagesberechnung", "Daily calculation"),
		"fahrten_titel":     t("Fahrten", "Journeys"),
		"ausgaben_titel":    t("Ausgaben", "Expenses"),
		"summe_label":       t("Reisesumme", "Trip total"),
		"reisen":            trips,
		"bewirtung_titel":   t("Bewirtungs-Eigenbelege", "Entertainment records"),
		"bewirtungen":       bewirt,
		"eigenbeleg_titel":  t("Eigenbelege", "Substitute receipts"),
		"eigenbelege":       eigen,
		"ust_titel":         t("Umsatzsteuer", "VAT"),
		"ust":               vatLines(vatDE, de),
		"ust_ausland_titel": t("Ausländische Umsatzsteuer", "Foreign VAT"),
		"ust_ausland":       foreignLines(vatForeign),
		"keine_ust":         t("Keine Umsatzsteuer auf Einzelnachweisen.", "No VAT on individual receipts."),
		"ust_hinweis":       t("Ausländische Umsatzsteuer ist keine deutsche Vorsteuer (Vorsteuer-Vergütung).", "Foreign VAT is not German input VAT."),
		"ust_pauschale":     t("Pauschalen ohne Vorsteuer.", "Allowances do not include input VAT."),
		"hinweise_titel":    t("Hinweise und quittierte Warnungen", "Notes and acknowledged warnings"),
		"warnungen":         warnings,
		"keine_warnung":     t("Keine quittierten Warnungen.", "No acknowledged warnings."),
		"quellen_titel":     t("Rechtsgrundlagen und Satztabellen", "Legal basis and rate tables"),
		"quellen":           append([]string{t("§ 3 Nr. 16 EStG; BMF-Schreiben zu Reisekosten.", "§ 3 no. 16 EStG; BMF circulars on travel expenses.")}, sources...),
		"protokoll_titel":   t("Protokoll", "Protocol"),
		"protokoll":         protocol,
		"kein_protokoll":    t("Keine Protokolleinträge im Snapshot.", "No protocol entries in the snapshot."),
		"bilder":            bilder, "dateien": attached,
	}
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

func vatLines(rows map[int64]*vatAcc, de bool) []string {
	if len(rows) == 0 {
		return []string{}
	}
	keys := make([]int64, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sortInt(keys)
	label := "vorsteuerfähig"
	if !de {
		label = "input VAT"
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		row := rows[k]
		out = append(out, fmt.Sprintf("DE %s · Netto %s · USt %s · Brutto %s · %s %s",
			percentLabel(row.satz), euroLabel(row.netto), euroLabel(row.ust), euroLabel(row.brutto), label, euroLabel(row.vorst)))
	}
	return out
}

func foreignLines(rows map[string]*vatAcc) []string {
	if len(rows) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		row := rows[k]
		out = append(out, fmt.Sprintf("%s %s · Netto %s · USt %s · Brutto %s",
			row.land, percentLabel(row.satz), euroLabel(row.netto), euroLabel(row.ust), euroLabel(row.brutto)))
	}
	return out
}

func percentLabel(hundredths int64) string {
	sign := ""
	if hundredths < 0 {
		sign = "-"
		hundredths = -hundredths
	}
	return fmt.Sprintf("%s%d,%02d %%", sign, hundredths/100, hundredths%100)
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

func stringsOrEmpty(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func line(label string, cents int64) map[string]string {
	return map[string]string{"label": label, "betrag": euroLabel(cents)}
}

func ifLang(de bool) string {
	if de {
		return "de"
	}
	return "en"
}
