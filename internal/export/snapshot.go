// Package export builds the Abrechnung snapshot, CSV files, ZIP archive and PDF/A-3b.
package export

import (
	"fmt"
	"strings"
	"time"
)

const SchemaVersion = "1"

// Snapshot is abrechnung.json (SPEC 7.2).
type Snapshot struct {
	SchemaVersion      string      `json:"schema_version"`
	AppVersion         string      `json:"app_version"`
	ErzeugtAm          string      `json:"erzeugt_am"`
	ExportSprache      string      `json:"export_sprache"`
	Abrechnung         Kopf        `json:"abrechnung"`
	Nutzer             Person      `json:"nutzer"`
	Arbeitgeber        Arbeitgeber `json:"arbeitgeber"`
	Satztabellen       []SatzJahr  `json:"satztabellen"`
	Reisen             []Reise     `json:"reisen"`
	Belege             []Beleg     `json:"belege"`
	WarnungenQuittiert []Quittung  `json:"warnungen_quittiert"`
	Protokoll          []Ereignis  `json:"protokoll"`
}

// Kopf is the cover of one export version.
type Kopf struct {
	Nummer         string      `json:"nummer"`
	Version        int         `json:"version"`
	ZeitraumArt    string      `json:"zeitraum_art"`
	Von            string      `json:"von"`
	Bis            string      `json:"bis"`
	Titel          string      `json:"titel"`
	Status         string      `json:"status"`
	EingereichtAm  string      `json:"eingereicht_am,omitempty"`
	Summen         Summen      `json:"summen"`
	ErstattungCent int64       `json:"erstattung_cent"`
	Vorschuesse    []Vorschuss `json:"vorschuesse"`
	VorschussCent  int64       `json:"vorschuss_cent"`
	AuszahlungCent int64       `json:"auszahlung_cent"`
}

// Summen are the cost-type totals in euro cents.
type Summen struct {
	FahrtkostenCent              int64 `json:"fahrtkosten_cent"`
	DavonKilometerCent           int64 `json:"davon_kilometer_cent"`
	VerpflegungCent              int64 `json:"verpflegung_cent"`
	DavonPauschalenCent          int64 `json:"davon_pauschalen_cent"`
	UebernachtungCent            int64 `json:"uebernachtung_cent"`
	DavonUebernachtungPauschCent int64 `json:"davon_uebernachtung_pauschale_cent"`
	ReisenebenkostenCent         int64 `json:"reisenebenkosten_cent"`
	BewirtungCent                int64 `json:"bewirtung_cent"`
}

// Vorschuss is one settled advance.
type Vorschuss struct {
	ID    string `json:"id"`
	Datum string `json:"datum"`
	Cent  int64  `json:"betrag_cent"`
	Notiz string `json:"notiz,omitempty"`
}

// Person is the traveller.
type Person struct {
	Name           string `json:"name"`
	Personalnummer string `json:"personalnummer,omitempty"`
}

// Arbeitgeber is the paying employer.
type Arbeitgeber struct {
	Name      string `json:"name"`
	Anschrift string `json:"anschrift"`
}

// SatzJahr is the rate snapshot used at submit time (I11).
type SatzJahr struct {
	Jahr    int              `json:"jahr"`
	Quelle  string           `json:"quelle"`
	Werte   map[string]int64 `json:"werte"`
	Laender []SatzLand       `json:"laender,omitempty"`
}

// SatzLand is one foreign rate that a trip day used.
type SatzLand struct {
	LandISO       string `json:"land_iso"`
	Satzort       string `json:"satzort"`
	VMA24hCent    int64  `json:"vma_24h_cent"`
	VMA8hCent     int64  `json:"vma_8h_cent"`
	Uebernachtung int64  `json:"uebernachtung_cent"`
}

// Reise is one trip inside the export.
type Reise struct {
	Nr          int       `json:"nr"`
	ID          string    `json:"id"`
	Anlass      string    `json:"anlass"`
	Projekt     string    `json:"projekt,omitempty"`
	Beginn      string    `json:"beginn"`
	BeginnZone  string    `json:"beginn_zone"`
	Ende        string    `json:"ende"`
	EndeZone    string    `json:"ende_zone"`
	SummeCent   int64     `json:"summe_cent"`
	Ortswechsel []Ort     `json:"ortswechsel"`
	Reisetage   []Tag     `json:"reisetage"`
	Fahrten     []Fahrt   `json:"fahrten"`
	Ausgaben    []Ausgabe `json:"ausgaben"`
}

// Ort is one Ortswechsel.
type Ort struct {
	Ankunft        string `json:"ankunft"`
	Verkehrsmittel string `json:"verkehrsmittel"`
	LandISO        string `json:"land_iso"`
	Ort            string `json:"ort,omitempty"`
}

// Tag is one Reisetag row.
type Tag struct {
	Datum              string   `json:"datum"`
	Tagesart           string   `json:"tagesart"`
	AbwesenheitMin     int      `json:"abwesenheit_min"`
	LandISO            string   `json:"land_iso"`
	Satzort            string   `json:"satzort,omitempty"`
	Satz24hCent        int64    `json:"satz_24h_cent"`
	PauschaleCent      int64    `json:"pauschale_cent"`
	KuerzungFruehCent  int64    `json:"kuerzung_fruehstueck_cent"`
	KuerzungMittagCent int64    `json:"kuerzung_mittag_cent"`
	KuerzungAbendCent  int64    `json:"kuerzung_abend_cent"`
	ZuzahlungenCent    int64    `json:"zuzahlungen_cent"`
	VerpflegungCent    int64    `json:"verpflegung_cent"`
	Unterkunft         string   `json:"unterkunft"`
	UebernachtungCent  int64    `json:"uebernachtung_cent"`
	RegelIDs           []string `json:"regel_ids"`
	Hinweise           []string `json:"hinweise"`
}

