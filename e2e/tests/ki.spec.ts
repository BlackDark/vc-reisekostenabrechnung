import zlib from "node:zlib";
import { expect, type Page, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD || "smoke-password-1";

test.setTimeout(90_000);

test.afterEach(async ({ page }, info) => {
	if (info.status === info.expectedStatus) return;
	console.log("page url", page.url());
	console.log((await page.content()).slice(0, 1200));
});

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

test("receipt suggestion is accepted into an Ausgabe", async ({ page }) => {
	const stamp = Date.now().toString(36);
	const anlass = `KI ${stamp}`;
	const project = test.info().project.name;
	const salt =
		project === "mobile" ? 0x3c : project === "mobile-webkit" ? 0xc3 : 0x71;
	const seed = (Date.now() & 255) ^ salt;
	await login(page);
	await page.goto("/profil");
	await expect(page.getByTestId("ki-privacy")).toContainText("127.0.0.1:8091");
	await page.locator("#ki-erlaubt").check();
	await page.getByRole("button", { name: /Speichern|Save/ }).click();
	await expect(page.locator("#ki-erlaubt")).toBeChecked();

	await page.goto("/arbeitgeber");
	await page.getByLabel("Name", { exact: true }).fill(`AG ${stamp}`);
	await page.getByLabel(/Anschrift|Address/).fill("Weg 1\n10115 Berlin");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(
		page.locator("li").filter({ hasText: `AG ${stamp}` }),
	).toBeVisible();

	await page.goto("/reisen/neu");
	await expect
		.poll(async () => page.locator("#reise-ag option").count())
		.toBeGreaterThan(0);
	await page.locator("#reise-ag").selectOption({ label: `AG ${stamp}` });
	await page.locator("#reise-anlass").fill(anlass);
	await page.locator("#reise-beginn").fill("2026-10-09T08:00");
	await page.locator("#reise-ende").fill("2026-10-09T18:00");
	await page.locator("#leg-land").selectOption("DE");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByRole("heading", { name: anlass })).toBeVisible();

	await page.goto("/belege/neu");
	await page.getByTestId("beleg-file").setInputFiles({
		name: "receipt.png",
		mimeType: "image/png",
		buffer: tinyPNG(seed),
	});
	await expect(page.getByTestId("beleg-page")).toBeVisible({ timeout: 45_000 });
	await page.getByRole("button", { name: /Hochladen|Upload/ }).click();
	await expect(page).toHaveURL(/\/belege\/(?!neu)/, { timeout: 45_000 });
	await expect(page.getByTestId("beleg-preview")).toBeVisible({
		timeout: 45_000,
	});
	await page.getByTestId("ki-auslesen").click();
	await expect(page.getByTestId("ki-vorschlag")).toContainText("Cafe Roma");
	await expect(page.getByTestId("ki-konfidenz")).toBeVisible();
	await page.locator("#ausgabe-reise").selectOption({ label: anlass });
	await page.getByTestId("ki-uebernehmen").click();
	await expect(page).toHaveURL(/vorschlag=1/);
	await expect(page.locator("#ausgabe-leistender")).toHaveValue("Cafe Roma");
	await expect(page.locator("#ausgabe-betrag")).toHaveValue("48.50");
	await expect(page.locator("#ausgabe-waehrung")).toHaveValue("USD");
	await expect(page.locator("#ausgabe-empfaenger")).toHaveValue("Fremde GmbH");
	await expect(page.getByTestId("ki-marke")).toBeVisible();
	await expect(page.getByTestId("warn-w02")).toBeVisible();

	await page.getByRole("button", { name: "EN", exact: true }).click();
	await expect(page.getByTestId("ki-marke")).toContainText("AI suggestion");
	const wide = await page.evaluate(
		() =>
			document.documentElement.scrollWidth <=
			document.documentElement.clientWidth + 1,
	);
	expect(wide).toBe(true);
});
