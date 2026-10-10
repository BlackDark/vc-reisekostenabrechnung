import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("$lib/api", () => ({
	api: {
		GET: vi.fn(async () => ({ data: undefined, response: { ok: false } })),
	},
}));

const { session } = await import("./session.svelte");

function setNavigatorOnline(value: boolean) {
	Object.defineProperty(navigator, "onLine", {
		configurable: true,
		get: () => value,
	});
}

/** Every offline guard reads `session.online`, which mirrors the browser. */
describe("session.online", () => {
	beforeEach(() => {
		setNavigatorOnline(true);
	});

	it("takes the browser state on init", async () => {
		setNavigatorOnline(false);
		await session.init();

		expect(session.online).toBe(false);
	});

	it("follows the browser going offline and back online", async () => {
		await session.init();
		expect(session.online).toBe(true);

		dispatchEvent(new Event("offline"));
		expect(session.online).toBe(false);

		dispatchEvent(new Event("online"));
		expect(session.online).toBe(true);
	});
});
