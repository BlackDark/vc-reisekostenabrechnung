import { readdirSync, readFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const dir = fileURLToPath(new URL("../web/dist/assets/", import.meta.url));
const files = readdirSync(dir);

function gzip(name) {
	return gzipSync(readFileSync(join(dir, name))).length;
}

const js = files.filter((name) => name.endsWith(".js"));
const css = files.filter((name) => name.endsWith(".css"));
const initial = js.filter((name) => name.startsWith("index-"));
if (initial.length !== 1) {
	console.error("expected one initial JS chunk", initial);
	process.exit(1);
}

const initialJs = gzip(initial[0]);
const cssBytes = css.reduce((sum, name) => sum + gzip(name), 0);
const jsBytes = js.reduce((sum, name) => sum + gzip(name), 0);
console.log(`initial JS gzip ${initialJs} bytes (${initial[0]})`);
console.log(`CSS gzip ${cssBytes} bytes (${css.join(", ") || "none"})`);
console.log(`total JS gzip ${jsBytes} bytes`);
