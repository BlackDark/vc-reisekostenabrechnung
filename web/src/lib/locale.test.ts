import { describe, expect, it } from "vitest";
import { pickLocale } from "./locale";

describe("pickLocale", () => {
	it("prefers the profile, then storage, then the browser", () => {
		expect(pickLocale("en", "de", ["de"], "de")).toBe("en");
		expect(pickLocale(null, "en", ["de"], "de")).toBe("en");
		expect(pickLocale(null, null, ["en-GB", "de"], "de")).toBe("en");
		expect(pickLocale(null, null, ["fr"], "en")).toBe("en");
		expect(pickLocale(null, null, [], "fr")).toBe("de");
	});
});
