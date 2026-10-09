import zlib from "node:zlib";
import { expect, type Page, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD || "smoke-password-1";

test.setTimeout(90_000);

async function login(page: Page) {
	await page.goto("/login");
	await page.getByLabel(/Benutzername|Username/).fill("smoke");
	await page.getByLabel(/Passwort|Password/).fill(password);
	await page.getByRole("button", { name: /Anmelden|Sign in/ }).click();
	await expect(
		page.getByRole("heading", { name: /Reisen|Business trips/ }),
	).toBeVisible();
}

function crc(buf: Buffer) {
	return zlib.crc32(buf);
}

function tinyPNG(seed: number) {
	const sig = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
	const w = 24;
	const h = 24;
	const ihdr = Buffer.alloc(13);
	ihdr.writeUInt32BE(w, 0);
	ihdr.writeUInt32BE(h, 4);
	ihdr[8] = 8;
	ihdr[9] = 2;
	const raw = Buffer.alloc(h * (1 + w * 3));
	for (let y = 0; y < h; y++) {
		const row = y * (1 + w * 3);
		for (let x = 0; x < w; x++) {
			const i = row + 1 + x * 3;
			raw[i] = (seed * 37 + x * 13) & 255;
			raw[i + 1] = (seed * 17 + y * 11) & 255;
			raw[i + 2] = (seed + x + y) & 255;
		}
	}
	const idat = zlib.deflateSync(raw);
	const chunk = (type: string, data: Buffer) => {
		const body = Buffer.concat([Buffer.from(type), data]);
		const len = Buffer.alloc(4);
		len.writeUInt32BE(data.length, 0);
		const sum = Buffer.alloc(4);
		sum.writeUInt32BE(crc(body), 0);
		return Buffer.concat([len, body, sum]);
	};
	return Buffer.concat([
		sig,
		chunk("IHDR", ihdr),
		chunk("IDAT", idat),
		chunk("IEND", Buffer.alloc(0)),
	]);
}

test("upload a receipt, see the thumbnail, download the original", async ({
	page,
}) => {
	const seed =
		(Date.now() & 255) ^ (test.info().project.name === "mobile" ? 0x5a : 0);
	await login(page);
	await page.goto("/belege/neu");
	await page.getByTestId("beleg-file").setInputFiles({
		name: "receipt.png",
		mimeType: "image/png",
		buffer: tinyPNG(seed),
	});
	await expect(page.getByTestId("beleg-page")).toBeVisible();
	await page.getByRole("button", { name: /Hochladen|Upload/ }).click();
	await expect(page).toHaveURL(/\/belege\/(?!neu)/);
	await expect(page.getByTestId("beleg-preview")).toBeVisible({
		timeout: 45_000,
	});
	const href = await page
		.getByRole("link", { name: /Original/ })
		.getAttribute("href");
	expect(href).toBeTruthy();
	const res = await page.request.get(href ?? "");
	expect(res.status()).toBe(200);
	const body = await res.body();
	expect(body[0]).toBe(0xff);
	expect(body[1]).toBe(0xd8);
});
