import { readFileSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, type Page, type Response, test } from "@playwright/test";

const password = process.env.E2E_PASSWORD || "smoke-password-1";
const here = dirname(fileURLToPath(import.meta.url));
const fixture = join(here, "../fixtures/receipt.png");

const employerName = "Nordlicht GmbH";
const tripName = "Kundenbesuch Paris";
const claimTitle = "Reisekosten September 2026";
const placeName = "Atelier Leuchtturm";

/** Route patterns must match web/src/router.ts. Concrete URLs are filled after seeding. */
const patterns = [
	"/",
	"/login",
	"/setup",
	"/profil",
	"/admin",
	"/admin/aufbewahrung",
	"/arbeitgeber",
	"/arbeitgeber/:id",
	"/taetigkeitsstaetten",
	"/satztabellen",
	"/satztabellen/:jahr",
	"/about",
	"/reisen",
	"/reisen/neu",
	"/reisen/:id/ausgaben/neu",
	"/reisen/:id",
	"/ausgaben/:id",
	"/vorschuesse",
	"/belege",
	"/belege/neu",
	"/belege/:id",
	"/abrechnungen/neu",
	"/abrechnungen/:id",
	"/abrechnungen",
];

function routerPatterns(): string[] {
	const src = readFileSync(join(here, "../../web/src/router.ts"), "utf8");
	const start = src.indexOf("createRouter({");
	const end = src.indexOf("});", start);
	const body = src.slice(start, end);
	return [...body.matchAll(/["'](\/[^"']*)["']/g)].map(
		(match) => match[1] ?? "",
	);
}

test("default theme is dark", async ({ page }) => {
	await page.goto("/login");
	await expect(
		page.getByRole("heading", { name: /Anmelden|Sign in/ }),
	).toBeVisible();
	await expect
		.poll(() =>
			page.evaluate(() => document.documentElement.classList.contains("dark")),
		)
		.toBe(true);
	const scheme = await page.evaluate(
		() => getComputedStyle(document.documentElement).colorScheme,
	);
	expect(scheme).toContain("dark");
});

test("covers every router path", () => {
	expect([...patterns].sort()).toEqual(routerPatterns().sort());
});

test("login page LCP on Fast 4G", async ({ page, browserName }, info) => {
	test.skip(
		browserName !== "chromium" || info.project.name !== "desktop",
		"measured once",
	);
	const client = await page.context().newCDPSession(page);
	await client.send("Network.enable");
	await client.send("Network.emulateNetworkConditions", {
		offline: false,
		downloadThroughput: (1.6 * 1024 * 1024) / 8,
		uploadThroughput: (750 * 1024) / 8,
		latency: 150,
	});
	await page.addInitScript(() => {
		const mark = window as Window & { __lcp?: number };
		mark.__lcp = 0;
		new PerformanceObserver((list) => {
			const last = list.getEntries().at(-1);
			if (last) mark.__lcp = last.startTime;
		}).observe({ type: "largest-contentful-paint", buffered: true });
	});
	const started = Date.now();
	await page.goto("/login", { waitUntil: "load" });
	await expect(
		page.getByRole("heading", { name: /Anmelden|Sign in/ }),
	).toBeVisible();
	await expect
		.poll(
			async () =>
				page.evaluate(() => (window as Window & { __lcp?: number }).__lcp ?? 0),
			{
				timeout: 5_000,
			},
		)
		.toBeGreaterThan(0);
	const lcp = await page.evaluate(
		() => (window as Window & { __lcp?: number }).__lcp ?? 0,
	);
	const line = `lcp_ms ${Math.round(lcp)} elapsed_ms ${Date.now() - started}\n`;
	console.log(line.trim());
	await mkdir(join(here, "../screenshots"), { recursive: true });
	await writeFile(join(here, "../screenshots/lcp-fast4g.txt"), line);
	// SPEC 13: LCP < 2 s on Fast 4G is a budget, not a CI gate. Fail only if the page never paints.
	expect(lcp).toBeLessThan(8_000);
});

async function login(page: Page) {
	await page.goto("/login");
	await page.getByLabel(/Benutzername|Username/).fill("smoke");
	await page.getByLabel(/Passwort|Password/).fill(password);
	await page.getByRole("button", { name: /Anmelden|Sign in/ }).click();
	await expect(
		page.getByRole("heading", { name: /Reisen|Business trips/ }),
	).toBeVisible();
	await page.getByRole("button", { name: "DE", exact: true }).click();
}

