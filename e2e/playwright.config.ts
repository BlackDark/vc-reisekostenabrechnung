import { defineConfig, devices, type Project } from "@playwright/test";

const baseURL = process.env.E2E_BASE_URL || "http://127.0.0.1:8080";

const projects: Project[] = [
	{
		name: "desktop",
		use: {
			...devices["Desktop Chrome"],
			locale: "de-DE",
			timezoneId: "Europe/Berlin",
			viewport: { width: 1280, height: 800 },
		},
	},
	{
		name: "mobile",
		use: {
			...devices["Pixel 7"],
			locale: "de-DE",
			timezoneId: "Europe/Berlin",
		},
	},
];

if (process.env.E2E_WEBKIT === "1") {
	projects.push({
		name: "mobile-webkit",
		use: {
			...devices["iPhone 13"],
			locale: "de-DE",
			timezoneId: "Europe/Berlin",
		},
	});
}

export default defineConfig({
	testDir: "./tests",
	timeout: 60_000,
	expect: { timeout: 15_000 },
	fullyParallel: false,
	workers: 1,
	retries: process.env.CI ? 1 : 0,
	reporter: [["list"], ["html", { open: "never" }]],
	use: { baseURL, trace: "retain-on-failure" },
	projects,
});
