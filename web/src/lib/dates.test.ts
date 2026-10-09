import { describe, expect, it } from "vitest";
import { formatWhen } from "./dates";

describe("formatWhen", () => {
	it("formats a wall-clock range in German and English", () => {
		expect(formatWhen("2026-09-07T20:00", "de")).toBe("07.09.2026, 20:00");
		expect(formatWhen("2026-09-10T18:00:00", "de")).toBe("10.09.2026, 18:00");
		expect(formatWhen("2026-09-07T20:00", "en")).toBe("07/09/2026, 20:00");
	});

	it("formats a date without a time", () => {
		expect(formatWhen("2026-11-16", "de")).toBe("16.11.2026");
		expect(formatWhen("2026-11-16T20:00", "en", "date")).toBe("16/11/2026");
	});
});