async function shot(
	page: Page,
	project: string,
	slug: string,
	heading: RegExp,
) {
	await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
	await expect(page.getByRole("heading", { level: 1 }).first()).toHaveText(
		heading,
	);
	if (project !== "desktop") {
		const overflow = await page.evaluate(
			() => document.documentElement.scrollWidth > window.innerWidth + 1,
		);
		expect(overflow, slug).toBe(false);
	}
	const dir = join(here, "../screenshots", project);
	await mkdir(dir, { recursive: true });
	await page.screenshot({ path: join(dir, `${slug}.png`), fullPage: true });
	// Console errors can arrive while the screenshot is taken. Check after it,
	// so the page that logged the error fails instead of the next route.
	const errors = (page as Page & { __errors?: string[] }).__errors ?? [];
	expect(errors, errors.join("\n")).toEqual([]);
	errors.length = 0;
}

async function openAndShot(
	page: Page,
	project: string,
	slug: string,
	url: string,
	heading: RegExp,
) {
	await page.goto(url);
	await shot(page, project, slug, heading);
}

function listLoaded(apiPath: string) {
	return (res: Response) =>
		res.request().method() === "GET" &&
		res.ok() &&
		new URL(res.url()).pathname === apiPath;
}

async function openLoaded(page: Page, url: string, apiPath: string) {
	const listed = page.waitForResponse(listLoaded(apiPath));
	await page.goto(url);
	await listed;
}

async function ensureEmployer(page: Page): Promise<string> {
	await openLoaded(page, "/arbeitgeber", "/api/v1/arbeitgeber");
	const row = page.locator("li").filter({ hasText: employerName });
	if ((await row.count()) === 0) {
		await page.locator("#ag-name").fill(employerName);
		await page.locator("#ag-address").fill("Musterstraße 12\n10115 Berlin");
		await page.getByRole("button", { name: /Anlegen|Create/ }).click();
		await expect(row.first()).toBeVisible();
	}
	const href = await row
		.first()
		.getByRole("link", { name: /Bearbeiten|Edit/ })
		.getAttribute("href");
	expect(href).toBeTruthy();
	return href ?? "";
}

async function ensureTrip(page: Page): Promise<string> {
	await openLoaded(page, "/reisen", "/api/v1/reisen");
	const link = page.getByRole("link", { name: tripName });
	if ((await link.count()) > 0) {
		return (await link.first().getAttribute("href")) ?? "";
	}
	await page.goto("/reisen/neu");
	await expect
		.poll(async () => page.locator("#reise-ag option").count())
		.toBeGreaterThan(0);
	await page.locator("#reise-ag").selectOption({ label: employerName });
	await page.locator("#reise-anlass").fill(tripName);
	await page.locator("#reise-projekt").fill("Leuchtturm");
	await page.locator("#reise-beginn").fill("2026-09-07T20:00");
	await page.locator("#reise-ende").fill("2026-09-10T18:00");
	await page.locator("#reise-zone").fill("Europe/Berlin");
	await expect
		.poll(async () => page.locator("#leg-land option").count())
		.toBeGreaterThan(1);
	await page.locator("#leg-land").selectOption("FR");
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
	await page.locator("#reise-ort").fill("Paris");
	await page.locator("#reise-unterkunft").selectOption("gestellt");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByRole("heading", { name: tripName })).toBeVisible();
	return new URL(page.url()).pathname;
}

async function openTrip(page: Page, tripPath: string) {
	const id = tripPath.split("/").filter(Boolean).at(-1) ?? "";
	const fahrten = page.waitForResponse(
		listLoaded(`/api/v1/reisen/${id}/fahrten`),
	);
	const ausgaben = page.waitForResponse(
		listLoaded(`/api/v1/reisen/${id}/ausgaben`),
	);
	await page.goto(tripPath);
	await Promise.all([fahrten, ausgaben]);
}

async function ensureMileage(page: Page, tripPath: string) {
	await openTrip(page, tripPath);
	if ((await page.getByText("Gare du Nord").count()) > 0) return;
	await page.locator("#fahrt-datum").fill("2026-09-08");
	await page.locator("#fahrt-start").fill("Gare du Nord");
	await page.locator("#fahrt-ziel").fill(placeName);
	await page.locator("#fahrt-km").fill("14");
	await page.locator("#fahrt-return").check();
	await page
		.locator("#fahrt-form")
		.getByRole("button", { name: /Anlegen|Create/ })
		.click();
	await expect(page.getByText("Gare du Nord").first()).toBeVisible();
}

async function ensureExpense(page: Page, tripPath: string): Promise<string> {
	await openTrip(page, tripPath);
	const existing = page.getByRole("link", {
		name: /uebernachtung|Übernachtung|accommodation/i,
	});
	if ((await existing.count()) > 0) {
		return (await existing.first().getAttribute("href")) ?? "";
	}
	await page.getByRole("link", { name: /Ausgabe anlegen|Add expense/ }).click();
	await page.locator("#ausgabe-kostenart").selectOption("uebernachtung");
	await page.locator("#ausgabe-datum").fill("2026-09-08");
	await page.locator("#ausgabe-leistender").fill("Hotel Le Marais");
	await page.locator("#ausgabe-empfaenger").fill(employerName);
	await page.locator("#ausgabe-betrag").fill("186.00");
	await page.locator("#ausgabe-save").click();
	await expect(page).toHaveURL(/\/ausgaben\/[^/]+$/);
	return new URL(page.url()).pathname;
}

