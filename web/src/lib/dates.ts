/** Format a stored wall-clock stamp. de → 07.09.2026, 20:00; en → 07/09/2026, 20:00. */
export function formatWhen(
	value: string,
	locale: string,
	mode: "auto" | "date" = "auto",
): string {
	const match = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2}))?/.exec(value);
	if (!match) return value;
	const year = Number(match[1]);
	const month = Number(match[2]);
	const day = Number(match[3]);
	const hour = match[4];
	const minute = match[5];
	const withTime =
		mode === "auto" && hour !== undefined && minute !== undefined;
	const date = new Date(
		Date.UTC(
			year,
			month - 1,
			day,
			withTime ? Number(hour) : 0,
			withTime ? Number(minute) : 0,
		),
	);
	const tag = locale === "en" ? "en-GB" : "de-DE";
	return new Intl.DateTimeFormat(tag, {
		day: "2-digit",
		month: "2-digit",
		year: "numeric",
		timeZone: "UTC",
		...(withTime ? { hour: "2-digit", minute: "2-digit" } : {}),
	}).format(date);
}

/** Inclusive display range. Dates stay dates; a stamp with a time keeps the clock. */
export function formatRange(
	from: string,
	to: string,
	locale: string,
	mode: "auto" | "date" = "date",
): string {
	return `${formatWhen(from, locale, mode)} – ${formatWhen(to, locale, mode)}`;
}
