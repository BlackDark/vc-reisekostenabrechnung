export type Locale = "de" | "en";

export function pickLocale(
	profile: string | null | undefined,
	stored: string | null | undefined,
	languages: readonly string[],
	fallback: string | null | undefined,
): Locale {
	const candidates = [
		profile,
		stored,
		...languages.map((lang) => lang.slice(0, 2).toLowerCase()),
		fallback,
	];
	for (const candidate of candidates) {
		if (candidate === "de" || candidate === "en") return candidate;
	}
	return "de";
}
