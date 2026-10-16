import { readFileSync } from "node:fs";

// The files may group keys, and a group added to one locale only would pass a
// comparison of the top-level names while rendering as a missing message in the
// other locale. Flattening compares the keys that are actually looked up.
function flatten(value, prefix) {
	const out = new Map();
	for (const [key, entry] of Object.entries(value)) {
		if (key === "$schema") continue;
		const path = prefix ? `${prefix}.${key}` : key;
		if (entry !== null && typeof entry === "object") {
			for (const [nested, text] of flatten(entry, path)) out.set(nested, text);
		} else {
			out.set(path, String(entry));
		}
	}
	return out;
}

function messages(locale) {
	return flatten(
		JSON.parse(readFileSync(new URL(`../web/messages/${locale}.json`, import.meta.url), "utf8")),
		"",
	);
}

function placeholders(text) {
	return [...text.matchAll(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g)].map((match) => match[1]).sort();
}

const de = messages("de");
const en = messages("en");
const missingEn = [...de.keys()].filter((key) => !en.has(key)).sort();
const missingDe = [...en.keys()].filter((key) => !de.has(key)).sort();

// A placeholder named in one locale and not in the other renders as literal
// braces in the second one, so the two sets have to match key by key.
const incompatible = [];
for (const [key, text] of de) {
	if (!en.has(key)) continue;
	const want = placeholders(text).join(",");
	const got = placeholders(en.get(key)).join(",");
	if (want !== got) incompatible.push({ key, de: want, en: got });
}

if (missingEn.length || missingDe.length || incompatible.length) {
	console.error("i18n mismatch", { missingEn, missingDe, incompatible });
	process.exit(1);
}
console.log(`i18n keys ok (${de.size})`);