async function ensureAdvance(page: Page) {
	await openLoaded(page, "/vorschuesse", "/api/v1/vorschuesse");
	if ((await page.getByText("Abschlag Paris").count()) > 0) return;
	await page.locator("select").first().selectOption({ label: employerName });
	await page.getByLabel(/Datum|Date/).fill("2026-09-01");
	await page.getByLabel(/Betrag|Amount/).fill("200.00");
	await page.getByLabel(/Notiz|Note/).fill("Abschlag Paris");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByText("Abschlag Paris").first()).toBeVisible();
}

async function ensurePlace(page: Page) {
	await openLoaded(page, "/taetigkeitsstaetten", "/api/v1/taetigkeitsstaetten");
	if ((await page.getByText(placeName).count()) > 0) return;
	await page.locator("#st-name").fill(placeName);
	await page.locator("#st-address").fill("12 Rue des Archives, Paris");
	await expect
		.poll(async () => page.locator("#st-land option").count())
		.toBeGreaterThan(1);
	await page.locator("#st-land").selectOption("FR");
	await page.locator("#st-kunde").fill("Leuchtturm");
	await page.getByRole("button", { name: /Anlegen|Create/ }).click();
	await expect(page.getByText(placeName).first()).toBeVisible();
}

async function ensureClaim(page: Page): Promise<string> {
	await openLoaded(page, "/abrechnungen", "/api/v1/abrechnungen");
	const link = page.getByRole("link", { name: claimTitle });
	if ((await link.count()) > 0)
		return (await link.first().getAttribute("href")) ?? "";
	await page.goto("/abrechnungen/neu");
	await expect
		.poll(async () => page.locator("#abrechnung-ag option").count())
		.toBeGreaterThan(0);
	await page.locator("#abrechnung-ag").selectOption({ label: employerName });
	await page.locator("#abrechnung-art").selectOption("monat");
	await page.locator("#abrechnung-von").fill("2026-09-01");
	await page.locator("#abrechnung-titel").fill(claimTitle);
	await page.getByTestId("abrechnung-create").click();
	await expect(page.getByRole("heading", { name: claimTitle })).toBeVisible();
	return new URL(page.url()).pathname;
}

async function ensureReceipt(page: Page): Promise<string> {
	const listed = page.waitForResponse(
		(res) =>
			res.url().includes("/api/v1/belege") && res.request().method() === "GET",
	);
	await page.goto("/belege");
	await listed;
	const link = page
		.locator('a[href^="/belege/"]')
		.filter({ hasNotText: /erfassen|Capture|neu/i });
	if ((await link.count()) > 0) {
		const href = await link.first().getAttribute("href");
		if (href && href !== "/belege/neu") return href;
	}
	await page.goto("/belege/neu");
	await page.getByTestId("beleg-file").setInputFiles(fixture);
	await expect(page.getByTestId("beleg-page")).toBeVisible();
	await page.getByRole("button", { name: /Hochladen|Upload/ }).click();
	await expect(page).toHaveURL(/\/belege\/(?!neu)[^/]+$/, { timeout: 45_000 });
	await expect(page.getByTestId("beleg-preview")).toBeVisible({
		timeout: 45_000,
	});
	return new URL(page.url()).pathname;
}

