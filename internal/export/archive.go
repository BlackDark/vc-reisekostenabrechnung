package export

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type memFile struct {
	name string
	body []byte
}

func buildZIP(s Snapshot, pdf []byte, extra []memFile) ([]byte, error) {
	jsonBody, err := canonical(s)
	if err != nil {
		return nil, err
	}
	ausgaben, err := ausgabenCSV(s)
	if err != nil {
		return nil, err
	}
	tage, err := reisetageCSV(s)
	if err != nil {
		return nil, err
	}
	fahrten, err := fahrtenCSV(s)
	if err != nil {
		return nil, err
	}
	protokoll, err := protokollCSV(s)
	if err != nil {
		return nil, err
	}
	folder := s.Folder()
	files := []memFile{
		{folder + "/" + folder + ".pdf", pdf},
		{folder + "/abrechnung.json", jsonBody},
		{folder + "/ausgaben.csv", ausgaben},
		{folder + "/reisetage.csv", tage},
		{folder + "/fahrten.csv", fahrten},
		{folder + "/protokoll.csv", protokoll},
		{folder + "/LIESMICH.txt", []byte(liesmich(s))},
	}
	for _, beleg := range s.Belege {
		if strings.TrimSpace(beleg.Text) == "" || beleg.Nummer == "" {
			continue
		}
		files = append(files, memFile{folder + "/belegtexte/" + safeName(beleg.Nummer) + ".txt", []byte(beleg.Text)})
	}
	for _, f := range extra {
		files = append(files, memFile{folder + "/belege/" + safeName(f.name), f.body})
	}
	manifest := shaLines(files)
	files = append(files, memFile{folder + "/MANIFEST.sha256", []byte(manifest)})

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	when := time.Unix(s.Stamp(), 0).UTC()
	if when.IsZero() || s.Stamp() == 0 {
		when = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: when}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func shaLines(files []memFile) string {
	lines := make([]string, 0, len(files))
	for _, f := range files {
		sum := sha256.Sum256(f.body)
		rel := f.name
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			rel = rel[i+1:]
		}
		lines = append(lines, hex.EncodeToString(sum[:])+"  "+rel)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

func liesmich(s Snapshot) string {
	return fmt.Sprintf(`vc-reisekostenabrechnung %s
Abrechnung %s Version %d
CSV: UTF-8 mit BOM, Trennzeichen Semikolon, Dezimalkomma, Datum JJJJ-MM-TT.
Spaltennamen bleiben deutsch. Kostenarten bleiben die gespeicherten Schlüssel.
ausgaben.csv enthält eine Zeile je Steueranteil. Bewirtungsbeträge stehen nur in der ersten Zeile einer Ausgabe.
belege/ enthält Archivbelege (JPEG) und Originale (PDF, XML).
belegtexte/ enthält bestätigte Volltexte.
MANIFEST.sha256 ist das sha256sum-Format aller anderen Dateien.
`, s.AppVersion, s.Abrechnung.Nummer, s.Abrechnung.Version)
}

func safeName(name string) string {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.TrimSpace(name)
	if name == "" {
		return "datei"
	}
	return name
}
