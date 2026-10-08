import { readFileSync } from "node:fs";

function keys(path) {
	const data = JSON.parse(readFileSync(path, "utf8"));
	return Object.keys(data)
		.filter((key) => key !== "$schema")
		.sort();
}

const de = keys(new URL("../web/messages/de.json", import.meta.url));
const en = keys(new URL("../web/messages/en.json", import.meta.url));
const missingEn = de.filter((key) => !en.includes(key));
const missingDe = en.filter((key) => !de.includes(key));
if (missingEn.length || missingDe.length) {
	console.error("i18n key mismatch", { missingEn, missingDe });
	process.exit(1);
}
console.log(`i18n keys ok (${de.length})`);
