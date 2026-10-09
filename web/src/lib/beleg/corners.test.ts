import { describe, expect, it } from "vitest";
import { detectFromRGBA, suggestProfil } from "./geometry";

function fill(
	w: number,
	h: number,
	paint: (x: number, y: number) => [number, number, number],
) {
	const data = new Uint8ClampedArray(w * h * 4);
	for (let y = 0; y < h; y++) {
		for (let x = 0; x < w; x++) {
			const [r, g, b] = paint(x, y);
			const i = (y * w + x) * 4;
			data[i] = r;
			data[i + 1] = g;
			data[i + 2] = b;
			data[i + 3] = 255;
		}
	}
	return data;
}

describe("corners", () => {
	it("suggests a receipt profile for a tall page", () => {
		expect(suggestProfil(100, 200)).toBe("bon");
		expect(suggestProfil(200, 200)).toBe("a4");
	});

	it("finds a bright sheet on a dark background", () => {
		const data = fill(40, 40, (x, y) =>
			x >= 10 && x < 30 && y >= 8 && y < 32 ? [250, 250, 245] : [20, 30, 20],
		);
		const corners = detectFromRGBA(data, 40, 40, 40, 40);
		const xs = corners.map((p) => p.x).sort((a, b) => a - b);
		const ys = corners.map((p) => p.y).sort((a, b) => a - b);
		expect(xs[0] ?? 0).toBeGreaterThan(5);
		expect(xs[3] ?? 0).toBeLessThan(35);
		expect(ys[0] ?? 0).toBeGreaterThan(3);
		expect(ys[3] ?? 0).toBeLessThan(36);
	});
});
