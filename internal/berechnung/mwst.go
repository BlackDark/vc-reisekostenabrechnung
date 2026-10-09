package berechnung

import "errors"

// Tax splits a gross amount into net and VAT for a rate in hundredths of a percent.
func Tax(brutto, satz int64) (netto, steuer int64) {
	steuer = roundDiv(brutto*satz, 10000+satz)
	return brutto - steuer, steuer
}

// SplitGastronomie splits a gross amount into 70% food at 7% and 30% drinks at 19% (2026).
// The result only prefills an editor.
func SplitGastronomie(brutto int64) []SteuerInput {
	drinks := roundDiv(brutto*30, 100)
	food := brutto - drinks
	return []SteuerInput{
		{Satz: 700, Steuerland: "DE", Brutto: food},
		{Satz: 1900, Steuerland: "DE", Brutto: drinks},
	}
}

// SplitHotel splits a business-package gross amount: 20% at 19% through 2025, 15% from 2026, rest at 7%.
func SplitHotel(brutto int64, year int) []SteuerInput {
	pct := int64(15)
	if year > 0 && year < 2026 {
		pct = 20
	}
	high := roundDiv(brutto*pct, 100)
	return []SteuerInput{
		{Satz: 1900, Steuerland: "DE", Brutto: high},
		{Satz: 700, Steuerland: "DE", Brutto: brutto - high},
	}
}

// EURFromKurs converts a foreign amount with a units-per-EUR rate.
func EURFromKurs(betrag int64, kurs string) (int64, error) {
	num, den, err := parseDecimal(kurs)
	if err != nil || num <= 0 {
		return 0, errors.New("kurs")
	}
	return roundDiv(betrag*den, num), nil
}

// ImplicitKurs documents betrag/betrag_eur to 6 decimal places.
func ImplicitKurs(betrag, betragEUR int64) string {
	if betragEUR <= 0 {
		return ""
	}
	return formatScaled(roundDiv(betrag*1_000_000, betragEUR), 6)
}
