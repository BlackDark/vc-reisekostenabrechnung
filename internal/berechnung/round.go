package berechnung

import (
	"fmt"
	"strings"
)

// roundDiv divides n by d and rounds half away from zero.
func roundDiv(n, d int64) int64 {
	if d == 0 {
		return 0
	}
	neg := false
	if n < 0 {
		n = -n
		neg = true
	}
	if d < 0 {
		d = -d
		neg = !neg
	}
	q := (n + d/2) / d
	if neg {
		return -q
	}
	return q
}

// parseDecimal reads a non-negative decimal into numerator/denominator.
func parseDecimal(s string) (num, den int64, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 1, fmt.Errorf("empty decimal")
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	if !hasFrac {
		frac = ""
	}
	if strings.Contains(frac, ".") {
		return 0, 1, fmt.Errorf("decimal %q", s)
	}
	den = 1
	for range frac {
		den *= 10
	}
	var w, f int64
	if _, err = fmt.Sscan(whole, &w); err != nil {
		return 0, 1, err
	}
	if frac != "" {
		if _, err = fmt.Sscan(frac, &f); err != nil {
			return 0, 1, err
		}
	}
	num = w*den + f
	if neg {
		num = -num
	}
	return num, den, nil
}

// formatScaled prints scaled/10^digits with exactly digits decimals.
func formatScaled(scaled int64, digits int) string {
	neg := scaled < 0
	if neg {
		scaled = -scaled
	}
	div := int64(1)
	for range digits {
		div *= 10
	}
	s := fmt.Sprintf("%d.%0*d", scaled/div, digits, scaled%div)
	if neg {
		return "-" + s
	}
	return s
}
