export type Pt = { x: number; y: number };
export type Profil = "bon" | "a4";

export const profileWidth: Record<Profil, number> = { bon: 945, a4: 2480 };

export function detectFromRGBA(
	data: Uint8ClampedArray,
	w: number,
	h: number,
	srcW: number,
	srcH: number,
): Pt[] {
	const score = new Float32Array(w * h);
	let lo = Number.POSITIVE_INFINITY;
	let hi = Number.NEGATIVE_INFINITY;
	for (let i = 0, p = 0; i < data.length; i += 4, p++) {
		const r = data[i] ?? 0;
		const g = data[i + 1] ?? 0;
		const b = data[i + 2] ?? 0;
		const mx = Math.max(r, g, b);
		const sat = mx === 0 ? 0 : mx - Math.min(r, g, b);
		const s = (r + g + b) / 3 - sat;
		score[p] = s;
		if (s < lo) lo = s;
		if (s > hi) hi = s;
	}
	const gray = new Uint8Array(score.length);
	const span = hi - lo || 1;
	for (let i = 0; i < score.length; i++) {
		gray[i] = Math.max(
			0,
			Math.min(255, Math.round((((score[i] ?? 0) - lo) / span) * 255)),
		);
	}
	const paper = otsu(gray);
	const mask = largest(paper, w, h);
	let count = 0;
	for (const v of mask) if (v) count++;
	if (count < w * h * 0.05) return frame(srcW, srcH);
	let tl = 0;
	let tr = 0;
	let br = 0;
	let bl = 0;
	let tlS = Number.POSITIVE_INFINITY;
	let trS = Number.NEGATIVE_INFINITY;
	let brS = Number.NEGATIVE_INFINITY;
	let blS = Number.POSITIVE_INFINITY;
	let seen = false;
	for (let y = 0; y < h; y++) {
		for (let x = 0; x < w; x++) {
			if (!mask[y * w + x]) continue;
			seen = true;
			const s1 = x + y;
			const s2 = x - y;
			if (s1 < tlS) {
				tlS = s1;
				tl = y * w + x;
			}
			if (s2 > trS) {
				trS = s2;
				tr = y * w + x;
			}
			if (s1 > brS) {
				brS = s1;
				br = y * w + x;
			}
			if (s2 < blS) {
				blS = s2;
				bl = y * w + x;
			}
		}
	}
	if (!seen) return frame(srcW, srcH);
	const sx = srcW / w;
	const sy = srcH / h;
	return [tl, tr, br, bl].map((i) => ({
		x: (i % w) * sx,
		y: Math.floor(i / w) * sy,
	}));
}

export function warpRGBA(
	data: Uint8ClampedArray,
	sw: number,
	sh: number,
	corners: Pt[],
	profil: Profil,
): ImageData {
	const qw = Math.hypot(
		(corners[1]?.x ?? 0) - (corners[0]?.x ?? 0),
		(corners[1]?.y ?? 0) - (corners[0]?.y ?? 0),
	);
	const qh = Math.hypot(
		(corners[3]?.x ?? 0) - (corners[0]?.x ?? 0),
		(corners[3]?.y ?? 0) - (corners[0]?.y ?? 0),
	);
	let tw = Math.min(profileWidth[profil], Math.max(1, Math.round(qw)));
	let th = Math.max(1, Math.round(tw * (qh / Math.max(qw, 1))));
	if (th > 8000) {
		const scale = 8000 / th;
		th = 8000;
		tw = Math.max(1, Math.round(tw * scale));
	}
	while (tw * th > 8_000_000) {
		tw = Math.max(1, Math.round(tw * 0.9));
		th = Math.max(1, Math.round(th * 0.9));
	}
	const dst = [
		{ x: 0, y: 0 },
		{ x: tw - 1, y: 0 },
		{ x: tw - 1, y: th - 1 },
		{ x: 0, y: th - 1 },
	];
	const h = solveHomography(dst, corners);
	const out = new Uint8ClampedArray(tw * th * 4);
	if (!h) {
		return new ImageData(out, tw, th);
	}
	for (let y = 0; y < th; y++) {
		for (let x = 0; x < tw; x++) {
			const p = applyH(h, x, y);
			sample(data, sw, sh, p.x, p.y, out, (y * tw + x) * 4);
		}
	}
	return new ImageData(out, tw, th);
}

export function suggestProfil(width: number, height: number): Profil {
	if (width <= 0) return "a4";
	return height / width > 1.6 ? "bon" : "a4";
}

function frame(w: number, h: number): Pt[] {
	const x = Math.max(0, w - 1);
	const y = Math.max(0, h - 1);
	return [
		{ x: 0, y: 0 },
		{ x, y: 0 },
		{ x, y },
		{ x: 0, y },
	];
}

