<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Land = { land_iso: string; land_name_de: string };
	type Place = { satzort: string; ort_name?: string | null };
	type Employer = { id: string; name: string };

	type Extra = { ankunft: string; land: string; satzort: string; ort: string; means: string; places: Place[] };

	let anlass = $state("");
	let projekt = $state("");
	let projects = $state<string[]>([]);
	let beginn = $state("2026-09-07T20:00");
	let ende = $state("2026-09-10T18:00");
	let zone = $state("Europe/Berlin");
	let employer = $state("");
	let employers = $state<Employer[]>([]);
	let land = $state("DE");
	let lands = $state<Land[]>([]);
	let landQuery = $state("");
	let satzort = $state("");
	let places = $state<Place[]>([]);
	let ort = $state("");
	let means = $state("bahn");
	let unterkunft = $state("gestellt");
	let extras = $state<Extra[]>([]);
	let year = $state(2026);
	let error = $state("");
	let templates = $state<{ id: string; name: string }[]>([]);

	function visibleLands(selected: string): Land[] {
		const q = landQuery.trim().toLowerCase();
		return lands.filter((row) => {
			if (row.land_iso === selected) return true;
			if (!q) return true;
			return row.land_iso.toLowerCase().includes(q) || row.land_name_de.toLowerCase().includes(q);
		});
	}

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void boot();
	});

	async function boot() {
		const [ags, vorlagen, projekte] = await Promise.all([
			api.GET("/api/v1/arbeitgeber"),
			api.GET("/api/v1/vorlagen", { params: { query: { art: "reise" } } }),
			api.GET("/api/v1/projekte"),
		]);
		employers = ags.data?.items ?? [];
		if (!employer && employers[0]) employer = employers[0].id;
		templates = (vorlagen.data?.items ?? []).map((row) => ({ id: row.id, name: row.name }));
		projects = projekte.data?.items ?? [];
		await loadLands();
	}

	function tripYear(): number {
		const parsed = Number(beginn.slice(0, 4));
		return Number.isInteger(parsed) && parsed > 2000 ? parsed : 2026;
	}

	async function loadLands() {
		year = tripYear();
		const tables = await api.GET("/api/v1/satztabellen");
		const active = (tables.data?.items ?? []).filter((row) => row.status === "aktiv");
		if (!active.some((row) => row.jahr === year) && active[0]) {
			year = active.reduce((best, row) => (row.jahr > best ? row.jahr : best), active[0].jahr);
		}
		const detail = await api.GET("/api/v1/satztabellen/{jahr}", { params: { path: { jahr: year } } });
		lands = [{ land_iso: "DE", land_name_de: "Deutschland" }, ...(detail.data?.laender ?? [])];
		if (!lands.some((row) => row.land_iso === land)) land = lands[0]?.land_iso ?? "DE";
		await loadPlaces();
	}

	async function loadPlaces() {
		if (land === "DE") {
			places = [];
			satzort = "";
			return;
		}
		const res = await api.GET("/api/v1/satztabellen/{jahr}/auslandssaetze", {
			params: { path: { jahr: year }, query: { land_iso: land, limit: 200 } },
		});
		places = res.data?.items ?? [];
		if (!places.some((place) => place.satzort === satzort)) satzort = "";
	}

	async function addLeg() {
		const row: Extra = { ankunft: ende, land, satzort: "", ort: "", means, places: [] };
		extras = [...extras, row];
		await loadExtraPlaces(extras.length - 1);
	}

	async function loadExtraPlaces(index: number) {
		const row = extras[index];
		if (!row) return;
		if (row.land === "DE") {
			row.places = [];
			row.satzort = "";
			return;
		}
		const res = await api.GET("/api/v1/satztabellen/{jahr}/auslandssaetze", {
			params: { path: { jahr: year }, query: { land_iso: row.land, limit: 200 } },
		});
		row.places = res.data?.items ?? [];
		if (!row.places.some((place) => place.satzort === row.satzort)) row.satzort = "";
	}

	function stamp(value: string): string {
		return value.length === 16 ? `${value}:00` : value;
	}

	async function applyTemplate(event: Event) {
		const id = (event.target as HTMLSelectElement).value;
		if (!id) return;
		const res = await api.POST("/api/v1/vorlagen/{id}/anwenden", {
			params: { path: { id } },
			body: { beginn: stamp(beginn).slice(0, 19), beginn_zone: zone, ende: stamp(ende).slice(0, 19), ende_zone: zone },
		});
		if (res.data?.reise) void navigate("/reisen/:id", { params: { id: res.data.reise.id } });
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.POST("/api/v1/reisen", {
			body: {
				anlass,
				projekt,
				arbeitgeber_id: employer,
				beginn: stamp(beginn),
				beginn_zone: zone,
				ende: stamp(ende),
				ende_zone: zone,
				unterkunft,
				ortswechsel: [
					{
						ankunft: stamp(beginn),
						ankunft_zone: zone,
						verkehrsmittel: means,
						land_iso: land,
						satzort,
						ort,
					},
					...extras.map((leg) => ({
						ankunft: stamp(leg.ankunft),
						ankunft_zone: zone,
						verkehrsmittel: leg.means,
						land_iso: leg.land,
						satzort: leg.satzort,
						ort: leg.ort,
					})),
				],
			},
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		void navigate("/reisen/:id", { params: { id: res.data.id } });
	}
</script>

<h1 class="text-2xl font-semibold">{m.reise_new()}</h1>
<p class="mt-2 text-sm">{m.reise_zone_hint()}</p>
{#if templates.length > 0}
	<label class="mt-4 grid gap-1 text-sm">
		{m.reise_apply()}
		<select id="vorlage-apply" class="rounded border px-3 py-2" onchange={applyTemplate}>
			<option value=""> </option>
			{#each templates as item (item.id)}
				<option value={item.id}>{item.name}</option>
			{/each}
		</select>
	</label>
{/if}
<form class="mt-4 grid gap-3" onsubmit={create}>
	<label class="grid gap-1 text-sm" for="reise-anlass">
		{m.reise_anlass()}
		<input id="reise-anlass" class="rounded border px-3 py-2" bind:value={anlass} required />
	</label>
	<label class="grid gap-1 text-sm" for="reise-projekt">
		{m.reise_projekt()}
		<input id="reise-projekt" class="rounded border px-3 py-2" list="projekte" bind:value={projekt} />
		<datalist id="projekte">
			{#each projects as name (name)}
				<option value={name}></option>
			{/each}
		</datalist>
	</label>
	<label class="grid gap-1 text-sm" for="reise-ag">
		{m.reise_employer()}
		<select id="reise-ag" class="rounded border px-3 py-2" bind:value={employer} required>
			{#each employers as item (item.id)}
				<option value={item.id}>{item.name}</option>
			{/each}
		</select>
	</label>
	<label class="grid gap-1 text-sm" for="reise-beginn">
		{m.reise_beginn()}
		<input id="reise-beginn" class="rounded border px-3 py-2" type="datetime-local" bind:value={beginn} onchange={() => void loadLands()} required />
	</label>
	<label class="grid gap-1 text-sm" for="reise-ende">
		{m.reise_ende()}
		<input id="reise-ende" class="rounded border px-3 py-2" type="datetime-local" bind:value={ende} required />
	</label>
	<label class="grid gap-1 text-sm" for="reise-zone">
		{m.reise_zone()}
		<input id="reise-zone" class="rounded border px-3 py-2" list="zonen" bind:value={zone} required />
		<datalist id="zonen">
			<option value="Europe/Berlin"></option>
			<option value="Europe/Paris"></option>
			<option value="Australia/Sydney"></option>
			<option value="America/New_York"></option>
		</datalist>
	</label>
	<label class="grid gap-1 text-sm" for="land-search">
		{m.reise_search()}
		<input id="land-search" class="rounded border px-3 py-2" bind:value={landQuery} />
	</label>
	<label class="grid gap-1 text-sm" for="leg-land">
		{m.reise_country()}
		<select
			id="leg-land"
			class="rounded border px-3 py-2"
			bind:value={land}
			onchange={() => void loadPlaces()}
		>
			{#each visibleLands(land) as item (item.land_iso)}
				<option value={item.land_iso}>{item.land_name_de}</option>
			{/each}
		</select>
	</label>
	<label class="grid gap-1 text-sm" for="leg-place">
		{m.reise_place()}
		<select id="leg-place" class="rounded border px-3 py-2" bind:value={satzort}>
			<option value="">im Übrigen</option>
			{#each places as place (place.satzort + (place.ort_name ?? ""))}
				{#if place.satzort}
					<option value={place.satzort}>{place.ort_name || place.satzort}</option>
				{/if}
			{/each}
		</select>
	</label>
	<label class="grid gap-1 text-sm" for="reise-ort">
		{m.reise_city()}
		<input id="reise-ort" class="rounded border px-3 py-2" bind:value={ort} />
	</label>
	<label class="grid gap-1 text-sm" for="reise-means">
		{m.reise_means()}
		<select id="reise-means" class="rounded border px-3 py-2" bind:value={means}>
			<option value="bahn">Bahn</option>
			<option value="flug">Flug</option>
			<option value="pkw">Pkw</option>
			<option value="schiff">Schiff</option>
			<option value="bus">Bus</option>
			<option value="sonstiges">Sonstiges</option>
		</select>
	</label>
	<label class="grid gap-1 text-sm" for="reise-unterkunft">
		{m.reise_lodging()}
		<select id="reise-unterkunft" class="rounded border px-3 py-2" bind:value={unterkunft}>
			<option value="gestellt">{m.lodging_gestellt()}</option>
			<option value="pauschale">{m.lodging_pauschale()}</option>
			<option value="beleg">{m.lodging_beleg()}</option>
			<option value="verkehrsmittel">{m.lodging_verkehrsmittel()}</option>
			<option value="keine">{m.lodging_keine()}</option>
		</select>
	</label>
	{#each extras as leg, index (index)}
		<fieldset class="grid gap-2 rounded border p-3">
			<legend class="text-sm">{m.reise_add_leg()}</legend>
			<label class="grid gap-1 text-sm">
				{m.reise_beginn()}
				<input class="rounded border px-3 py-2" type="datetime-local" bind:value={leg.ankunft} required />
			</label>
			<label class="grid gap-1 text-sm">
				{m.reise_country()}
				<select class="rounded border px-3 py-2" bind:value={leg.land} onchange={() => void loadExtraPlaces(index)}>
					{#each visibleLands(leg.land) as item (item.land_iso)}
						<option value={item.land_iso}>{item.land_name_de}</option>
					{/each}
				</select>
			</label>
			<label class="grid gap-1 text-sm">
				{m.reise_place()}
				<select class="rounded border px-3 py-2" bind:value={leg.satzort}>
					<option value="">im Übrigen</option>
					{#each leg.places as place (place.satzort + (place.ort_name ?? ""))}
						{#if place.satzort}
							<option value={place.satzort}>{place.ort_name || place.satzort}</option>
						{/if}
					{/each}
				</select>
			</label>
			<label class="grid gap-1 text-sm">
				{m.reise_city()}
				<input class="rounded border px-3 py-2" bind:value={leg.ort} />
			</label>
		</fieldset>
	{/each}
	<button class="rounded border px-3 py-2" type="button" onclick={() => void addLeg()}>{m.reise_add_leg()}</button>
	{#if error}<p class="text-sm" role="alert">{error}</p>{/if}
	<button class="rounded bg-neutral-900 px-3 py-3 text-white" type="submit">{m.create()}</button>
</form>
<p class="mt-4 text-sm"><a href={p("/reisen")}>{m.back()}</a></p>
