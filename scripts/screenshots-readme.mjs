import { spawnSync } from "node:child_process";
import { mkdir, readdir, copyFile } from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const source = path.join(root, "e2e/screenshots/gallery");
const dest = path.join(root, "docs/assets/screenshots");
const viewports = ["desktop", "mobile"];

const copied = [];
for (const viewport of viewports) {
	const from = path.join(source, viewport);
	let names = [];
	try {
		names = (await readdir(from)).filter((name) => name.endsWith(".png"));
	} catch {
		console.error(`missing ${from}`);
		process.exit(1);
	}
	const target = path.join(dest, viewport);
	await mkdir(target, { recursive: true });
	for (const name of names) {
		const out = path.join(target, name);
		await copyFile(path.join(from, name), out);
		copied.push(out);
	}
}

const oxipng = spawnSync("oxipng", ["-o", "2", "--strip", "safe", ...copied], { stdio: "inherit" });
if (oxipng.error || oxipng.status !== 0) {
	console.log("oxipng not applied; screenshots were copied as Playwright wrote them");
} else {
	console.log(`optimized ${copied.length} screenshots`);
}
