// Package fx resolves ECB daily reference rates and BMF monthly VAT rates.
// Both sources are cached by the caller. An embedded fixture is the offline fallback.
package fx

import (
	"context"
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//go:embed ecb.csv bmf.csv
var files embed.FS

// Rate is one daily ECB rate: foreign-currency units per 1 EUR.
type Rate struct {
	Waehrung string
	Datum    string
	Kurs     string
}

// Monat is one BMF monthly VAT conversion rate.
type Monat struct {
	Jahr     int
	Monat    int
	Waehrung string
	Kurs     string
}

// Lookback returns the rate on datum, or the latest published rate at most 7 days earlier.
func Lookback(rates []Rate, waehrung, datum string) (kurs, used string, ok bool) {
	want, err := time.Parse("2006-01-02", datum)
	if err != nil {
		return "", "", false
	}
	bestGap := 8
	var best Rate
	for _, k := range rates {
		if k.Waehrung != waehrung || k.Kurs == "" {
			continue
		}
		kd, err := time.Parse("2006-01-02", k.Datum)
		if err != nil || kd.After(want) {
			continue
		}
		gap := int(want.Sub(kd).Hours() / 24)
		if gap <= 7 && gap < bestGap {
			best = k
			bestGap = gap
		}
	}
	if best.Kurs == "" {
		return "", "", false
	}
	return best.Kurs, best.Datum, true
}

// EmbeddedECB is the offline ECB fixture (G16: USD 2026-09-11 = 1.1650).
func EmbeddedECB() []Rate {
	rates, err := ParseECB(mustRead("ecb.csv"))
	if err != nil {
		return nil
	}
	return rates
}

// EmbeddedBMF is the offline monthly VAT-rate fixture.
func EmbeddedBMF() []Monat {
	rows, err := ParseBMF(mustRead("bmf.csv"))
	if err != nil {
		return nil
	}
	return rows
}

// BMF returns the embedded monthly rate for a currency.
func BMF(waehrung string, jahr, monat int) (string, bool) {
	for _, row := range EmbeddedBMF() {
		if row.Waehrung == waehrung && row.Jahr == jahr && row.Monat == monat {
			return row.Kurs, true
		}
	}
	return "", false
}

func mustRead(name string) io.Reader {
	b, err := files.ReadFile(name)
	if err != nil {
		return strings.NewReader("")
	}
	return strings.NewReader(string(b))
}

// ParseECB reads an ECB SDMX csvdata file (TIME_PERIOD, OBS_VALUE, optional CURRENCY).
func ParseECB(r io.Reader) ([]Rate, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	col := indexOf(rows[0])
	timeCol, okT := col["TIME_PERIOD"]
	obsCol, okO := col["OBS_VALUE"]
	if !okT || !okO {
		return nil, fmt.Errorf("ecb csv")
	}
	ccyCol, hasCcy := col["CURRENCY"]
	var out []Rate
	for _, row := range rows[1:] {
		if timeCol >= len(row) || obsCol >= len(row) {
			continue
		}
		kurs := strings.TrimSpace(row[obsCol])
		if kurs == "" {
			continue
		}
		ccy := "USD"
		if hasCcy && ccyCol < len(row) && strings.TrimSpace(row[ccyCol]) != "" {
			ccy = strings.ToUpper(strings.TrimSpace(row[ccyCol]))
		}
		out = append(out, Rate{Waehrung: ccy, Datum: strings.TrimSpace(row[timeCol]), Kurs: kurs})
	}
	return out, nil
}

// ParseBMF reads jahr,monat,waehrung,kurs.
func ParseBMF(r io.Reader) ([]Monat, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	col := indexOf(rows[0])
	need := []string{"jahr", "monat", "waehrung", "kurs"}
	for _, n := range need {
		if _, ok := col[n]; !ok {
			return nil, fmt.Errorf("bmf csv")
		}
	}
	var out []Monat
	for _, row := range rows[1:] {
		jahr, err1 := strconv.Atoi(field(row, col["jahr"]))
		monat, err2 := strconv.Atoi(field(row, col["monat"]))
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, Monat{
			Jahr: jahr, Monat: monat, Waehrung: strings.ToUpper(field(row, col["waehrung"])), Kurs: field(row, col["kurs"]),
		})
	}
	return out, nil
}

func indexOf(header []string) map[string]int {
	out := map[string]int{}
	for i, h := range header {
		out[strings.ToLower(strings.TrimSpace(h))] = i
		out[strings.TrimSpace(h)] = i
	}
	return out
}

func field(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// ValidCurrency is a three-letter ISO code.
func ValidCurrency(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// FetchECB loads daily rates from the ECB Data API. The currency must already be validated.
func FetchECB(ctx context.Context, client *http.Client, waehrung, start, end string) ([]Rate, error) {
	if !ValidCurrency(waehrung) {
		return nil, fmt.Errorf("waehrung")
	}
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}
	u := url.URL{
		Scheme: "https",
		Host:   "data-api.ecb.europa.eu",
		Path:   "/service/data/EXR/D." + waehrung + ".EUR.SP00.A",
	}
	q := u.Query()
	q.Set("startPeriod", start)
	q.Set("endPeriod", end)
	q.Set("format", "csvdata")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ecb %d", res.StatusCode)
	}
	rates, err := ParseECB(res.Body)
	if err != nil {
		return nil, err
	}
	for i := range rates {
		if rates[i].Waehrung == "" || rates[i].Waehrung == "USD" && waehrung != "USD" {
			rates[i].Waehrung = waehrung
		}
		if !ValidCurrency(rates[i].Waehrung) {
			rates[i].Waehrung = waehrung
		}
	}
	return rates, nil
}

// bmfCSVURL is a best-effort download. A failure leaves the embedded fixture in place.
const bmfCSVURL = "https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Steuerarten/Umsatzsteuer/Umsatzsteuer-Umrechnungskurse/umsatzsteuer-umrechnungskurse.csv?__blob=publicationFile"

// FetchBMF tries the monthly VAT-rate CSV. Callers ignore the error and use the cache or the fixture.
func FetchBMF(ctx context.Context, client *http.Client) ([]Monat, error) {
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bmfCSVURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bmf %d", res.StatusCode)
	}
	return ParseBMF(io.LimitReader(res.Body, 1<<20))
}

// ShiftDate moves a civil date by whole days.
func ShiftDate(datum string, days int) string {
	t, err := time.Parse("2006-01-02", datum)
	if err != nil {
		return datum
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}
