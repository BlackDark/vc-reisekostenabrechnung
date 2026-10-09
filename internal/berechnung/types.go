package berechnung

import "github.com/BlackDark/vc-reisekostenabrechnung/internal/satz"

// Zeitpunkt is a civil clock time in an IANA zone. Lokal is "2006-01-02T15:04:05".
type Zeitpunkt struct {
	Lokal string
	Zone  string
}

// Ortswechsel is one ordered leg. The return home is the trip's Ende, not a leg.
type Ortswechsel struct {
	Abfahrt                         *Zeitpunkt
	Ankunft                         Zeitpunkt
	Verkehrsmittel                  string
	LandISO                         string
	Satzort                         string
	Ort                             string
	TaetigkeitsstaetteID            string
	Taetigkeit                      bool
	ZwischenlandungMitUebernachtung bool
}

// TagEingabe is the stored Reisetag input. Empty Unterkunft means keine.
type TagEingabe struct {
	LandManuell               string
	SatzortManuell            string
	Begruendung               string
	Fruehstueck               bool
	Mittag                    bool
	Abend                     bool
	ZuzahlungFruehstueck      int64
	ZuzahlungMittag           int64
	ZuzahlungAbend            int64
	Unterkunft                string
	VerpflegungAusgeschlossen bool
}

// SteuerInput is one VAT share. Netto and Steuer are filled from Brutto when nil.
type SteuerInput struct {
	Satz       int64
	Steuerland string
	Brutto     int64
	Netto      *int64
	Steuer     *int64
}

// Bewirtung is the entertainment record (SPEC 4.12). Nil skips B03 and W06.
type Bewirtung struct {
	Anlass      string
	Teilnehmer  int
	Ort         string
	Bewirtender string
	Bestaetigt  bool
}

// Ausgabe is a receipt line the calculator understands.
type Ausgabe struct {
	ID                     string
	Kostenart              string
	Datum                  string
	Leistender             string
	Waehrung               string
	Betrag                 int64
	Kurs                   string
	KursQuelle             string
	KursDatum              string
	BetragEUR              int64
	Rechnungsart           string
	RechnungAufArbeitgeber bool
	Empfaenger             string
	Naechte                []string
	FruehstueckEnthalten   bool
	Mahlzeit               string
	Verkehrsmittel         string
	Anteile                []SteuerInput
	Bewirtung              *Bewirtung
	TSE                    bool
	BelegAnzahl            int
	BelegOffen             bool
	Eigenbeleg             bool
	PruefeBeleg            bool
	Monatskurs             string
}

// Fahrt is a private-vehicle mileage line.
type Fahrt struct {
	ID            string
	Datum         string
	Fahrzeugart   string
	Km            int64
	HinUndZurueck bool
}

// Reise is one trip of a single Nutzer. Status empty means offen.
type Reise struct {
	ID              string
	Anlass          string
	ArbeitgeberID   string
	ArbeitgeberName string
	Konstellation   string
	Beginn          Zeitpunkt
	Ende            Zeitpunkt
	Status          string
	Ortswechsel     []Ortswechsel
	Tage            map[string]TagEingabe
	Ausgaben        []Ausgabe
	Fahrten         []Fahrt
	Snapshot        map[string]int64
}

// Kurs is one FX rate: foreign-currency units per 1 EUR.
type Kurs struct {
	Waehrung string
	Datum    string
	Kurs     string
}

// Eingabe is everything Berechne needs. Jahre must be aktiv for every year used.
type Eingabe struct {
	Reisen      []Reise
	Jahre       map[int]*satz.Year
	Kurse       []Kurs
	Vorschuesse []int64
}

// Kuerzung is one meal reduction on the day that received the Pauschale.
type Kuerzung struct {
	Art    string
	Betrag int64
	Regel  string
}

// Hinweis is H-SACHBEZUG or H-TAG-VERRECHNET.
type Hinweis struct {
	Code    string
	Betrag  int64
	ReiseID string
	Anlass  string
	Datum   string
}

// TagErgebnis is one Reisetag after 4.4–4.8 and 4.14.
type TagErgebnis struct {
	ReiseID        string
	Datum          string
	Tagesart       string
	LandISO        string
	Satzort        string
	LandRegel      string
	AbwesenheitMin int
	Pauschale      int64
	Kuerzungen     []Kuerzung
	Ergebnis       int64
	Uebernachtung  int64
	RegelIDs       []string
	Hinweise       []Hinweis
	Warnungen      []string
}

// SteuerErgebnis is one share after 4.11.
type SteuerErgebnis struct {
	Satz       int64
	Steuerland string
	Netto      int64
	Steuer     int64
	Brutto     int64
	NettoEUR   int64
	SteuerEUR  int64
	BruttoEUR  int64
	Vorsteuer  bool
}

// AusgabeErgebnis is one Ausgabe after currency and tax.
type AusgabeErgebnis struct {
	ID         string
	Kostenart  string
	BetragEUR  int64
	Kurs       string
	KursDatum  string
	Monatskurs string
	Anteile    []SteuerErgebnis
	Warnungen  []string
}

// ReiseErgebnis is the per-trip rollup.
type ReiseErgebnis struct {
	ID                      string
	Tage                    []TagErgebnis
	Ausgaben                []AusgabeErgebnis
	Fahrtkosten             int64
	Verpflegung             int64
	Uebernachtung           int64
	Reisenebenkosten        int64
	Bewirtung               int64
	BewirtungB              int64
	BewirtungAbziehbar      int64
	BewirtungNichtAbziehbar int64
	Vorsteuer               int64
	Summe                   int64
	Warnungen               []string
	Blocker                 []string
}

// Ergebnis is the whole Nutzer preview, including 4.15 when Vorschüsse are set.
type Ergebnis struct {
	Reisen              []ReiseErgebnis
	Erstattung          int64
	Auszahlung          int64
	VorschussVerrechnet int64
}

// Vorschlag is an automatic meal tick (4.7).
type Vorschlag struct {
	Datum    string
	Mahlzeit string
	Quelle   string
}

// FristWarnung is W03 for one visit day.
type FristWarnung struct {
	Staette   string
	Datum     string
	Fristende string
	Code      string
}

// Visit is one day a Tätigkeitsstätte was attended.
type Visit struct {
	Staette string
	Datum   string
}
