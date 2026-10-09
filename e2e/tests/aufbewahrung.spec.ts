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

test("admin sees the retention notice", async ({ page }) => {
	await login(page);
	await page.goto("/admin/aufbewahrung");
	await expect(
		page.getByRole("heading", { name: /Aufbewahrung|Retention/ }),
	).toBeVisible();
	await expect(page.getByTestId("aufbewahrung-hinweis")).toContainText(
		/Ablaufhemmung/,
	);
});
