<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import StatusBadge from "$lib/components/status-badge.svelte";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
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
	let reisen = $state<{ id: string; anlass: string }[]>([]);
	let reiseId = $state("");
	let ki = $state<components["schemas"]["KiStand"] | null>(null);
	let kiBusy = $state(false);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void api.GET("/api/v1/reisen").then((res) => {
			reisen = (res.data?.items ?? []).map((row) => ({ id: row.id, anlass: row.anlass }));
			if (!reiseId && reisen[0]) reiseId = reisen[0].id;
		});
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
	const kiOn = $derived(session.aiAktiv && session.nutzer?.ki_erlaubt === true);

	async function loadKI() {
		if (!kiOn || !id) return;
		const res = await api.GET("/api/v1/belege/{id}/ki", { params: { path: { id } } });
		if (res.data) ki = res.data;
	}

	async function readKI() {
		if (!beleg) return;
		kiBusy = true;
		error = "";
		const res = await api.POST("/api/v1/belege/{id}/ki-auslesen", { params: { path: { id: beleg.id } } });
		if (!res.response.ok) {
			error = m.ki_unreachable();
			kiBusy = false;
			return;
		}
		for (let i = 0; i < 20; i++) {
			await new Promise((resolve) => setTimeout(resolve, 500));
			const stand = await api.GET("/api/v1/belege/{id}/ki", { params: { path: { id: beleg.id } } });
			if (stand.data) ki = stand.data;
			if (ki && ki.status !== "laeuft") break;
		}
		kiBusy = false;
	}

	$effect(() => {
		if (!ready || !kiOn || !id) return;
		void loadKI();
	});

	async function acceptKI() {
		if (!beleg || !ki?.vorschlag || !reiseId) return;
		const res = await api.POST("/api/v1/belege/{id}/texte", {
			params: { path: { id: beleg.id } },
			body: { volltext: ki.vorschlag.volltext ?? "", felder: ki.vorschlag },
		});
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		window.location.assign(`/reisen/${reiseId}/ausgaben/neu?beleg=${beleg.id}&vorschlag=1`);
	}
</script>

{#if beleg}
	<p><a class="text-sm underline" href={p("/belege")}>{m.back()}</a></p>
	<h1 class="mt-2 text-2xl font-semibold tracking-tight">{beleg.belegnummer || m.beleg_title()}</h1>
	<p class="mt-2 flex flex-wrap items-center gap-2 text-sm">
		{m.beleg_status()}
		<StatusBadge status={beleg.status} />
	</p>
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
	{#if ready && kiOn && beleg.typ !== "e_rechnung_xml"}
		<Button variant="outline" class="mt-4" type="button" data-testid="ki-auslesen" disabled={kiBusy} onclick={() => void readKI()}>{m.ki_read()}</Button>
		{#if kiBusy || ki?.status === "laeuft"}
			<p class="mt-2 text-sm">{m.ki_running()}</p>
		{:else if ki?.status === "nicht_erreichbar"}
			<p class="mt-2 text-sm" role="alert">{m.ki_unreachable()}</p>
		{:else if ki?.status === "leer"}
			<p class="mt-2 text-sm text-muted-foreground">{m.ki_empty()}</p>
		{:else if ki?.status === "vorschlag" && ki.vorschlag}
			<section class="mt-3 grid gap-1 bg-card rounded-xl border p-3 text-sm" data-testid="ki-vorschlag">
				<p class="font-medium">{m.ki_suggestion()}</p>
				<p>{ki.vorschlag.leistender}</p>
				<p>{((ki.vorschlag.betrag_brutto_cent ?? 0) / 100).toFixed(2)} {ki.vorschlag.waehrung}</p>
				{#if ki.vorschlag.konfidenz}
					<p data-testid="ki-konfidenz">
						{m.ki_confidence()}:
						{#each Object.entries(ki.vorschlag.konfidenz) as [key, value] (key)}
							<span>{key} {Math.round(Number(value) * 100)}%</span>
						{/each}
					</p>
				{/if}
				<Button variant="outline" class="mt-2" type="button" data-testid="ki-verwerfen" onclick={() => { ki = null; }}>{m.ki_discard()}</Button>
			</section>
		{/if}
	{/if}
	<p class="mt-4">
		<a class="underline" href={`/api/v1/belege/${beleg.id}/original`}>{m.beleg_original()}</a>
	</p>
	{#if error}
		<p class="mt-2 text-sm" role="alert">{error}</p>
	{/if}
	{#if reisen.length > 0}
		<div class="mt-4 grid gap-2">
			<label class="grid gap-1 text-sm" for="ausgabe-reise">
				{m.ausgabe_pick_reise()}
				<NativeSelect class="w-full" id="ausgabe-reise" bind:value={reiseId}>
					{#each reisen as trip (trip.id)}
						<option value={trip.id}>{trip.anlass}</option>
					{/each}
				</NativeSelect>
			</label>
			<a class="underline" data-testid="ausgabe-anlegen" href={`/reisen/${reiseId}/ausgaben/neu?beleg=${beleg.id}`}>{m.ausgabe_new()}</a>
			{#if ki?.status === "vorschlag"}
				<Button variant="outline" class="text-left" type="button" data-testid="ki-uebernehmen" onclick={() => void acceptKI()}>{m.ki_accept()}</Button>
			{/if}
		</div>
	{/if}
	<div class="mt-4 grid gap-2">
		{#if beleg.status === "zur_bestaetigung"}
			<Button type="button" onclick={() => void confirm()}>{m.beleg_confirm()}</Button>
			<Button variant="outline" type="button" onclick={() => void reprocess()}>{m.beleg_reprocess()}</Button>
			<Button variant="outline" type="button" onclick={() => void remove()}>{m.beleg_delete()}</Button>
		{:else if beleg.status === "fehlgeschlagen" || beleg.status === "in_aufbereitung" || beleg.status === "hochgeladen"}
			<Button variant="outline" type="button" onclick={() => void remove()}>{m.beleg_delete()}</Button>
		{:else if beleg.status === "bestaetigt"}
			<label class="grid gap-1 text-sm" for="f-belegdetail-1">
				{m.beleg_storno_reason()}
				<Input id="f-belegdetail-1" bind:value={grund}  />
			</label>
			<Button variant="outline" type="button" onclick={() => void storno()}>{m.beleg_storno()}</Button>
		{/if}
	</div>
{/if}