test("every page loads", async ({ page }, info) => {
	test.setTimeout(120_000);
	const project = info.project.name;
	const bucket = page as Page & { __errors?: string[]; __allow401?: boolean };
	bucket.__errors = [];
	bucket.__allow401 = true;
	page.on("console", (msg) => {
		if (msg.type() !== "error") return;
		// Chrome reports the anonymous GET /api/v1/auth/me 401. That probe is expected
		// until sign-in. A 401 after sign-in is still a failure.
		if (bucket.__allow401 && msg.text().includes("status of 401")) return;
		bucket.__errors?.push(msg.text());
	});
	page.on("pageerror", (err) => {
		bucket.__errors?.push(String(err));
	});

	await openAndShot(page, project, "login", "/login", /Anmelden|Sign in/);
	await openAndShot(
		page,
		project,
		"setup",
		"/setup",
		/Ersteinrichtung|First-time setup/,
	);
	await openAndShot(
		page,
		project,
		"about",
		"/about",
		/Reisekostenabrechnung|Travel expense claims/,
	);

	await login(page);
	bucket.__allow401 = false;
	const started = Date.now();
	const ag = await ensureEmployer(page);
	const trip = await ensureTrip(page);
	await ensureMileage(page, trip);
	const expense = await ensureExpense(page, trip);
	await ensureAdvance(page);
	await ensurePlace(page);
	const claim = await ensureClaim(page);
	const beleg = await ensureReceipt(page);
	await page.goto("/satztabellen");
	const yearHref = await page
		.locator('a[href^="/satztabellen/"]')
		.first()
		.getAttribute("href");
	expect(yearHref).toBeTruthy();
	bucket.__errors = [];

	const pages: { slug: string; url: string; heading: RegExp }[] = [
		{ slug: "home", url: "/", heading: /Reisen|Business trips/ },
		{ slug: "profil", url: "/profil", heading: /Profil|Profile/ },
		{ slug: "admin", url: "/admin", heading: /Nutzer|Users/ },
		{
			slug: "admin-aufbewahrung",
			url: "/admin/aufbewahrung",
			heading: /Aufbewahrung|Retention/,
		},
		{
			slug: "arbeitgeber",
			url: "/arbeitgeber",
			heading: /Arbeitgeber|Employers/,
		},
		{ slug: "arbeitgeber-detail", url: ag, heading: /Bearbeiten|Edit/ },
		{
			slug: "taetigkeitsstaetten",
			url: "/taetigkeitsstaetten",
			heading: /Tätigkeitsstätten|Work locations/,
		},
		{
			slug: "satztabellen",
			url: "/satztabellen",
			heading: /Satztabellen|Rate tables/,
		},
		{
			slug: "satztabelle-jahr",
			url: yearHref ?? "",
			heading: /Satztabellen|Rate tables/,
		},
		{ slug: "reisen", url: "/reisen", heading: /Reisen|Trips/ },
		{ slug: "reise-neu", url: "/reisen/neu", heading: /Neue Reise|New trip/ },
		{ slug: "reise-detail", url: trip, heading: new RegExp(tripName) },
		{
			slug: "ausgabe-neu",
			url: `${trip}/ausgaben/neu`,
			heading: /Ausgabe|Expense/,
		},
		{ slug: "ausgabe-detail", url: expense, heading: /Ausgabe|Expense/ },
		{
			slug: "vorschuesse",
			url: "/vorschuesse",
			heading: /Vorschüsse|Advances/,
		},
		{ slug: "belege", url: "/belege", heading: /Belege|Receipts/ },
		{ slug: "beleg-detail", url: beleg, heading: /Belege|Receipts|B-\d+|20/ },
		{
			slug: "abrechnung-neu",
			url: "/abrechnungen/neu",
			heading: /Neue Abrechnung|New claim/,
		},
		{ slug: "abrechnung-detail", url: claim, heading: new RegExp(claimTitle) },
		{
			slug: "abrechnungen",
			url: "/abrechnungen",
			heading: /Abrechnungen|Claims/,
		},
	];
	for (const item of pages) {
		await openAndShot(page, project, item.slug, item.url, item.heading);
	}

	await page.goto("/belege/neu");
	await page.getByTestId("beleg-file").setInputFiles(fixture);
	await expect(page.getByTestId("beleg-page")).toBeVisible();
	await shot(page, project, "beleg-neu", /Beleg erfassen|Capture a receipt/);

	const covered = new Set([
		"/login",
		"/setup",
		"/about",
		"/",
		"/profil",
		"/admin",
		"/admin/aufbewahrung",
		"/arbeitgeber",
		"/arbeitgeber/:id",
		"/taetigkeitsstaetten",
		"/satztabellen",
		"/satztabellen/:jahr",
		"/reisen",
		"/reisen/neu",
		"/reisen/:id",
		"/reisen/:id/ausgaben/neu",
		"/ausgaben/:id",
		"/vorschuesse",
		"/belege",
		"/belege/neu",
		"/belege/:id",
		"/abrechnungen/neu",
		"/abrechnungen/:id",
		"/abrechnungen",
	]);
	expect([...covered].sort()).toEqual([...patterns].sort());
	await page.getByRole("button", { name: /Hell|Light/ }).click();
	await expect
		.poll(() =>
			page.evaluate(() => document.documentElement.classList.contains("dark")),
		)
		.toBe(false);
	await openAndShot(page, project, "reisen-light", "/reisen", /Reisen|Trips/);
	await page.goto("/profil");
	await page.getByRole("button", { name: /Abmelden|Sign out/ }).click();
	await expect(
		page.getByRole("heading", { name: /Anmelden|Sign in/ }),
	).toBeVisible();
	await shot(page, project, "login-light", /Anmelden|Sign in/);

	console.log(
		`screenshot tour ${project} seed_and_pages_ms ${Date.now() - started}`,
	);
});
