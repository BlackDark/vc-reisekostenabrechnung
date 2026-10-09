import { expect, type Page, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD || "smoke-password-1";

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

async function employer(page: Page, name: string) {
	await page.goto("/arbeitgeber");
	await page.getByLabel("Name", { exact: true }).fill(name);
	await page.getByLabel(/Anschrift|Address/).fill("Weg 1\n10115 Berlin");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.locator("li").filter({ hasText: name })).toBeVisible();
}

async function openNewTrip(page: Page, employerName: string) {
	await page.goto("/reisen/neu");
	await expect
		.poll(async () => page.locator("#reise-ag option").count())
		.toBeGreaterThan(0);
	await page.locator("#reise-ag").selectOption({ label: employerName });
	await expect
		.poll(async () => page.locator("#leg-land option").count())
		.toBeGreaterThan(1);
}

test("multi-day foreign trip", async ({ page }) => {
	const stamp = Date.now().toString(36);
	const anlass = `Paris ${stamp}`;
	await login(page);
	await employer(page, `AG ${stamp}`);
	await openNewTrip(page, `AG ${stamp}`);
	await page.locator("#reise-anlass").fill(anlass);
	await page.locator("#reise-projekt").fill(`Projekt ${stamp}`);
	await page.locator("#reise-beginn").fill("2026-09-07T20:00");
	await page.locator("#reise-ende").fill("2026-09-10T18:00");
	await page.locator("#reise-zone").fill("Europe/Berlin");
	const land = page.locator("#leg-land");
	const lands = await land
		.locator("option")
		.evaluateAll((opts) => opts.map((opt) => (opt as HTMLOptionElement).value));
	const iso = lands.includes("FR")
		? "FR"
		: (lands.find((value) => value !== "DE") ?? lands[0]);
	await land.selectOption(iso);
	if (iso === "FR") {
		const place = page.locator("#leg-place");
		await expect
			.poll(async () =>
				place
					.locator("option")
					.evaluateAll((opts) =>
						opts.map((opt) => (opt as HTMLOptionElement).value).join("|"),
					),
			)
			.toMatch(/FR-PARIS/);
		await place.selectOption("FR-PARIS");
	}
	await page.locator("#reise-unterkunft").selectOption("gestellt");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByRole("heading", { name: anlass })).toBeVisible();
	await expect(page.getByRole("article")).toHaveCount(4);
	await expect(page.getByRole("article").nth(1)).toContainText(iso);
	await expect(page.locator("[data-field=pauschale]").nth(1)).not.toHaveText(
		/^0[,.]00$/,
	);
	const here = page.url();

	await page.locator("#fahrt-start").fill("Hotel");
	await page.locator("#fahrt-ziel").fill("Kunde");
	await page.locator("#fahrt-km").fill("12");
	await page.locator("#fahrt-return").check();
	await page
		.locator("#fahrt-form")
		.getByRole("button", { name: /Anlegen|Create/ })
		.click();
	await expect(page.getByText("Hotel – Kunde")).toBeVisible();

	await page.locator("#vorlage-name").fill(`Vorlage ${stamp}`);
	await page
		.getByRole("button", { name: /Als Vorlage speichern|Save as template/ })
		.click();
	await expect(page.getByRole("status")).toContainText(`Vorlage ${stamp}`);

	await page.getByRole("button", { name: "EN", exact: true }).click();
	await expect(page.getByText("Applicable country").first()).toBeVisible();
	await expect(page.getByRole("heading", { name: anlass })).toBeVisible();
	const wide = await page.evaluate(
		() =>
			document.documentElement.scrollWidth <=
			document.documentElement.clientWidth + 1,
	);
	expect(wide).toBe(true);

	await page.goto("/reisen/neu");
	await page.locator("#reise-beginn").fill("2026-10-05T08:00");
	await page.locator("#reise-ende").fill("2026-10-07T18:00");
	await expect(
		page.locator("#vorlage-apply option", { hasText: `Vorlage ${stamp}` }),
	).toHaveCount(1);
	await page
		.locator("#vorlage-apply")
		.selectOption({ label: `Vorlage ${stamp}` });
	await expect(page).toHaveURL(/\/reisen\/(?!neu$)[^/]+$/);
	await expect(page).not.toHaveURL(here);
	await expect(page.getByRole("heading", { name: anlass })).toBeVisible();
	await expect(page.getByRole("article")).toHaveCount(3);
	await expect(page.getByRole("article").first()).toContainText(iso);
});

test("inland day shows the meal allowance", async ({ page }) => {
	const stamp = Date.now().toString(36);
	const anlass = `Inland ${stamp}`;
	await login(page);
	await employer(page, `AGI ${stamp}`);
	await openNewTrip(page, `AGI ${stamp}`);
	await page.locator("#reise-anlass").fill(anlass);
	await page.locator("#reise-beginn").fill("2026-03-10T07:00");
	await page.locator("#reise-ende").fill("2026-03-10T17:30");
	await page.locator("#leg-land").selectOption("DE");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByRole("heading", { name: anlass })).toBeVisible();
	const day = page.getByRole("article");
	await expect(day).toHaveCount(1);
	await expect(day).toContainText("eintaegig");
	await expect(day).toContainText("DE");
	await expect(day.locator("[data-field=pauschale]")).not.toHaveText(
		/^0[,.]00$/,
	);
	await day.getByLabel(/Mittag gestellt|Lunch provided/).check();
	await day.getByRole("button", { name: /Speichern|Save/ }).click();
	await expect(day.locator("[data-field=kuerzung]")).not.toHaveText(
		/^0[,.]00$/,
	);
});
