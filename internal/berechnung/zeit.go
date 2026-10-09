package berechnung

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

type civil struct {
	Y int
	M int
	D int
}

func (c civil) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", c.Y, c.M, c.D)
}

func parseCivil(s string) (civil, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return civil{}, err
	}
	y, m, d := t.Date()
	return civil{y, int(m), d}, nil
}

func (c civil) next() civil {
	t := time.Date(c.Y, time.Month(c.M), c.D, 12, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	y, m, d := t.Date()
	return civil{y, int(m), d}
}

func (c civil) add(days int) civil {
	t := time.Date(c.Y, time.Month(c.M), c.D, 12, 0, 0, 0, time.UTC).AddDate(0, 0, days)
	y, m, d := t.Date()
	return civil{y, int(m), d}
}

func (c civil) after(o civil) bool {
	if c.Y != o.Y {
		return c.Y > o.Y
	}
	if c.M != o.M {
		return c.M > o.M
	}
	return c.D > o.D
}

func (c civil) before(o civil) bool { return o.after(c) }

func (c civil) equal(o civil) bool { return c == o }

func (c civil) daysUntil(o civil) int {
	a := time.Date(c.Y, time.Month(c.M), c.D, 12, 0, 0, 0, time.UTC)
	b := time.Date(o.Y, time.Month(o.M), o.D, 12, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

func (z Zeitpunkt) instant() (time.Time, error) {
	if z.Zone == "" {
		return time.Time{}, fmt.Errorf("missing zone")
	}
	loc, err := time.LoadLocation(z.Zone)
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation("2006-01-02T15:04:05", z.Lokal, loc)
}

func (z Zeitpunkt) civil() (civil, error) {
	t, err := z.instant()
	if err != nil {
		return civil{}, err
	}
	y, m, d := t.Date()
	return civil{y, int(m), d}, nil
}

func eachDate(from, to civil) []civil {
	if to.before(from) {
		return nil
	}
	out := make([]civil, 0, from.daysUntil(to)+1)
	for d := from; !d.after(to); d = d.next() {
		out = append(out, d)
	}
	return out
}

// absenceMinutes is the overlap of [start, end] with the civil day in zone.
func absenceMinutes(day civil, zone string, start, end time.Time) (int, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return 0, err
	}
	dayStart := time.Date(day.Y, time.Month(day.M), day.D, 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1)
	a := start
	if a.Before(dayStart) {
		a = dayStart
	}
	b := end
	if b.After(dayEnd) {
		b = dayEnd
	}
	if !b.After(a) {
		return 0, nil
	}
	return int(b.Sub(a) / time.Minute), nil
}

func mondayOf(c civil) civil {
	t := time.Date(c.Y, time.Month(c.M), c.D, 12, 0, 0, 0, time.UTC)
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return c.add(-(wd - 1))
}

func addMonths(c civil, months int) civil {
	t := time.Date(c.Y, time.Month(c.M), c.D, 12, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	y, m, d := t.Date()
	return civil{y, int(m), d}
}
