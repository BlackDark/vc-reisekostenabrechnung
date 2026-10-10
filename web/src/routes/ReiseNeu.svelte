<script lang="ts">
	import { api } from "$lib/api";
	import DateField from "$lib/components/date-field.svelte";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
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

<h1 class="text-2xl font-semibold tracking-tight">{m.reise_new()}</h1>
<p class="mt-2 text-sm">{m.reise_zone_hint()}</p>
{#if templates.length > 0}
	<label class="mt-4 grid gap-1 text-sm" for="vorlage-apply">
		{m.reise_apply()}
		<NativeSelect class="w-full" id="vorlage-apply" onchange={applyTemplate}>
			<option value=""> </option>
			{#each templates as item (item.id)}
				<option value={item.id}>{item.name}</option>
			{/each}
		</NativeSelect>
	</label>
{/if}
<form class="mt-4 grid gap-3" onsubmit={create}>
	<fieldset class="grid gap-3" disabled={!session.online}>
		<label class="grid gap-1 text-sm" for="reise-anlass">
			{m.reise_anlass()}
			<Input id="reise-anlass" bind:value={anlass} required  />
		</label>
		<label class="grid gap-1 text-sm" for="reise-projekt">
			{m.reise_projekt()}
			<Input id="reise-projekt" list="projekte" bind:value={projekt}  />
			<datalist id="projekte">
				{#each projects as name (name)}
					<option value={name}></option>
				{/each}
			</datalist>
		</label>
		<label class="grid gap-1 text-sm" for="reise-ag">
			{m.reise_employer()}
			<NativeSelect class="w-full" id="reise-ag" bind:value={employer} required>
				{#each employers as item (item.id)}
					<option value={item.id}>{item.name}</option>
				{/each}
			</NativeSelect>
		</label>
		<label class="grid gap-1 text-sm" for="reise-beginn">
			{m.reise_beginn()}
			<DateField id="reise-beginn" type="datetime-local" bind:value={beginn} onchange={() => void loadLands()} required  />
		</label>
		<label class="grid gap-1 text-sm" for="reise-ende">
			{m.reise_ende()}
			<DateField id="reise-ende" type="datetime-local" bind:value={ende} required  />
		</label>
		<label class="grid gap-1 text-sm" for="reise-zone">
			{m.reise_zone()}
			<Input id="reise-zone" list="zonen" bind:value={zone} required  />
			<datalist id="zonen">
				<option value="Europe/Berlin"></option>
				<option value="Europe/Paris"></option>
				<option value="Australia/Sydney"></option>
				<option value="America/New_York"></option>
			</datalist>
		</label>
		<label class="grid gap-1 text-sm" for="land-search">
			{m.reise_search()}
			<Input id="land-search" bind:value={landQuery}  />
		</label>
		<label class="grid gap-1 text-sm" for="leg-land">
			{m.reise_country()}
			<NativeSelect class="w-full"
				id="leg-land"
				bind:value={land}
				onchange={() => void loadPlaces()}
			>
				{#each visibleLands(land) as item (item.land_iso)}
					<option value={item.land_iso}>{item.land_name_de}</option>
				{/each}
			</NativeSelect>
		</label>
		<label class="grid gap-1 text-sm" for="leg-place">
			{m.reise_place()}
			<NativeSelect class="w-full" id="leg-place" bind:value={satzort}>
				<option value="">im Übrigen</option>
				{#each places as place (place.satzort + (place.ort_name ?? ""))}
					{#if place.satzort}
						<option value={place.satzort}>{place.ort_name || place.satzort}</option>
					{/if}
				{/each}
			</NativeSelect>
		</label>
		<label class="grid gap-1 text-sm" for="reise-ort">
			{m.reise_city()}
			<Input id="reise-ort" bind:value={ort}  />
		</label>
		<label class="grid gap-1 text-sm" for="reise-means">
			{m.reise_means()}
			<NativeSelect class="w-full" id="reise-means" bind:value={means}>
				<option value="bahn">Bahn</option>
				<option value="flug">Flug</option>
				<option value="pkw">Pkw</option>
				<option value="schiff">Schiff</option>
				<option value="bus">Bus</option>
				<option value="sonstiges">Sonstiges</option>
			</NativeSelect>
		</label>
		<label class="grid gap-1 text-sm" for="reise-unterkunft">
			{m.reise_lodging()}
			<NativeSelect class="w-full" id="reise-unterkunft" bind:value={unterkunft}>
				<option value="gestellt">{m.lodging_gestellt()}</option>
				<option value="pauschale">{m.lodging_pauschale()}</option>
				<option value="beleg">{m.lodging_beleg()}</option>
				<option value="verkehrsmittel">{m.lodging_verkehrsmittel()}</option>
				<option value="keine">{m.lodging_keine()}</option>
			</NativeSelect>
		</label>
		{#each extras as leg, index (index)}
			<fieldset class="grid gap-2 bg-card rounded-xl border p-3">
					<legend class="text-sm">{m.reise_add_leg()}</legend>
					<label class="grid gap-1 text-sm" for="f-reiseneu-1">
						{m.reise_beginn()}
						<DateField id="f-reiseneu-1" type="datetime-local" bind:value={leg.ankunft} required  />
					</label>
					<label class="grid gap-1 text-sm" for="f-reiseneu-2">
						{m.reise_country()}
						<NativeSelect id="f-reiseneu-2" class="w-full" bind:value={leg.land} onchange={() => void loadExtraPlaces(index)}>
							{#each visibleLands(leg.land) as item (item.land_iso)}
								<option value={item.land_iso}>{item.land_name_de}</option>
							{/each}
						</NativeSelect>
					</label>
					<label class="grid gap-1 text-sm" for="f-reiseneu-3">
						{m.reise_place()}
						<NativeSelect id="f-reiseneu-3" class="w-full" bind:value={leg.satzort}>
							<option value="">im Übrigen</option>
							{#each leg.places as place (place.satzort + (place.ort_name ?? ""))}
								{#if place.satzort}
									<option value={place.satzort}>{place.ort_name || place.satzort}</option>
								{/if}
							{/each}
						</NativeSelect>
					</label>
					<label class="grid gap-1 text-sm" for="f-reiseneu-4">
						{m.reise_city()}
						<Input id="f-reiseneu-4" bind:value={leg.ort}  />
					</label>
			</fieldset>
		{/each}
		<Button variant="outline" type="button" onclick={() => void addLeg()}>{m.reise_add_leg()}</Button>
		{#if error}<p class="text-sm" role="alert">{error}</p>{/if}
		<Button type="submit">{m.create()}</Button>
	</fieldset>
</form>
<p class="mt-4 text-sm"><a href={p("/reisen")}>{m.back()}</a></p>
