// Package satz loads the shipped Satztabellen and resolves a rate.
package satz

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/BlackDark/vc-reisekostenabrechnung/docs/research/data"
	"github.com/BlackDark/vc-reisekostenabrechnung/satztabellen"
)

const (
	WegExakt     = "exakt"
	WegUebriges  = "uebriges"
	WegErsatz    = "ersatz"
	WegLuxemburg = "luxemburg"
	WegInland    = "inland"
)

// Row is one Auslandssatz. Satzort "" is the whole country or "im Übrigen".
type Row struct {
	LandISO       string
	LandNameDE    string
	Satzort       string
	OrtName       string
	VMA24h        int64
	VMA8h         int64
	Uebernachtung int64
}

// Inland holds the domestic figures for one year, in cents or hundredths of a percent.
type Inland struct {
	VMA24h                   int64
	VMA8h                    int64
	KuerzungFruehstueckPct   int64
	KuerzungHauptmahlzeitPct int64
	Uebernachtung            int64
	KmKraftwagen             int64
	KmAnderes                int64
	SachbezugFruehstueck     int64
	SachbezugHauptmahlzeit   int64
	UeblicheMahlzeit         int64
	Kleinbetrag              int64
	BewirtungAbzug           int64
	AufbewahrungJahre        int64
	UstSaetze                string
	Ersatz                   map[string]string
	Flug                     string
	Schiff                   string
	Quelle                   string
	GastroHinweis            bool
}

// Year is one Satztabelle plus its Auslandssätze.
type Year struct {
	Jahr   int
	Status string
	Inland Inland
	Rows   []Row
	byLand map[string]map[string]Row
}

// Hit is the rate Satz resolved, including which fallback applied.
type Hit struct {
	Jahr          int
	LandISO       string
	LandNameDE    string
	Satzort       string
	OrtName       string
	VMA24h        int64
	VMA8h         int64
	Uebernachtung int64
	Weg           string
	ErsatzVon     string
}

// Catalog is the shipped 2024–2026 data.
type Catalog struct {
	Names map[string]string
	Years map[int]*Year
}

// Load reads the embedded CSVs. Every BMF country name must be mapped.
func Load() (*Catalog, error) {
	names, err := loadNames()
	if err != nil {
		return nil, err
	}
	cat := &Catalog{Names: names, Years: map[int]*Year{}}
	for _, year := range []int{2024, 2025, 2026} {
		y, err := loadYear(year, names)
		if err != nil {
			return nil, err
		}
		cat.Years[year] = y
	}
	return cat, nil
}

func loadNames() (map[string]string, error) {
	raw, err := satztabellen.Files.ReadFile("laender.csv")
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(string(raw)))
	r.FieldsPerRecord = 2
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if header[0] != "bmf_name" || header[1] != "land_iso" {
		return nil, errors.New("laender.csv header")
	}
	out := map[string]string{}
	seenISO := map[string]string{}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name, iso := rec[0], rec[1]
		if len(iso) != 2 || strings.ToUpper(iso) != iso {
			return nil, fmt.Errorf("land_iso %q for %q", iso, name)
		}
		if prev, ok := seenISO[iso]; ok {
			return nil, fmt.Errorf("duplicate ISO %s for %q and %q", iso, prev, name)
		}
		seenISO[iso] = name
		out[name] = iso
	}
	return out, nil
}

