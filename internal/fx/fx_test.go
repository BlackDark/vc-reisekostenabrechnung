package fx

import (
	"strings"
	"testing"
)

func TestWeekendLookbackAndFixture(t *testing.T) {
	kurs, used, ok := Lookback(EmbeddedECB(), "USD", "2026-09-12")
	if !ok || used != "2026-09-11" || kurs != "1.1650" {
		t.Fatalf("%s %s %v", kurs, used, ok)
	}
	if _, _, ok := Lookback(EmbeddedECB(), "USD", "2026-09-20"); ok {
		t.Fatal("gap")
	}
	if _, ok := BMF("USD", 2026, 9); !ok {
		t.Fatal("bmf")
	}
}

func TestParseECB(t *testing.T) {
	raw := "TIME_PERIOD,OBS_VALUE\n2026-09-11,1.1650\n2026-09-12,\n"
	rates, err := ParseECB(strings.NewReader(raw))
	if err != nil || len(rates) != 1 || rates[0].Kurs != "1.1650" {
		t.Fatalf("%+v %v", rates, err)
	}
}
