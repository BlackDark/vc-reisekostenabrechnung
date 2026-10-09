export function euro(cents: number, locale: string): string {
	const abs = Math.abs(Math.trunc(cents));
	const whole = Math.floor(abs / 100);
	const frac = String(abs % 100).padStart(2, "0");
	const sep = locale === "en" ? "." : ",";
	return `${cents < 0 ? "-" : ""}${whole}${sep}${frac}`;
}
