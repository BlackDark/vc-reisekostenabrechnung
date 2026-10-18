import type { Locator, Page } from "@playwright/test";

// Sidebar link by name. On small screens the sidebar lives in a sheet behind "Mehr".
export async function navLink(page: Page, name: RegExp): Promise<Locator> {
	const more = page.getByRole("button", { name: /Mehr|More/ });
	if (await more.isVisible()) {
		await page
			.locator('[data-slot="sheet-content"]')
			.waitFor({ state: "hidden" })
			.catch(() => {});
		await more.click();
	}
	return page.getByRole("link", { name }).first();
}
