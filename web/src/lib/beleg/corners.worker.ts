/// <reference lib="webworker" />
import { detectFromRGBA, type Profil, type Pt, warpRGBA } from "./geometry";

type In =
	| {
			op: "detect";
			width: number;
			height: number;
			rgba: Uint8ClampedArray;
			srcW: number;
			srcH: number;
	  }
	| {
			op: "jpeg";
			width: number;
			height: number;
			rgba: Uint8ClampedArray;
			corners: Pt[];
			profil: Profil;
	  };

self.onmessage = (ev: MessageEvent<In>) => {
	const msg = ev.data;
	try {
		if (msg.op === "detect") {
			const corners = detectFromRGBA(
				msg.rgba,
				msg.width,
				msg.height,
				msg.srcW,
				msg.srcH,
			);
			self.postMessage({ ok: true, corners });
			return;
		}
		const image = warpRGBA(
			msg.rgba,
			msg.width,
			msg.height,
			msg.corners,
			msg.profil,
		);
		const canvas = new OffscreenCanvas(image.width, image.height);
		const ctx = canvas.getContext("2d");
		if (!ctx) throw new Error("canvas");
		ctx.putImageData(image, 0, 0);
		canvas.convertToBlob({ type: "image/jpeg", quality: 0.9 }).then(
			async (blob) => {
				self.postMessage({ ok: true, jpeg: await blob.arrayBuffer() });
			},
			(err: unknown) => {
				self.postMessage({
					ok: false,
					error: err instanceof Error ? err.message : "jpeg",
				});
			},
		);
	} catch (err) {
		self.postMessage({
			ok: false,
			error: err instanceof Error ? err.message : "worker",
		});
	}
};
