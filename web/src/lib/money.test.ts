import { describe, expect, it } from "vitest";
import { parseEuroToCents } from "./money";

describe("parseEuroToCents", () => {
	it("accepts a comma or a dot", () => {
		expect(parseEuroToCents("12,50")).toBe(1250);
		expect(parseEuroToCents("12.5")).toBe(1250);
		expect(parseEuroToCents(" 0 ")).toBe(0);
		expect(parseEuroToCents("nope")).toBe(0);
	});
});
