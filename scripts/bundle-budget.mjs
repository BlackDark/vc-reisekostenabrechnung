import { readdirSync, readFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const dir = fileURLToPath(new URL("../web/dist/assets/", import.meta.url));
const files = readdirSync(dir).filter((name) => name.startsWith("index-") && name.endsWith(".js"));
if (files.length !== 1) {
	console.error("expected one initial JS chunk", files);
	process.exit(1);
}
const raw = readFileSync(join(dir, files[0]));
const size = gzipSync(raw).length;
const limit = 150 * 1024;
console.log(`initial JS gzip ${size} bytes (${files[0]})`);
if (size > limit) {
	console.error(`bundle budget exceeded: ${size} > ${limit}`);
	process.exit(1);
}