func loadYear(year int, names map[string]string) (*Year, error) {
	raw, err := data.Files.ReadFile(fmt.Sprintf("auslandspauschalen_%d.csv", year))
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(string(raw)))
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	want := []string{"jahr", "land", "ort", "vma_24h_eur", "vma_an_abreise_8h_eur", "uebernachtung_ag_pauschal_eur"}
	if strings.Join(header, ",") != strings.Join(want, ",") {
		return nil, fmt.Errorf("unexpected header %d: %v", year, header)
	}
	y := &Year{Jahr: year, Status: "aktiv", Inland: InlandFor(year), byLand: map[string]map[string]Row{}}
	line := 1
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			return nil, err
		}
		rowYear, err := strconv.Atoi(rec[0])
		if err != nil || rowYear != year {
			return nil, fmt.Errorf("%d line %d year", year, line)
		}
		iso, ok := names[rec[1]]
		if !ok {
			return nil, fmt.Errorf("%d line %d unmapped country %q", year, line, rec[1])
		}
		vma24, err := eurosToCents(rec[3])
		if err != nil {
			return nil, fmt.Errorf("%d line %d vma: %w", year, line, err)
		}
		vma8, err := eurosToCents(rec[4])
		if err != nil {
			return nil, fmt.Errorf("%d line %d vma8: %w", year, line, err)
		}
		ueb, err := eurosToCents(rec[5])
		if err != nil {
			return nil, fmt.Errorf("%d line %d uebernachtung: %w", year, line, err)
		}
		row := Row{
			LandISO: iso, LandNameDE: rec[1], Satzort: SatzortKey(iso, rec[2]), OrtName: ortName(rec[2]),
			VMA24h: vma24, VMA8h: vma8, Uebernachtung: ueb,
		}
		if _, ok := y.byLand[iso]; !ok {
			y.byLand[iso] = map[string]Row{}
		}
		if _, exists := y.byLand[iso][row.Satzort]; exists {
			return nil, fmt.Errorf("%d duplicate %s %s", year, iso, row.Satzort)
		}
		y.byLand[iso][row.Satzort] = row
		y.Rows = append(y.Rows, row)
	}
	return y, nil
}

func ortName(ort string) string {
	ort = strings.TrimSpace(ort)
	if ort == "" || ort == "im Übrigen" {
		if ort == "im Übrigen" {
			return "im Übrigen"
		}
		return ""
	}
	return ort
}

// SatzortKey builds the Satztabelle key. Empty means the whole country or "im Übrigen".
// The Paris row uses FR-PARIS, as in the spec example.
func SatzortKey(landISO, ort string) string {
	ort = strings.TrimSpace(ort)
	if ort == "" || ort == "im Übrigen" {
		return ""
	}
	name := ort
	if strings.HasPrefix(ort, "Paris") {
		name = "Paris"
	}
	return landISO + "-" + slug(name)
}

