import { expect, type Page, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD || "smoke-password-1";

test.afterEach(async ({ page }, info) => {
	if (info.status === info.expectedStatus) return;
	console.log("page url", page.url());
	console.log((await page.content()).slice(0, 1500));
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

async function employer(page: Page, name: string) {
	await page.goto("/arbeitgeber");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page.getByLabel(/Anschrift|Address/).fill("Weg 1\n10115 Berlin");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.locator("li").filter({ hasText: name })).toBeVisible();
}

test("month claim, export, payment, unlock, second export", async ({
	page,
}) => {
	test.setTimeout(120_000);
	const stamp = Date.now().toString(36);
	const anlass = `Oktober ${stamp}`;
	await login(page);
	await employer(page, `AG ${stamp}`);

	await page.goto("/reisen/neu");
	await expect
		.poll(async () => page.locator("#reise-ag option").count())
		.toBeGreaterThan(0);
	await page.locator("#reise-ag").selectOption({ label: `AG ${stamp}` });
	await page.locator("#reise-anlass").fill(anlass);
	await page.locator("#reise-beginn").fill("2026-10-01T08:00");
	await page.locator("#reise-ende").fill("2026-10-02T18:00");
	await page.locator("#leg-land").selectOption("DE");
	await page.locator("#reise-unterkunft").selectOption("gestellt");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByRole("heading", { name: anlass })).toBeVisible();

	await page.goto("/abrechnungen/neu");
	await expect
		.poll(async () => page.locator("#abrechnung-ag option").count())
		.toBeGreaterThan(0);
	await page.locator("#abrechnung-ag").selectOption({ label: `AG ${stamp}` });
	await page.locator("#abrechnung-art").selectOption("monat");
	await page.locator("#abrechnung-von").fill("2026-10-01");
	await page.getByTestId("abrechnung-create").click();
	await expect(
		page.getByRole("heading", {
			name: /Reisekosten Oktober 2026|Travel expenses October 2026/,
		}),
	).toBeVisible();
	await expect(page.getByRole("checkbox", { name: anlass })).toBeChecked();

	const boxes = page.locator("[data-warn]");
	const count = await boxes.count();
	for (let i = 0; i < count; i++) await boxes.nth(i).check();
	await page.getByTestId("abrechnung-submit").click();
	await expect(page.locator("[data-export-version='1']")).toBeVisible({
		timeout: 30_000,
	});
	const download = page.waitForEvent("download");
	await page
		.locator("[data-export-version='1']")
		.getByTestId("abrechnung-pdf")
		.click();
	expect((await download).suggestedFilename()).toContain(".pdf");
	await expect(page.locator("[data-status]")).toHaveAttribute(
		"data-status",
		"eingereicht",
	);

	const today = await page.evaluate(() =>
		new Date().toLocaleDateString("en-CA", { timeZone: "Europe/Berlin" }),
	);
	await page.locator("#bezahlt-am").fill(today);
	await page.getByTestId("abrechnung-paid").click();
	await expect(page.locator("[data-status]")).toHaveAttribute(
		"data-status",
		"bezahlt",
	);

	await page.locator("#zurueck-grund").fill("Zahlung war zu früh");
	await page.getByTestId("abrechnung-withdraw").click();
	await expect(page.locator("[data-status]")).toHaveAttribute(
		"data-status",
		"eingereicht",
	);

	await page.locator("#entsperr-grund").fill("Beleg nachgereicht");
	await page.getByTestId("abrechnung-unlock").click();
	await expect(page.locator("[data-status]")).toHaveAttribute(
		"data-status",
		"entwurf",
	);
	await expect(page.getByTestId("abrechnung-submit")).toBeVisible();

	const again = page.locator("[data-warn]");
	const againCount = await again.count();
	for (let i = 0; i < againCount; i++) await again.nth(i).check();
	await page.getByTestId("abrechnung-submit").click();
	await expect(page.locator("[data-export-version='2']")).toBeVisible({
		timeout: 30_000,
	});
	await expect(page.locator("[data-status]")).toHaveAttribute(
		"data-status",
		"eingereicht",
	);
});
