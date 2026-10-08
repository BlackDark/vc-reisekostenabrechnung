import { expect, type Page, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD || "smoke-password-1";

async function noHorizontalScroll(page: Page) {
	const overflow = await page.evaluate(
		() => document.documentElement.scrollWidth > window.innerWidth + 1,
	);
	expect(overflow).toBe(false);
}

test("password login", async ({ page }) => {
	await page.goto("/login");
	await expect(page.locator('meta[name="rk-app"]')).toHaveAttribute(
		"content",
		"reisekosten",
	);
	await noHorizontalScroll(page);
	await page.getByLabel(/Benutzername|Username/).fill("smoke");
	await page.getByLabel(/Passwort|Password/).fill(password);
	await page.getByRole("button", { name: /Anmelden|Sign in/ }).click();
	await expect(
		page.getByRole("heading", { name: /Reisen|Business trips/ }),
	).toBeVisible();
	await noHorizontalScroll(page);
});

test("OIDC login provisions an admin", async ({ page }) => {
	test.skip(!process.env.E2E_OIDC, "OIDC mock is not running");
	await page.goto("/login");
	await page.getByRole("link", { name: "SSO" }).click();
	await page.getByPlaceholder("Enter any user/subject").fill("ada");
	await page.locator("textarea[name=claims]").fill(
		JSON.stringify({
			groups: ["rk-admins", "rk-users"],
			preferred_username: "ada",
			name: "Ada Lovelace",
			email: "ada@example.com",
			email_verified: true,
		}),
	);
	await page.locator('input[type="submit"]').click();
	await expect(
		page.getByRole("heading", { name: /Reisen|Business trips/ }),
	).toBeVisible();
	await expect(page.getByRole("link", { name: /Nutzer|Users/ })).toBeVisible();
	await noHorizontalScroll(page);
});
