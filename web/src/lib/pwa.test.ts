import {
	afterAll,
	beforeAll,
	beforeEach,
	describe,
	expect,
	it,
	vi,
} from "vitest";
import { isIOS, pwa } from "./pwa.svelte";

type Prompt = Event & {
	prompt: ReturnType<typeof vi.fn>;
	userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
};

function makePrompt(outcome: "accepted" | "dismissed"): Prompt {
	const event = new Event("beforeinstallprompt") as Prompt;
	event.prompt = vi.fn(async () => {});
	event.userChoice = Promise.resolve({ outcome });
	return event;
}

describe("pwa", () => {
	beforeAll(() => {
		vi.stubGlobal("matchMedia", (query: string) => ({
			matches: false,
			media: query,
			addEventListener: () => {},
			removeEventListener: () => {},
		}));
		pwa.init();
	});

	afterAll(() => {
		vi.unstubAllGlobals();
	});

	beforeEach(() => {
		pwa.canInstall = false;
		pwa.installed = false;
	});

	it("offers no install before the browser fires beforeinstallprompt", () => {
		expect(pwa.canInstall).toBe(false);
	});

	it("prompts once the browser captured an install event", async () => {
		const prompt = makePrompt("accepted");
		dispatchEvent(prompt);

		expect(pwa.canInstall).toBe(true);
		expect(await pwa.promptInstall()).toBe(true);
		expect(prompt.prompt).toHaveBeenCalledTimes(1);
		// The captured event is consumed, so the button disappears again.
		expect(pwa.canInstall).toBe(false);
	});

	it("reports a dismissed prompt as not installed", async () => {
		dispatchEvent(makePrompt("dismissed"));

		expect(await pwa.promptInstall()).toBe(false);
		expect(pwa.installed).toBe(false);
	});

	it("marks the app installed once the browser confirms it", () => {
		dispatchEvent(makePrompt("accepted"));
		dispatchEvent(new Event("appinstalled"));

		expect(pwa.canInstall).toBe(false);
		expect(pwa.installed).toBe(true);
	});

	it("never prompts again once installed", async () => {
		dispatchEvent(new Event("appinstalled"));
		const prompt = makePrompt("accepted");
		dispatchEvent(prompt);

		expect(await pwa.promptInstall()).toBe(false);
		expect(prompt.prompt).not.toHaveBeenCalled();
	});
});

describe("isIOS", () => {
	it("detects iPhone, iPad and iPod", () => {
		expect(
			isIOS("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", 5),
		).toBe(true);
		expect(isIOS("Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)", 5)).toBe(
			true,
		);
		expect(
			isIOS("Mozilla/5.0 (iPod touch; CPU iPhone OS 15_0 like Mac OS X)", 5),
		).toBe(true);
	});

	it("treats a touch-capable Macintosh as iPadOS", () => {
		expect(isIOS("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", 5)).toBe(
			true,
		);
		expect(isIOS("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", 0)).toBe(
			false,
		);
	});

	it("leaves Android and desktop browsers to the native prompt", () => {
		expect(isIOS("Mozilla/5.0 (Linux; Android 14) Mobile", 5)).toBe(false);
		expect(isIOS("Mozilla/5.0 (X11; Linux x86_64)", 0)).toBe(false);
		expect(isIOS("Mozilla/5.0 (Windows NT 10.0; Win64; x64)", 0)).toBe(false);
	});
});
