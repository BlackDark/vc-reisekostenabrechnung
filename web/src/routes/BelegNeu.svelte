<script lang="ts">
	import type { Profil, Pt } from "$lib/beleg/geometry";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Page = {
		id: string;
		bitmap: ImageBitmap;
		url: string;
		width: number;
		height: number;
		corners: Pt[];
	};

	let pages = $state<Page[]>([]);
	let profil = $state<Profil>("bon");
	let busy = $state(false);
	let error = $state("");
	let dupId = $state("");
	let mag = $state<{ page: number; corner: number } | null>(null);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	async function addFiles(list: File[]) {
		error = "";
		const docs = list.filter((file) => /pdf|xml/i.test(file.type) || /\.(pdf|xml)$/i.test(file.name));
		const images = list.filter((file) => !docs.includes(file));
		if (docs.length && images.length) {
			error = m.beleg_mixed();
			return;
		}
		if (docs.length) {
			await uploadRaw(docs, false);
			return;
		}
		const mod = await import("$lib/beleg/corners");
		for (const file of images) {
			let bitmap: ImageBitmap;
			try {
				bitmap = await createImageBitmap(file);
			} catch {
				error = m.beleg_heic();
				continue;
			}
			const corners = await mod.detectCorners(bitmap);
			if (pages.length === 0) profil = bitmap.height / Math.max(bitmap.width, 1) > 1.6 ? "bon" : "a4";
			pages.push({
				id: crypto.randomUUID(),
				bitmap,
				url: URL.createObjectURL(file),
				width: bitmap.width,
				height: bitmap.height,
				corners,
			});
		}
	}

	function onInput(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const files = [...(input.files ?? [])];
		input.value = "";
		void addFiles(files);
	}

	function onDrop(event: DragEvent) {
		event.preventDefault();
		void addFiles([...(event.dataTransfer?.files ?? [])]);
	}

	function onPaste(event: ClipboardEvent) {
		const files = [...(event.clipboardData?.files ?? [])];
		if (files.length) void addFiles(files);
	}

	function move(pageIndex: number, cornerIndex: number, event: PointerEvent, img: HTMLImageElement) {
		const page = pages[pageIndex];
		if (!page) return;
		const rect = img.getBoundingClientRect();
		const x = Math.min(page.width, Math.max(0, ((event.clientX - rect.left) / rect.width) * page.width));
		const y = Math.min(page.height, Math.max(0, ((event.clientY - rect.top) / rect.height) * page.height));
		page.corners[cornerIndex] = { x, y };
		const canvas = document.querySelector<HTMLCanvasElement>(`[data-mag="${pageIndex}"]`);
		const ctx = canvas?.getContext("2d");
		if (ctx && canvas) {
			ctx.drawImage(page.bitmap, x - 13, y - 13, 26, 26, 0, 0, 80, 80);
		}
	}

	async function upload(force: boolean) {
		if (busy || pages.length === 0) return;
		busy = true;
		error = "";
		try {
			const mod = await import("$lib/beleg/corners");
			const body = new FormData();
			body.set("profil", profil);
			body.set("ecken", JSON.stringify(pages.map((page) => page.corners)));
			if (force) body.set("duplikat_bestaetigt", "true");
			for (const page of pages) {
				const blob = await mod.toJPEG(page.bitmap, page.corners, profil);
				body.append("datei", blob, "seite.jpg");
			}
			const res = await fetch("/api/v1/belege", { method: "POST", body, credentials: "same-origin" });
			if (res.status === 409) {
				const problem = (await res.json()) as { detail?: string };
				dupId = problem.detail ?? "";
				error = m.beleg_duplikat();
				return;
			}
			if (!res.ok) {
				error = m.save_failed();
				return;
			}
			const doc = (await res.json()) as { id: string };
			await navigate("/belege/:id", { params: { id: doc.id } });
		} finally {
			busy = false;
		}
	}

	async function uploadRaw(files: File[], force: boolean) {
		busy = true;
		error = "";
		try {
			const body = new FormData();
			if (force) body.set("duplikat_bestaetigt", "true");
			for (const file of files) body.append("datei", file, file.name);
			const res = await fetch("/api/v1/belege", { method: "POST", body, credentials: "same-origin" });
			if (!res.ok) {
				error = m.save_failed();
				return;
			}
			const doc = (await res.json()) as { id: string };
			await navigate("/belege/:id", { params: { id: doc.id } });
		} finally {
			busy = false;
		}
	}
</script>

<svelte:window onpaste={onPaste} />

<h1 class="text-2xl font-semibold">{m.beleg_new()}</h1>
<p class="mt-2 text-sm">{m.beleg_drop()}</p>
<section
	class="mt-4 grid gap-3"
	aria-label={m.beleg_drop()}
	ondragover={(event) => event.preventDefault()}
	ondrop={onDrop}
>
	<div class="grid grid-cols-2 gap-2">
		<label class="rounded border px-3 py-3 text-center">
			{m.beleg_camera()}
			<input class="sr-only" type="file" accept="image/*" capture="environment" onchange={onInput} />
		</label>
		<label class="rounded border px-3 py-3 text-center">
			{m.beleg_file()}
			<input data-testid="beleg-file" class="sr-only" type="file" accept="image/*,application/pdf,text/xml,application/xml,.xml,.pdf" multiple onchange={onInput} />
		</label>
	</div>
	<div class="flex gap-2">
		<button class="rounded border px-3 py-2" type="button" aria-pressed={profil === "bon"} onclick={() => (profil = "bon")}>{m.beleg_profil_bon()}</button>
		<button class="rounded border px-3 py-2" type="button" aria-pressed={profil === "a4"} onclick={() => (profil = "a4")}>{m.beleg_profil_a4()}</button>
	</div>
	{#if error}
		<p class="text-sm" role="alert">{error}</p>
	{/if}
	{#if dupId}
		<p class="text-sm">
			<a class="underline" href={p("/belege/:id", { params: { id: dupId } })}>{m.beleg_duplikat()}</a>
			<button class="ml-2 underline" type="button" onclick={() => void upload(true)}>{m.beleg_duplikat_ok()}</button>
		</p>
	{/if}
	{#each pages as page, pageIndex (page.id)}
		<div class="relative" data-testid="beleg-page">
			<img class="w-full" src={page.url} alt={m.beleg_preview()} />
			{#each page.corners as corner, cornerIndex (`${page.id}-${cornerIndex}`)}
				<button
					class="absolute size-7 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white bg-blue-700"
					style={`left: ${(corner.x / page.width) * 100}%; top: ${(corner.y / page.height) * 100}%`}
					type="button"
					aria-label={m.beleg_corner()}
					onpointerdown={(event) => {
						(event.currentTarget as HTMLButtonElement).setPointerCapture(event.pointerId);
						mag = { page: pageIndex, corner: cornerIndex };
					}}
					onpointermove={(event) => {
						if (mag?.page !== pageIndex || mag.corner !== cornerIndex) return;
						const img = event.currentTarget.parentElement?.querySelector("img");
						if (img) move(pageIndex, cornerIndex, event, img);
					}}
					onpointerup={() => (mag = null)}
				></button>
			{/each}
			{#if mag?.page === pageIndex}
				<canvas class="pointer-events-none absolute top-2 right-2 border bg-white" data-mag={pageIndex} width="80" height="80" aria-hidden="true"></canvas>
			{/if}
		</div>
	{/each}
	<button class="rounded bg-blue-800 px-3 py-3 text-white disabled:opacity-50" type="button" disabled={busy || pages.length === 0} onclick={() => void upload(false)}>
		{m.beleg_upload()}
	</button>
</section>