func slug(s string) string {
	s = fold(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'a' && r <= 'z':
			b.WriteRune(unicode.ToUpper(r))
		default:
			if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
				b.WriteByte('-')
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func fold(s string) string {
	return strings.NewReplacer(
		"Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
		"À", "A", "Á", "A", "Â", "A", "Ã", "A", "à", "a", "á", "a", "â", "a", "ã", "a",
		"É", "E", "È", "E", "Ê", "E", "é", "e", "è", "e", "ê", "e",
		"Í", "I", "í", "i", "Ó", "O", "Ô", "O", "Õ", "O", "ó", "o", "ô", "o", "õ", "o",
		"Ú", "U", "ú", "u", "Ç", "C", "ç", "c", "Ñ", "N", "ñ", "n",
	).Replace(s)
}

func eurosToCents(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 || n > 1_000_000 {
		return 0, fmt.Errorf("euro amount %q", raw)
	}
	return n * 100, nil
}

// InlandFor returns the statutory domestic rates. Years other than 2024–2026
// copy 2026 so a CSV import of a later year has a starting point.
func InlandFor(year int) Inland {
	sachF, sachH := int64(237), int64(457)
	quelle := "BMF 05.12.2025, GZ IV C 5 - S 2353/00094/007/012"
	gastro := true
	switch year {
	case 2024:
		sachF, sachH = 217, 413
		quelle = "BMF 21.11.2023, BStBl I S. 2076"
		gastro = false
	case 2025:
		sachF, sachH = 230, 440
		quelle = "BMF 02.12.2024, BStBl I S. 1549"
		gastro = false
	}
	erm := "ermäßigt"
	if gastro {
		erm = "ermäßigt (Gastronomie-Speisen 7 %)"
	}
	ust, err := json.Marshal([]map[string]any{
		{"satz": 0, "bezeichnung": "steuerfrei"},
		{"satz": 700, "bezeichnung": erm},
		{"satz": 1900, "bezeichnung": "regelsatz"},
	})
	if err != nil {
		panic(err)
	}
	return Inland{
		VMA24h: 2800, VMA8h: 1400,
		KuerzungFruehstueckPct: 2000, KuerzungHauptmahlzeitPct: 4000,
		Uebernachtung: 2000, KmKraftwagen: 30, KmAnderes: 20,
		SachbezugFruehstueck: sachF, SachbezugHauptmahlzeit: sachH,
		UeblicheMahlzeit: 6000, Kleinbetrag: 25000, BewirtungAbzug: 7000,
		AufbewahrungJahre: 8, UstSaetze: string(ust), Ersatz: Ersatzlaender(),
		Flug: "AT", Schiff: "LU", Quelle: quelle, GastroHinweis: gastro,
	}
}

// Ersatzlaender maps countries and territories that the BMF table does not
// list. "*" is the Luxembourg catch-all.
func Ersatzlaender() map[string]string {
	m := map[string]string{"*": "LU", "FM": "PH"}
	for _, iso := range []string{"AG", "DM", "GD", "GY", "KN", "LC", "VC", "SR"} {
		m[iso] = "TT"
	}
	for _, iso := range []string{"GF", "GP", "MQ", "RE", "YT", "PF", "NC", "PM", "BL", "MF", "WF", "TF"} {
		m[iso] = "FR"
	}
	for _, iso := range []string{"AW", "CW", "SX", "BQ"} {
		m[iso] = "NL"
	}
	for _, iso := range []string{"GL", "FO"} {
		m[iso] = "DK"
	}
	for _, iso := range []string{"GI", "BM", "KY", "VG", "FK", "MS", "TC", "SH", "AI", "IO", "PN"} {
		m[iso] = "GB"
	}
	for _, iso := range []string{"PR", "GU", "VI", "AS", "MP"} {
		m[iso] = "US"
	}
	m["MO"] = "CN"
	m["HK"] = "CN"
	m["AX"] = "FI"
	return m
}

// Satz resolves land (ISO) and satzort. Missing countries use Ersatzlaender, then Luxembourg.
func (y *Year) Satz(land, satzort string) (Hit, error) {
	if y == nil {
		return Hit{}, errors.New("no satztabelle")
	}
	land = strings.ToUpper(strings.TrimSpace(land))
	if land == "DE" {
		return Hit{
			Jahr: y.Jahr, LandISO: "DE", LandNameDE: "Deutschland", Weg: WegInland,
			VMA24h: y.Inland.VMA24h, VMA8h: y.Inland.VMA8h, Uebernachtung: y.Inland.Uebernachtung,
		}, nil
	}
	key := NormalizeSatzort(land, satzort)
	resolved, weg := y.resolveLand(land)
	if resolved == "" {
		return Hit{}, fmt.Errorf("no rate for %s", land)
	}
	if weg == WegLuxemburg || weg == WegErsatz {
		key = y.retarget(resolved, key)
	}
	row, exact := y.byLand[resolved][key]
	if exact && key != "" {
		return y.hit(row, wegOr(weg, WegExakt), land), nil
	}
	if exact && key == "" {
		return y.hit(row, wegOr(weg, WegExakt), land), nil
	}
	fallback, ok := y.byLand[resolved][""]
	if !ok {
		return Hit{}, fmt.Errorf("no default rate for %s", resolved)
	}
	used := WegUebriges
	if weg == WegErsatz || weg == WegLuxemburg {
		used = weg
	}
	return y.hit(fallback, used, land), nil
}

func wegOr(resolved, direct string) string {
	if resolved == WegErsatz || resolved == WegLuxemburg {
		return resolved
	}
	return direct
}

func (y *Year) hit(row Row, weg, requested string) Hit {
	h := Hit{
		Jahr: y.Jahr, LandISO: row.LandISO, LandNameDE: row.LandNameDE,
		Satzort: row.Satzort, OrtName: row.OrtName,
		VMA24h: row.VMA24h, VMA8h: row.VMA8h, Uebernachtung: row.Uebernachtung,
		Weg: weg,
	}
	if requested != row.LandISO && (weg == WegErsatz || weg == WegLuxemburg) {
		h.ErsatzVon = requested
	}
	return h
}

func (y *Year) resolveLand(land string) (string, string) {
	if len(y.byLand[land]) > 0 {
		return land, ""
	}
	if next := y.Inland.Ersatz[land]; next != "" && len(y.byLand[next]) > 0 {
		return next, WegErsatz
	}
	if len(y.byLand["LU"]) > 0 {
		return "LU", WegLuxemburg
	}
	return "", ""
}

func (y *Year) retarget(land, key string) string {
	if key == "" {
		return ""
	}
	if _, ok := y.byLand[land][key]; ok {
		return key
	}
	return ""
}

// NormalizeSatzort accepts "", "im Übrigen", a full key such as FR-PARIS, or a bare slug.
func NormalizeSatzort(land, satzort string) string {
	satzort = strings.TrimSpace(satzort)
	if satzort == "" || satzort == "im Übrigen" {
		return ""
	}
	satzort = strings.ToUpper(satzort)
	if len(satzort) > 3 && satzort[2] == '-' {
		return satzort
	}
	return land + "-" + satzort
}

// Index rebuilds the land map after rows were replaced or overridden.
func (y *Year) Index() {
	y.byLand = map[string]map[string]Row{}
	for _, row := range y.Rows {
		if _, ok := y.byLand[row.LandISO]; !ok {
			y.byLand[row.LandISO] = map[string]Row{}
		}
		y.byLand[row.LandISO][row.Satzort] = row
	}
}

// LineError is one CSV import failure. Zeile is 1-based and includes the header.
type LineError struct {
	Zeile int
	Feld  string
	Code  string
}

// ParseImport reads a research CSV plus a land_iso column.
func ParseImport(r io.Reader, jahr int) ([]Row, []LineError) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil {
		return nil, []LineError{{Zeile: 1, Feld: "kopf", Code: "kopf"}}
	}
	want := []string{"jahr", "land", "ort", "vma_24h_eur", "vma_an_abreise_8h_eur", "uebernachtung_ag_pauschal_eur", "land_iso"}
	if strings.Join(trimAll(header), ",") != strings.Join(want, ",") {
		return nil, []LineError{{Zeile: 1, Feld: "kopf", Code: "kopf"}}
	}
	var rows []Row
	var errs []LineError
	seen := map[string]int{}
	line := 1
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			errs = append(errs, LineError{Zeile: line, Feld: "zeile", Code: "zeile"})
			continue
		}
		if len(rec) != 7 {
			errs = append(errs, LineError{Zeile: line, Feld: "zeile", Code: "spalten"})
			continue
		}
		rec = trimAll(rec)
		rowYear, err := strconv.Atoi(rec[0])
		if err != nil || rowYear != jahr {
			errs = append(errs, LineError{Zeile: line, Feld: "jahr", Code: "jahr"})
		}
		iso := strings.ToUpper(rec[6])
		if len(iso) != 2 || iso[0] < 'A' || iso[0] > 'Z' || iso[1] < 'A' || iso[1] > 'Z' {
			errs = append(errs, LineError{Zeile: line, Feld: "land_iso", Code: "land_iso"})
		}
		vma24, e1 := eurosToCents(rec[3])
		vma8, e2 := eurosToCents(rec[4])
		ueb, e3 := eurosToCents(rec[5])
		if e1 != nil || e2 != nil || e3 != nil {
			errs = append(errs, LineError{Zeile: line, Feld: "betrag", Code: "betrag"})
		}
		if rec[1] == "" {
			errs = append(errs, LineError{Zeile: line, Feld: "land", Code: "land"})
		}
		row := Row{
			LandISO: iso, LandNameDE: rec[1], Satzort: SatzortKey(iso, rec[2]), OrtName: ortName(rec[2]),
			VMA24h: vma24, VMA8h: vma8, Uebernachtung: ueb,
		}
		id := iso + "|" + row.Satzort
		if prev, ok := seen[id]; ok {
			errs = append(errs, LineError{Zeile: line, Feld: "satzort", Code: "doppelt"})
			_ = prev
		}
		seen[id] = line
		rows = append(rows, row)
	}
	if len(rows) == 0 && len(errs) == 0 {
		errs = append(errs, LineError{Zeile: 1, Feld: "zeile", Code: "leer"})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return rows, nil
}

