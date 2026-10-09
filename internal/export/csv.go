package export

import (
	"bytes"
	"encoding/csv"
	"strconv"
)

func writeCSV(header []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func ausgabenCSV(s Snapshot) ([]byte, error) {
	header := stringsCSV(`abrechnungsnummer;reise_nr;reise_anlass;projekt;ausgabe_id;belegnummern;datum;kostenart;verkehrsmittel;leistender;beschreibung;rechnungsart;rechnung_auf_arbeitgeber;waehrung;betrag_beleg;kurs;kurs_quelle;kurs_datum;betrag_eur;anteil_nr;ust_land;ust_satz;netto_eur;ust_eur;brutto_eur;vorsteuerfaehig;bewirtung_abziehbar_eur;bewirtung_nicht_abziehbar_eur`)
	var rows [][]string
	for _, reise := range s.Reisen {
		for _, a := range reise.Ausgaben {
			anteile := a.Anteile
			if len(anteile) == 0 {
				anteile = []Anteil{{}}
			}
			for i, sh := range anteile {
				abz, nicht := "0,00", "0,00"
				if i == 0 {
					abz = euroCSV(a.BewirtungAbziehbarCent)
					nicht = euroCSV(a.BewirtungNichtCent)
				}
				nr := sh.Nr
				if nr == 0 {
					nr = i + 1
				}
				rows = append(rows, []string{
					s.Abrechnung.Nummer, strconv.Itoa(reise.Nr), reise.Anlass, reise.Projekt, a.ID, a.Belegnummern,
					a.Datum, a.Kostenart, a.Verkehrsmittel, a.Leistender, a.Beschreibung, a.Rechnungsart,
					jaNein(a.RechnungAufArbeitgeber), a.Waehrung, euroCSV(a.BetragCent), commaDecimal(a.Kurs),
					a.KursQuelle, a.KursDatum, euroCSV(a.BetragEURCent), strconv.Itoa(nr), sh.Land,
					euroCSV(sh.Satz), euroCSV(sh.NettoCent), euroCSV(sh.UstCent), euroCSV(sh.BruttoCent),
					jaNein(sh.Vorsteuer), abz, nicht,
				})
			}
		}
	}
	return writeCSV(header, rows)
}

func reisetageCSV(s Snapshot) ([]byte, error) {
	header := stringsCSV(`abrechnungsnummer;reise_nr;datum;tagesart;abwesenheit_std;land_iso;satzort;satz_24h_eur;pauschale_eur;kuerzung_fruehstueck_eur;kuerzung_mittag_eur;kuerzung_abend_eur;zuzahlungen_eur;verpflegung_eur;unterkunft;uebernachtungspauschale_eur;regel_ids;hinweise`)
	var rows [][]string
	for _, reise := range s.Reisen {
		for _, day := range reise.Reisetage {
			rows = append(rows, []string{
				s.Abrechnung.Nummer, strconv.Itoa(reise.Nr), day.Datum, day.Tagesart, hoursCSV(day.AbwesenheitMin),
				day.LandISO, day.Satzort, euroCSV(day.Satz24hCent), euroCSV(day.PauschaleCent),
				euroCSV(day.KuerzungFruehCent), euroCSV(day.KuerzungMittagCent), euroCSV(day.KuerzungAbendCent),
				euroCSV(day.ZuzahlungenCent), euroCSV(day.VerpflegungCent), day.Unterkunft,
				euroCSV(day.UebernachtungCent), join(day.RegelIDs), join(day.Hinweise),
			})
		}
	}
	return writeCSV(header, rows)
}

func fahrtenCSV(s Snapshot) ([]byte, error) {
	header := stringsCSV(`abrechnungsnummer;reise_nr;datum;start;ziel;zweck;fahrzeugart;km;hin_und_zurueck;km_gesamt;satz_eur_km;betrag_eur`)
	var rows [][]string
	for _, reise := range s.Reisen {
		for _, f := range reise.Fahrten {
			rows = append(rows, []string{
				s.Abrechnung.Nummer, strconv.Itoa(reise.Nr), f.Datum, f.Start, f.Ziel, f.Zweck, f.Fahrzeugart,
				strconv.FormatInt(f.Km, 10), jaNein(f.HinUndZurueck), strconv.FormatInt(f.KmGesamt, 10),
				euroCSV(f.SatzCent), euroCSV(f.BetragCent),
			})
		}
	}
	return writeCSV(header, rows)
}

func protokollCSV(s Snapshot) ([]byte, error) {
	header := stringsCSV(`zeitpunkt;akteur_art;aktion;objekt_typ;objekt_id;grund`)
	rows := make([][]string, 0, len(s.Protokoll))
	for _, ev := range s.Protokoll {
		rows = append(rows, []string{ev.Zeitpunkt, ev.AkteurArt, ev.Aktion, ev.ObjektTyp, ev.ObjektID, ev.Grund})
	}
	return writeCSV(header, rows)
}

func stringsCSV(header string) []string {
	return splitSemi(header)
}

func splitSemi(s string) []string {
	out := make([]string, 0, 16)
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ';' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
