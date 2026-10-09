<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p, route } from "../router";

	type Beleg = {
		id: string;
		status: string;
		typ: string;
		seiten: number;
		version: number;
		belegnummer?: string | null;
		duplikat_von?: string | null;
		pipeline_version?: string | null;
	};

	const id = $derived(route.params.id ?? "");
	let beleg = $state<Beleg | null>(null);
	let error = $state("");
	let grund = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer || !id) return;
		let stop = false;
		let delay = 1000;
		async function poll() {
			const res = await api.GET("/api/v1/belege/{id}", { params: { path: { id } } });
			if (stop) return;
			if (res.data) {
				beleg = res.data;
				const etag = res.response.headers.get("ETag");
				if (etag) beleg = { ...res.data, version: Number(etag) || res.data.version };
			}
			if (!beleg || beleg.status === "in_aufbereitung" || beleg.status === "hochgeladen") {
				setTimeout(() => {
					if (!stop) {
						delay = Math.min(5000, Math.round(delay * 1.5));
						void poll();
					}
				}, delay);
			}
		}
		void poll();
		return () => {
			stop = true;
		};
	});

	function etag() {
		return String(beleg?.version ?? "");
	}

	async function confirm() {
		if (!beleg) return;
		const res = await api.POST("/api/v1/belege/{id}/bestaetigen", {
			params: { path: { id: beleg.id }, header: { "If-Match": etag() } },
		});
		if (res.data) beleg = res.data;
		else error = m.save_failed();
	}

	async function remove() {
		if (!beleg) return;
		const res = await api.DELETE("/api/v1/belege/{id}", {
			params: { path: { id: beleg.id }, header: { "If-Match": etag() } },
		});
		if (res.response.ok) await navigate("/belege");
		else error = m.save_failed();
	}

	async function storno() {
		if (!beleg || !grund.trim()) return;
		const res = await api.POST("/api/v1/belege/{id}/stornieren", {
			params: { path: { id: beleg.id }, header: { "If-Match": etag() } },
			body: { grund },
		});
		if (res.data) beleg = res.data;
		else error = m.save_failed();
	}

	async function reprocess() {
		if (!beleg) return;
		const res = await api.POST("/api/v1/belege/{id}/neu-aufbereiten", {
			params: { path: { id: beleg.id }, header: { "If-Match": etag() } },
		});
		if (res.data) beleg = { ...res.data, version: Number(res.response.headers.get("ETag")) || res.data.version };
		else error = m.save_failed();
	}

	const ready = $derived(beleg && beleg.status !== "in_aufbereitung" && beleg.status !== "hochgeladen");
</script>

{#if beleg}
	<p><a class="text-sm underline" href={p("/belege")}>{m.back()}</a></p>
	<h1 class="mt-2 text-2xl font-semibold">{beleg.belegnummer || m.beleg_title()}</h1>
	<p class="mt-2 text-sm">{m.beleg_status()}: {beleg.status}</p>
	{#if beleg.pipeline_version}
		<p class="text-sm">{m.about_pipeline()}: {beleg.pipeline_version}</p>
	{/if}
	{#if beleg.duplikat_von}
		<p class="mt-2 text-sm">
			<a class="underline" href={p("/belege/:id", { params: { id: beleg.duplikat_von } })}>{m.beleg_duplikat()}</a>
		</p>
	{/if}
	{#if ready && beleg.typ !== "e_rechnung_xml"}
		<img class="mt-4 w-full" data-testid="beleg-preview" src={`/api/v1/belege/${beleg.id}/vorschau`} alt={m.beleg_preview()} />
	{/if}
	<p class="mt-4">
		<a class="underline" href={`/api/v1/belege/${beleg.id}/original`}>{m.beleg_original()}</a>
	</p>
	{#if error}
		<p class="mt-2 text-sm" role="alert">{error}</p>
	{/if}
	<div class="mt-4 grid gap-2">
		{#if beleg.status === "zur_bestaetigung"}
			<button class="rounded bg-blue-800 px-3 py-3 text-white" type="button" onclick={() => void confirm()}>{m.beleg_confirm()}</button>
			<button class="rounded border px-3 py-3" type="button" onclick={() => void reprocess()}>{m.beleg_reprocess()}</button>
			<button class="rounded border px-3 py-3" type="button" onclick={() => void remove()}>{m.beleg_delete()}</button>
		{:else if beleg.status === "fehlgeschlagen" || beleg.status === "in_aufbereitung" || beleg.status === "hochgeladen"}
			<button class="rounded border px-3 py-3" type="button" onclick={() => void remove()}>{m.beleg_delete()}</button>
		{:else if beleg.status === "bestaetigt"}
			<label class="grid gap-1 text-sm">
				{m.beleg_storno_reason()}
				<input class="rounded border px-3 py-2" bind:value={grund} />
			</label>
			<button class="rounded border px-3 py-3" type="button" onclick={() => void storno()}>{m.beleg_storno()}</button>
		{/if}
	</div>
{/if}