// InlandFields are the Satztabelle columns an Admin may override.
func InlandFields() []string {
	return []string{
		"vma_inland_24h", "vma_inland_8h", "kuerzung_fruehstueck_pct", "kuerzung_hauptmahlzeit_pct",
		"uebernachtung_inland_pauschale", "km_kraftwagen", "km_anderes_motorfahrzeug",
		"sachbezug_fruehstueck", "sachbezug_hauptmahlzeit", "uebliche_mahlzeit_grenze",
		"kleinbetragsgrenze", "bewirtung_abzug_pct", "aufbewahrung_jahre",
	}
}

// SetInlandField writes one override onto a copy of the domestic rates.
func SetInlandField(in *Inland, feld string, n int64) bool {
	switch feld {
	case "vma_inland_24h":
		in.VMA24h = n
	case "vma_inland_8h":
		in.VMA8h = n
	case "kuerzung_fruehstueck_pct":
		in.KuerzungFruehstueckPct = n
	case "kuerzung_hauptmahlzeit_pct":
		in.KuerzungHauptmahlzeitPct = n
	case "uebernachtung_inland_pauschale":
		in.Uebernachtung = n
	case "km_kraftwagen":
		in.KmKraftwagen = n
	case "km_anderes_motorfahrzeug":
		in.KmAnderes = n
	case "sachbezug_fruehstueck":
		in.SachbezugFruehstueck = n
	case "sachbezug_hauptmahlzeit":
		in.SachbezugHauptmahlzeit = n
	case "uebliche_mahlzeit_grenze":
		in.UeblicheMahlzeit = n
	case "kleinbetragsgrenze":
		in.Kleinbetrag = n
	case "bewirtung_abzug_pct":
		in.BewirtungAbzug = n
	case "aufbewahrung_jahre":
		in.AufbewahrungJahre = n
	default:
		return false
	}
	return true
}

