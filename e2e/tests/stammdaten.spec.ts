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

const png = Buffer.from(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	"base64",
);

test("arbeitgeber, tätigkeitsstätte and rate tables", async ({ page }) => {
	const stamp = Date.now().toString(36);
	await login(page);
	await page.getByRole("link", { name: /Arbeitgeber|Employers/ }).click();
	await page.getByLabel(/^Name$/).fill(`Beispiel ${stamp}`);
	await page.getByLabel(/Anschrift|Address/).fill("Hauptstr. 1\n10115 Berlin");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	const employer = page.locator("li").filter({ hasText: `Beispiel ${stamp}` });
	await expect(employer).toBeVisible();
	const makeDefault = employer.getByRole("button", {
		name: /Als Standard|Make default/,
	});
	if ((await makeDefault.count()) > 0) await makeDefault.click();
	await employer.getByRole("link", { name: /Bearbeiten|Edit/ }).click();
	await page
		.getByLabel(/Logo/)
		.setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: png });
	await expect(
		page.getByRole("img", { name: `Beispiel ${stamp}` }),
	).toBeVisible();

	await page.getByRole("link", { name: /Profil|Profile/ }).click();
	await expect(
		page.getByRole("region", { name: /Briefkopf|Letterhead/ }),
	).toContainText(`Beispiel ${stamp}`);
	await expect(
		page.getByRole("img", { name: `Beispiel ${stamp}` }),
	).toBeVisible();

	await page
		.getByRole("link", { name: /Tätigkeitsstätten|Work locations/ })
		.click();
	await page.getByLabel(/Bezeichnung|^Name$/).fill(`Kunde ${stamp}`);
	await page.getByLabel(/^Land$|^Country$/).selectOption("FR");
	await page.getByLabel(/Satzort|Rate location/).selectOption("FR-PARIS");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByText(`Kunde ${stamp}`)).toBeVisible();
	await expect(page.getByText("FR-PARIS")).toBeVisible();

	await page.getByRole("link", { name: /Satztabellen|Rate tables/ }).click();
	await page.getByRole("link", { name: "2026" }).click();
	await page.getByLabel(/^Suche$|^Search$/).fill("Paris");
	await page.getByRole("button", { name: /^Suche$|^Search$/ }).click();
	await expect(page.getByRole("cell", { name: /58[,.]00/ })).toBeVisible();
	await page.getByLabel(/Grund|Reason/).fill(`E2E ${stamp}`);
	await page.getByRole("button", { name: /Überschreiben|Override/ }).click();
	await expect(page.getByText(`E2E ${stamp}`)).toBeVisible();
	await expect(page.getByRole("cell", { name: /61[,.]00/ })).toBeVisible();

	await page.getByRole("button", { name: "EN" }).click();
	await expect(
		page.getByRole("heading", { name: /Rate tables 2026/ }),
	).toBeVisible();

	await page.getByRole("link", { name: "Back" }).click();
	await page.getByLabel("CSV file").setInputFiles({
		name: "bad.csv",
		mimeType: "text/csv",
		buffer: Buffer.from(
			"jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur,land_iso\n2027,Test,,1,1,nope,TL\n",
		),
	});
	await expect(page.getByRole("alert")).toContainText(
		/nicht übernommen|not imported/,
	);
	await expect(page.getByRole("link", { name: "2027" })).toHaveCount(0);

	await page.getByLabel("CSV file").setInputFiles({
		name: "2027.csv",
		mimeType: "text/csv",
		buffer: Buffer.from(
			"jahr,land,ort,vma_24h_eur,vma_an_abreise_8h_eur,uebernachtung_ag_pauschal_eur,land_iso\n2027,Testland,,12,8,40,TL\n2027,Testland,Genf,13,9,41,TL\n",
		),
	});
	await expect(page.getByRole("link", { name: "2027" })).toBeVisible();
	await page.getByRole("link", { name: "2027" }).click();
	await page.getByRole("button", { name: /Aktivieren|Activate/ }).click();
	await expect(page.getByText(/aktiv|active/)).toBeVisible();
});
