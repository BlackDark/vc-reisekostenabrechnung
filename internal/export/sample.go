package export

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sample writes one PDF/A-3b for the container smoke test.
func Sample(typstPath, outPath string) error {
	set, err := sampleSet(typstPath)
	if err != nil {
		return err
	}
	doc := set["inland"]
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, doc.PDF, 0o644)
}

// Samples writes the veraPDF variants (Inland, Ausland, PDF receipt, XML, Bewirtung, en).
func Samples(typstPath, dir string) error {
	set, err := sampleSet(typstPath)
	if err != nil {
		return err
	}
	for name, doc := range set {
		sub := filepath.Join(dir, name)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(sub, "abrechnung.pdf"), doc.PDF, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(sub, "abrechnung.json"), doc.JSON, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func sampleSet(typst string) (map[string]Result, error) {
	jpg := mustJPEG()
	pdf, err := tinyPDF(typst)
	if err != nil {
		return nil, err
	}
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?><invoice><id>2026-0044</id><total>12.50</total></invoice>`)
	base := demoSnapshot("de")
	out := map[string]Result{}
	inland, err := Render(typst, base, []Source{{
		Name: "2026-0042_s1.jpg", MIME: "image/jpeg", Bytes: jpg, Art: "jpeg",
		Nummer: "2026-0042", Seite: 1, Kopf: "Beleg 2026-0042 · 12,50 € · 1/1",
	}}, false)
	if err != nil {
		return nil, fmt.Errorf("inland: %w", err)
	}
	out["inland"] = inland

	abroad := demoSnapshot("de")
	abroad.Reisen[0].Anlass = "Kundentermin Paris"
	abroad.Reisen[0].Ortswechsel = []Ort{{Ankunft: "2026-10-02T09:00:00", Verkehrsmittel: "bahn", LandISO: "FR", Ort: "Paris"}}
	abroad.Reisen[0].Reisetage[0].LandISO = "FR"
	abroad.Reisen[0].Reisetage[0].Satzort = "FR-PARIS"
	abroad.Abrechnung.Titel = "Reisekosten Ausland"
	got, err := Render(typst, abroad, nil, false)
	if err != nil {
		return nil, fmt.Errorf("ausland: %w", err)
	}
	out["ausland"] = got

	withPDF := demoSnapshot("de")
	withPDF.Belege = append(withPDF.Belege, Beleg{
		ID: "pdf", Nummer: "2026-0043", Typ: "pdf", Status: "bestaetigt", SHA256: strings.Repeat("ab", 32),
		PipelineVersion: "m7", Dateien: []Datei{{Variante: "original", Seite: 1, MIME: "application/pdf", Name: "2026-0043.pdf"}},
	})
	got, err = Render(typst, withPDF, []Source{{
		Name: "2026-0043.pdf", MIME: "application/pdf", Bytes: pdf, Art: "pdf",
		Nummer: "2026-0043", Kopf: "Beleg 2026-0043",
	}}, false)
	if err != nil {
		return nil, fmt.Errorf("pdf: %w", err)
	}
	out["pdf-beleg"] = got

	withXML := demoSnapshot("de")
	withXML.Belege = append(withXML.Belege, Beleg{
		ID: "xml", Nummer: "2026-0044", Typ: "e_rechnung_xml", Status: "bestaetigt", SHA256: strings.Repeat("cd", 32),
		PipelineVersion: "m7", Dateien: []Datei{{Variante: "original", Seite: 1, MIME: "application/xml", Name: "2026-0044.xml"}},
	})
	got, err = Render(typst, withXML, []Source{{
		Name: "2026-0044.xml", MIME: "application/xml", Bytes: xml, Art: "xml",
		Nummer: "2026-0044", Kopf: "Beleg 2026-0044",
		Text: "Strukturierte E-Rechnung, Original eingebettet: 2026-0044.xml",
	}}, false)
	if err != nil {
		return nil, fmt.Errorf("xml: %w", err)
	}
	out["xml"] = got

	host := demoSnapshot("de")
	host.Reisen[0].Ausgaben[0].Kostenart = "bewirtung"
	host.Reisen[0].Ausgaben[0].Bewirtung = &Bewirt{
		Anlass: "Projektgespräch", Ort: "Berlin", Teilnehmer: "Ada, Kim", Bewirtender: "Ada", Bestaetigt: "2026-10-02T18:00:00Z",
	}
	host.Reisen[0].Ausgaben[0].BewirtungAbziehbarCent = 875
	host.Reisen[0].Ausgaben[0].BewirtungNichtCent = 375
	host.Abrechnung.Summen.BewirtungCent = 1250
	got, err = Render(typst, host, nil, false)
	if err != nil {
		return nil, fmt.Errorf("bewirtung: %w", err)
	}
	out["bewirtung"] = got

	en := demoSnapshot("en")
	en.Abrechnung.Titel = "Travel expenses October 2026"
	got, err = Render(typst, en, []Source{{
		Name: "2026-0042_s1.jpg", MIME: "image/jpeg", Bytes: jpg, Art: "jpeg",
		Nummer: "2026-0042", Kopf: "Receipt 2026-0042",
	}}, false)
	if err != nil {
		return nil, fmt.Errorf("en: %w", err)
	}
	out["en"] = got
	return out, nil
}

func demoSnapshot(lang string) Snapshot {
	return Snapshot{
		SchemaVersion: SchemaVersion, AppVersion: "0.7.0", ErzeugtAm: "2026-10-09T12:00:00Z", ExportSprache: lang,
		Abrechnung: Kopf{
			Nummer: "RK-2026-007", Version: 1, ZeitraumArt: "monat", Von: "2026-10-01", Bis: "2026-10-31",
			Titel: "Reisekosten Oktober 2026", Status: "eingereicht", EingereichtAm: "2026-10-09T12:00:00Z",
			Summen: Summen{
				VerpflegungCent: 28740, DavonPauschalenCent: 28740, ReisenebenkostenCent: 12500,
			},
			ErstattungCent: 41240,
			Vorschuesse:    []Vorschuss{{ID: "v1", Datum: "2026-10-01", Cent: 30000, Notiz: "Abschlag"}},
			VorschussCent:  30000, AuszahlungCent: 11240,
		},
		Nutzer:      Person{Name: "Ada Lovelace", Personalnummer: "17"},
		Arbeitgeber: Arbeitgeber{Name: "ACME GmbH", Anschrift: "Weg 1\n10115 Berlin"},
		Satztabellen: []SatzJahr{{
			Jahr: 2026, Quelle: "BMF", Werte: map[string]int64{"vma_24h": 2800, "vma_8h": 1400},
		}},
		Reisen: []Reise{{
			Nr: 1, ID: "r1", Anlass: "Kundentermin", Projekt: "Alpha",
			Beginn: "2026-10-01T08:00:00", BeginnZone: "Europe/Berlin",
			Ende: "2026-10-02T18:00:00", EndeZone: "Europe/Berlin", SummeCent: 41240,
			Ortswechsel: []Ort{{Ankunft: "2026-10-01T10:00:00", Verkehrsmittel: "bahn", LandISO: "DE", Ort: "Berlin"}},
			Reisetage: []Tag{{
				Datum: "2026-10-01", Tagesart: "anreise", AbwesenheitMin: 960, LandISO: "DE",
				Satz24hCent: 2800, PauschaleCent: 1400, VerpflegungCent: 1400, Unterkunft: "gestellt",
				RegelIDs: []string{"TAG-ANREISE"},
			}},
			Fahrten: []Fahrt{{
				Datum: "2026-10-01", Start: "Wohnung", Ziel: "Bahnhof", Fahrzeugart: "kraftwagen",
				Km: 12, HinUndZurueck: true, KmGesamt: 24, SatzCent: 30, BetragCent: 720,
			}},
			Ausgaben: []Ausgabe{{
				ID: "a1", Belegnummern: "2026-0042", Datum: "2026-10-01", Kostenart: "reisenebenkosten",
				Leistender: "Café", Rechnungsart: "kleinbetragsrechnung", Waehrung: "EUR",
				BetragCent: 1250, BetragEURCent: 1250,
				Anteile: []Anteil{{Nr: 1, Land: "DE", Satz: 1900, NettoCent: 1050, UstCent: 200, BruttoCent: 1250, Vorsteuer: true}},
			}},
		}},
		Belege: []Beleg{{
			ID: "b1", Nummer: "2026-0042", Typ: "foto", Status: "bestaetigt", SHA256: strings.Repeat("aa", 32),
			PipelineVersion: "m7", Dateien: []Datei{{Variante: "export_jpeg", Seite: 1, MIME: "image/jpeg", SHA256: strings.Repeat("bb", 32), Name: "2026-0042_s1.jpg"}},
		}},
		WarnungenQuittiert: []Quittung{},
		Protokoll:          []Ereignis{{Zeitpunkt: "2026-10-09T12:00:00Z", AkteurArt: "nutzer", Aktion: "abrechnung.eingereicht", ObjektTyp: "abrechnung", ObjektID: "a"}},
	}
}

func mustJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 + x*4), G: 80, B: uint8(40 + y*6), A: 255})
		}
	}
	var out bytes.Buffer
	_ = jpeg.Encode(&out, img, &jpeg.Options{Quality: 70})
	return out.Bytes()
}

func tinyPDF(typst string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "rk-beleg-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	body := "#set page(paper: \"a5\", margin: 1cm)\n= Beleg 2026-0043\n12,50 EUR\n"
	if err := os.WriteFile(filepath.Join(dir, "b.typ"), []byte(body), 0o644); err != nil {
		return nil, err
	}
	cmd := exec.Command(typst, "compile", "--ignore-system-fonts", "b.typ", "b.pdf")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sample pdf: %w: %s", err, out)
	}
	return os.ReadFile(filepath.Join(dir, "b.pdf"))
}