// InlandField reads one domestic rate.
func InlandField(in Inland, feld string) (int64, bool) {
	cp := in
	if !SetInlandField(&cp, feld, 0) {
		return 0, false
	}
	switch feld {
	case "vma_inland_24h":
		return in.VMA24h, true
	case "vma_inland_8h":
		return in.VMA8h, true
	case "kuerzung_fruehstueck_pct":
		return in.KuerzungFruehstueckPct, true
	case "kuerzung_hauptmahlzeit_pct":
		return in.KuerzungHauptmahlzeitPct, true
	case "uebernachtung_inland_pauschale":
		return in.Uebernachtung, true
	case "km_kraftwagen":
		return in.KmKraftwagen, true
	case "km_anderes_motorfahrzeug":
		return in.KmAnderes, true
	case "sachbezug_fruehstueck":
		return in.SachbezugFruehstueck, true
	case "sachbezug_hauptmahlzeit":
		return in.SachbezugHauptmahlzeit, true
	case "uebliche_mahlzeit_grenze":
		return in.UeblicheMahlzeit, true
	case "kleinbetragsgrenze":
		return in.Kleinbetrag, true
	case "bewirtung_abzug_pct":
		return in.BewirtungAbzug, true
	case "aufbewahrung_jahre":
		return in.AufbewahrungJahre, true
	default:
		return 0, false
	}
}

func trimAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.TrimSpace(s)
	}
	return out
}
