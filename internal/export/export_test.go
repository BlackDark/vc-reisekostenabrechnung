package export

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestReproducible(t *testing.T) {
	typst := os.Getenv("TYPST_PATH")
	if typst == "" {
		typst = "/usr/local/bin/typst"
	}
	if _, err := os.Stat(typst); err != nil {
		t.Skip("typst not installed")
	}
	snap := demoSnapshot("de")
	jpg := mustJPEG()
	src := []Source{{
		Name: "2026-0042_s1.jpg", MIME: "image/jpeg", Bytes: jpg, Art: "jpeg",
		Nummer: "2026-0042", Seite: 1, Kopf: "Beleg 2026-0042",
	}}
	a, err := Render(typst, snap, src, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(typst, snap, src, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.JSON, b.JSON) || !bytes.Equal(a.PDF, b.PDF) || !bytes.Equal(a.ZIP, b.ZIP) {
		t.Fatalf("snapshot is not byte-stable pdf %d/%d zip %d/%d", len(a.PDF), len(b.PDF), len(a.ZIP), len(b.ZIP))
	}
}

func TestCSVGoldenAndSchema(t *testing.T) {
	snap := demoSnapshot("de")
	body, err := canonical(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON(body); err != nil {
		t.Fatal(err)
	}
	ausgaben, err := ausgabenCSV(snap)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ausgaben)
	if !strings.HasPrefix(text, "\uFEFF") {
		t.Fatal("missing BOM")
	}
	if !strings.Contains(text, "abrechnungsnummer;reise_nr;reise_anlass") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "RK-2026-007;1;Kundentermin;Alpha;a1;2026-0042;2026-10-01;reisenebenkosten") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "12,50;1;DE;19,00;10,50;2,00;12,50;ja") {
		t.Fatal(text)
	}
	tage, err := reisetageCSV(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tage), "2026-10-01;anreise;16,00;DE") {
		t.Fatal(string(tage))
	}
	fahrten, err := fahrtenCSV(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fahrten), "Wohnung;Bahnhof;;kraftwagen;12;ja;24;0,30;7,20") {
		t.Fatal(string(fahrten))
	}
	repo, err := os.ReadFile("../../api/schemas/abrechnung-export-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(repo), bytes.TrimSpace(schemaJSON)) {
		t.Fatal("schema copies differ")
	}
}

func TestPDFStructure(t *testing.T) {
	typst := os.Getenv("TYPST_PATH")
	if typst == "" {
		typst = "/usr/local/bin/typst"
	}
	if _, err := os.Stat(typst); err != nil {
		t.Skip("typst not installed")
	}
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo not installed")
	}
	set, err := sampleSet(typst)
	if err != nil {
		t.Fatal(err)
	}
	for name, doc := range set {
		if err := ValidateJSON(doc.JSON); err != nil {
			t.Fatalf("%s %v", name, err)
		}
		dir := t.TempDir()
		pdf := dir + "/a.pdf"
		if err := os.WriteFile(pdf, doc.PDF, 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := exec.Command("pdfinfo", pdf).CombinedOutput()
		if err != nil {
			t.Fatalf("%s pdfinfo %v %s", name, err, info)
		}
		if !bytes.Contains(info, []byte("Pages:")) {
			t.Fatalf("%s %s", name, info)
		}
		text, err := exec.Command("pdftotext", "-q", pdf, "-").CombinedOutput()
		if err != nil {
			t.Fatalf("%s text %v", name, err)
		}
		if name == "inland" && !bytes.Contains(text, []byte("112,40")) {
			t.Fatalf("sum missing %s", text)
		}
		if name == "inland" && !bytes.Contains(text, []byte("2026-0042")) {
			t.Fatalf("beleg missing %s", text)
		}
		if name == "inland" && !bytes.Contains(text, []byte("Umsatzsteuer")) {
			t.Fatalf("vat missing %s", text)
		}
		if name == "inland" && !bytes.Contains(text, []byte("Eigenbelege")) {
			t.Fatalf("eigenbeleg missing %s", text)
		}
		if name == "en" && !bytes.Contains(text, []byte("Substitute receipts")) {
			t.Fatalf("en layout %s", text)
		}
		if name == "xml" && !bytes.Contains(text, []byte("2026-0044.xml")) {
			t.Fatalf("xml placeholder %s", text)
		}
		if name == "pdf-beleg" || name == "xml" {
			list, err := exec.Command("pdfdetach", "-list", pdf).CombinedOutput()
			if err != nil {
				t.Fatalf("%s detach %v %s", name, err, list)
			}
			if !bytes.Contains(list, []byte("1 embedded")) && !bytes.Contains(list, []byte("embedded file")) {
				t.Fatalf("%s %s", name, list)
			}
		}
	}
}