// Fahrt is one mileage line.
type Fahrt struct {
	Datum         string `json:"datum"`
	Start         string `json:"start"`
	Ziel          string `json:"ziel"`
	Zweck         string `json:"zweck,omitempty"`
	Fahrzeugart   string `json:"fahrzeugart"`
	Km            int64  `json:"km"`
	HinUndZurueck bool   `json:"hin_und_zurueck"`
	KmGesamt      int64  `json:"km_gesamt"`
	SatzCent      int64  `json:"satz_cent_km"`
	BetragCent    int64  `json:"betrag_cent"`
}

// Ausgabe is one expense with its VAT shares.
type Ausgabe struct {
	ID                     string   `json:"id"`
	Belegnummern           string   `json:"belegnummern,omitempty"`
	Datum                  string   `json:"datum"`
	Kostenart              string   `json:"kostenart"`
	Verkehrsmittel         string   `json:"verkehrsmittel,omitempty"`
	Leistender             string   `json:"leistender,omitempty"`
	Beschreibung           string   `json:"beschreibung,omitempty"`
	Rechnungsart           string   `json:"rechnungsart"`
	RechnungAufArbeitgeber bool     `json:"rechnung_auf_arbeitgeber"`
	Waehrung               string   `json:"waehrung"`
	BetragCent             int64    `json:"betrag_cent"`
	Kurs                   string   `json:"kurs,omitempty"`
	KursQuelle             string   `json:"kurs_quelle,omitempty"`
	KursDatum              string   `json:"kurs_datum,omitempty"`
	BetragEURCent          int64    `json:"betrag_eur_cent"`
	Anteile                []Anteil `json:"steueranteile"`
	BewirtungAbziehbarCent int64    `json:"bewirtung_abziehbar_cent,omitempty"`
	BewirtungNichtCent     int64    `json:"bewirtung_nicht_abziehbar_cent,omitempty"`
	Bewirtung              *Bewirt  `json:"bewirtung,omitempty"`
}

// Anteil is one VAT share.
type Anteil struct {
	Nr         int    `json:"anteil_nr"`
	Land       string `json:"ust_land"`
	Satz       int64  `json:"ust_satz"`
	NettoCent  int64  `json:"netto_eur_cent"`
	UstCent    int64  `json:"ust_eur_cent"`
	BruttoCent int64  `json:"brutto_eur_cent"`
	Vorsteuer  bool   `json:"vorsteuerfaehig"`
}

// Bewirt is the entertainment record printed on the export.
type Bewirt struct {
	Anlass      string `json:"anlass"`
	Ort         string `json:"ort,omitempty"`
	Teilnehmer  string `json:"teilnehmer,omitempty"`
	Bewirtender string `json:"bewirtender,omitempty"`
	Bestaetigt  string `json:"bestaetigt_am,omitempty"`
}

// Beleg is one receipt in the archive.
type Beleg struct {
	ID              string  `json:"id"`
	Nummer          string  `json:"nummer"`
	Typ             string  `json:"typ"`
	Status          string  `json:"status"`
	SHA256          string  `json:"sha256"`
	PipelineVersion string  `json:"pipeline_version"`
	BestaetigtAm    string  `json:"bestaetigt_am,omitempty"`
	Dateien         []Datei `json:"dateien"`
	Text            string  `json:"text,omitempty"`
}

// Datei is one file of a Beleg. Name is the path inside the ZIP.
type Datei struct {
	Variante string `json:"variante"`
	Seite    int    `json:"seite"`
	MIME     string `json:"mime"`
	SHA256   string `json:"sha256"`
	Name     string `json:"name"`
}

// Quittung is one acknowledged warning.
type Quittung struct {
	Code     string `json:"code"`
	ObjektID string `json:"objekt_id"`
}

// Ereignis is one audit row in the protocol.
type Ereignis struct {
	Zeitpunkt string `json:"zeitpunkt"`
	AkteurArt string `json:"akteur_art"`
	Aktion    string `json:"aktion"`
	ObjektTyp string `json:"objekt_typ"`
	ObjektID  string `json:"objekt_id"`
	Grund     string `json:"grund,omitempty"`
}

// Folder is the ZIP directory name, for example RK-2026-007_v1.
func (s Snapshot) Folder() string {
	n := s.Abrechnung.Nummer
	if n == "" {
		n = "ENTWURF"
	}
	n = strings.NewReplacer("/", "-", "\\", "-", " ", "_").Replace(n)
	v := s.Abrechnung.Version
	if v < 1 {
		v = 1
	}
	return fmt.Sprintf("%s_v%d", n, v)
}

// Stamp returns a Unix time for a reproducible PDF creation timestamp.
func (s Snapshot) Stamp() int64 {
	t, err := time.Parse(time.RFC3339, s.ErzeugtAm)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func euroCSV(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d,%02d", sign, cents/100, cents%100)
}

func euroLabel(cents int64) string {
	return euroCSV(cents) + " €"
}

func jaNein(v bool) string {
	if v {
		return "ja"
	}
	return "nein"
}

func join(parts []string) string {
	return strings.Join(parts, "|")
}

func hoursCSV(minutes int) string {
	sign := ""
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}
	return fmt.Sprintf("%s%d,%02d", sign, minutes/60, (minutes%60)*100/60)
}

func commaDecimal(s string) string {
	return strings.ReplaceAll(s, ".", ",")
}
