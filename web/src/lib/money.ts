/** Parse a euro amount typed with either comma or dot. Invalid input is 0 cents. */
export function parseEuroToCents(raw: string): number {
	const n = Number(raw.trim().replace(/\s/g, "").replace(",", "."));
	if (!Number.isFinite(n)) return 0;
	return Math.round(n * 100);
}

export function euro(cents: number, locale: string): string {
	const abs = Math.abs(Math.trunc(cents));
	const whole = Math.floor(abs / 100);
	const frac = String(abs % 100).padStart(2, "0");
	const sep = locale === "en" ? "." : ",";
	return `${cents < 0 ? "-" : ""}${whole}${sep}${frac}`;
}

/** Locale currency with grouping. de → 1.234,56 €, en → €1,234.56. */
export function euroAmount(cents: number, locale: string): string {
	const value = Math.trunc(cents) / 100;
	const tag = locale === "en" ? "en-GB" : "de-DE";
	return new Intl.NumberFormat(tag, {
		style: "currency",
		currency: "EUR",
	}).format(value);
}
