import { detectFromRGBA, type Profil, type Pt, warpRGBA } from "./geometry";

export {
	detectFromRGBA,
	profileWidth,
	suggestProfil,
	warpRGBA,
} from "./geometry";
export type { Profil, Pt };

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

export async function detectCorners(bitmap: ImageBitmap): Promise<Pt[]> {
	const max = 180;
	const scale = Math.min(1, max / Math.max(bitmap.width, bitmap.height));
	const w = Math.max(1, Math.round(bitmap.width * scale));
	const h = Math.max(1, Math.round(bitmap.height * scale));
	const canvas = document.createElement("canvas");
	canvas.width = w;
	canvas.height = h;
	const ctx = canvas.getContext("2d", { willReadFrequently: true });
	if (!ctx) return frame(bitmap.width, bitmap.height);
	ctx.drawImage(bitmap, 0, 0, w, h);
	const image = ctx.getImageData(0, 0, w, h);
	try {
		return await workerCall({
			op: "detect",
			width: w,
			height: h,
			rgba: image.data,
			srcW: bitmap.width,
			srcH: bitmap.height,
		});
	} catch {
		return detectFromRGBA(image.data, w, h, bitmap.width, bitmap.height);
	}
}

export async function toJPEG(
	bitmap: ImageBitmap,
	corners: Pt[],
	profil: Profil,
): Promise<Blob> {
	if (bitmap.width * bitmap.height > 40_000_000) {
		throw new Error("pixels");
	}
	const canvas = document.createElement("canvas");
	canvas.width = bitmap.width;
	canvas.height = bitmap.height;
	const ctx = canvas.getContext("2d", { willReadFrequently: true });
	if (!ctx) throw new Error("canvas");
	ctx.drawImage(bitmap, 0, 0);
	const image = ctx.getImageData(0, 0, bitmap.width, bitmap.height);
	try {
		const buf = await workerCall({
			op: "jpeg",
			width: bitmap.width,
			height: bitmap.height,
			rgba: image.data,
			corners,
			profil,
		});
		return new Blob([buf], { type: "image/jpeg" });
	} catch {
		const out = warpRGBA(
			image.data,
			bitmap.width,
			bitmap.height,
			corners,
			profil,
		);
		return imageDataToJPEG(out);
	}
}

async function imageDataToJPEG(image: ImageData): Promise<Blob> {
	const canvas = document.createElement("canvas");
	canvas.width = image.width;
	canvas.height = image.height;
	const ctx = canvas.getContext("2d");
	if (!ctx) throw new Error("canvas");
	ctx.putImageData(image, 0, 0);
	const blob = await new Promise<Blob | null>((resolve) =>
		canvas.toBlob(resolve, "image/jpeg", 0.9),
	);
	if (!blob) throw new Error("jpeg");
	return blob;
}

type DetectMsg = {
	op: "detect";
	width: number;
	height: number;
	rgba: Uint8ClampedArray;
	srcW: number;
	srcH: number;
};
type JpegMsg = {
	op: "jpeg";
	width: number;
	height: number;
	rgba: Uint8ClampedArray;
	corners: Pt[];
	profil: Profil;
};

function workerCall(msg: DetectMsg): Promise<Pt[]>;
function workerCall(msg: JpegMsg): Promise<ArrayBuffer>;
function workerCall(msg: DetectMsg | JpegMsg): Promise<Pt[] | ArrayBuffer> {
	return new Promise((resolve, reject) => {
		const worker = new Worker(new URL("./corners.worker.ts", import.meta.url), {
			type: "module",
		});
		const timer = setTimeout(() => {
			worker.terminate();
			reject(new Error("worker"));
		}, 20000);
		worker.onmessage = (
			ev: MessageEvent<{
				ok: boolean;
				corners?: Pt[];
				jpeg?: ArrayBuffer;
				error?: string;
			}>,
		) => {
			clearTimeout(timer);
			worker.terminate();
			if (!ev.data.ok) {
				reject(new Error(ev.data.error ?? "worker"));
				return;
			}
			if (msg.op === "detect" && ev.data.corners) resolve(ev.data.corners);
			else if (msg.op === "jpeg" && ev.data.jpeg) resolve(ev.data.jpeg);
			else reject(new Error("worker"));
		};
		worker.onerror = () => {
			clearTimeout(timer);
			worker.terminate();
			reject(new Error("worker"));
		};
		worker.postMessage(msg);
	});
}