function otsu(gray: Uint8Array): Uint8Array {
	const hist = new Array<number>(256).fill(0);
	for (const v of gray) hist[v] = (hist[v] ?? 0) + 1;
	const total = gray.length;
	let sum = 0;
	for (let i = 0; i < 256; i++) sum += i * (hist[i] ?? 0);
	let sumB = 0;
	let wB = 0;
	let max = -1;
	let threshold = 128;
	for (let t = 0; t < 256; t++) {
		wB += hist[t] ?? 0;
		if (wB === 0) continue;
		const wF = total - wB;
		if (wF === 0) break;
		sumB += t * (hist[t] ?? 0);
		const mB = sumB / wB;
		const mF = (sum - sumB) / wF;
		const between = wB * wF * (mB - mF) * (mB - mF);
		if (between > max) {
			max = between;
			threshold = t;
		}
	}
	const out = new Uint8Array(gray.length);
	for (let i = 0; i < gray.length; i++)
		out[i] = (gray[i] ?? 0) > threshold ? 1 : 0;
	return out;
}

function largest(bin: Uint8Array, w: number, h: number): Uint8Array {
	const seen = new Uint8Array(bin.length);
	const best = new Uint8Array(bin.length);
	let bestN = 0;
	const stack: number[] = [];
	for (let i = 0; i < bin.length; i++) {
		if (!bin[i] || seen[i]) continue;
		stack.push(i);
		seen[i] = 1;
		const comp: number[] = [];
		while (stack.length) {
			const cur = stack.pop();
			if (cur === undefined) break;
			comp.push(cur);
			const x = cur % w;
			const y = Math.floor(cur / w);
			const next = [
				x > 0 ? cur - 1 : -1,
				x + 1 < w ? cur + 1 : -1,
				y > 0 ? cur - w : -1,
				y + 1 < h ? cur + w : -1,
			];
			for (const n of next) {
				if (n >= 0 && bin[n] && !seen[n]) {
					seen[n] = 1;
					stack.push(n);
				}
			}
		}
		if (comp.length > bestN) {
			bestN = comp.length;
			best.fill(0);
			for (const p of comp) best[p] = 1;
		}
	}
	return best;
}

function solveHomography(from: Pt[], to: Pt[]): number[] | null {
	const a: number[][] = [];
	const b: number[] = [];
	for (let i = 0; i < 4; i++) {
		const x = from[i]?.x ?? 0;
		const y = from[i]?.y ?? 0;
		const u = to[i]?.x ?? 0;
		const v = to[i]?.y ?? 0;
		a.push([x, y, 1, 0, 0, 0, -u * x, -u * y]);
		b.push(u);
		a.push([0, 0, 0, x, y, 1, -v * x, -v * y]);
		b.push(v);
	}
	const h = gauss(a, b);
	if (!h) return null;
	h.push(1);
	return h;
}

function gauss(raw: number[][], b: number[]): number[] | null {
	const n = b.length;
	const m = raw.map((row, i) => [...row, b[i] ?? 0]);
	for (let col = 0; col < n; col++) {
		let pivot = col;
		for (let r = col + 1; r < n; r++) {
			if (Math.abs(m[r]?.[col] ?? 0) > Math.abs(m[pivot]?.[col] ?? 0))
				pivot = r;
		}
		const row = m[col];
		const swap = m[pivot];
		if (!row || !swap) return null;
		m[col] = swap;
		m[pivot] = row;
		const div = m[col]?.[col] ?? 0;
		if (Math.abs(div) < 1e-9) return null;
		for (let c = col; c <= n; c++) {
			const line = m[col];
			if (!line) return null;
			line[c] = (line[c] ?? 0) / div;
		}
		for (let r = 0; r < n; r++) {
			if (r === col) continue;
			const factor = m[r]?.[col] ?? 0;
			for (let c = col; c <= n; c++) {
				const line = m[r];
				if (!line) return null;
				line[c] = (line[c] ?? 0) - factor * (m[col]?.[c] ?? 0);
			}
		}
	}
	return m.map((row) => row[n] ?? 0);
}

function applyH(h: number[], x: number, y: number): Pt {
	const w = (h[6] ?? 0) * x + (h[7] ?? 0) * y + (h[8] ?? 1);
	return {
		x: ((h[0] ?? 0) * x + (h[1] ?? 0) * y + (h[2] ?? 0)) / w,
		y: ((h[3] ?? 0) * x + (h[4] ?? 0) * y + (h[5] ?? 0)) / w,
	};
}

function sample(
	src: Uint8ClampedArray,
	sw: number,
	sh: number,
	x: number,
	y: number,
	dst: Uint8ClampedArray,
	at: number,
) {
	if (x < 0 || y < 0 || x >= sw - 1 || y >= sh - 1) {
		dst[at] = 255;
		dst[at + 1] = 255;
		dst[at + 2] = 255;
		dst[at + 3] = 255;
		return;
	}
	const x0 = Math.floor(x);
	const y0 = Math.floor(y);
	const dx = x - x0;
	const dy = y - y0;
	const i00 = (y0 * sw + x0) * 4;
	const i10 = i00 + 4;
	const i01 = i00 + sw * 4;
	const i11 = i01 + 4;
	for (let c = 0; c < 4; c++) {
		const v =
			(src[i00 + c] ?? 0) * (1 - dx) * (1 - dy) +
			(src[i10 + c] ?? 0) * dx * (1 - dy) +
			(src[i01 + c] ?? 0) * (1 - dx) * dy +
			(src[i11 + c] ?? 0) * dx * dy;
		dst[at + c] = Math.max(0, Math.min(255, Math.round(v)));
	}
}
