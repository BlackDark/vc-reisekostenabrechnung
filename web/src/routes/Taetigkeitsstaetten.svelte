<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	type Land = { land_iso: string; land_name_de: string };
	type Place = { satzort: string; ort_name?: string | null };
	type Row = {
		id: string;
		bezeichnung: string;
		land_iso: string;
		satzort: string;
		kunde?: string | null;
		version: number;
	};

	let items = $state<Row[]>([]);
	let lands = $state<Land[]>([]);
	let places = $state<Place[]>([]);
	let bezeichnung = $state("");
	let anschrift = $state("");
	let kunde = $state("");
	let land = $state("FR");
	let satzort = $state("");
	let year = $state(2026);
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const [list, tables] = await Promise.all([
			api.GET("/api/v1/taetigkeitsstaetten"),
			api.GET("/api/v1/satztabellen"),
		]);
		if (list.data) items = list.data.items;
		const active = (tables.data?.items ?? []).filter((row) => row.status === "aktiv");
		const newest = active.reduce((best, row) => (row.jahr > best ? row.jahr : best), 0);
		if (newest > 0) {
			year = newest;
			const detail = await api.GET("/api/v1/satztabellen/{jahr}", { params: { path: { jahr: year } } });
			if (detail.data) lands = detail.data.laender;
		}
		await loadPlaces();
	}

	async function loadPlaces() {
		const res = await api.GET("/api/v1/satztabellen/{jahr}/auslandssaetze", {
			params: { path: { jahr: year }, query: { land_iso: land, limit: 200 } },
		});
		places = res.data?.items ?? [];
		if (!places.some((place) => place.satzort === satzort)) satzort = "";
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.POST("/api/v1/taetigkeitsstaetten", {
			body: { bezeichnung, anschrift, land_iso: land, satzort, kunde },
		});
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		bezeichnung = "";
		anschrift = "";
		kunde = "";
		await load();
	}

	async function remove(row: Row) {
		await api.DELETE("/api/v1/taetigkeitsstaetten/{id}", { params: { path: { id: row.id } } });
		await load();
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">{m.staette_title()}</h1>
<form class="mt-4 grid max-w-sm gap-3" onsubmit={create}>
	<fieldset class="grid gap-3" disabled={!session.online}>
		<legend class="text-sm font-medium">{m.staette_new()}</legend>
		<label class="grid gap-1 text-sm" for="st-name">
			{m.staette_name()}
			<Input id="st-name" bind:value={bezeichnung} required  />
		</label>
		<label class="grid gap-1 text-sm" for="st-address">
			{m.staette_address()}
			<Input id="st-address" bind:value={anschrift}  />
		</label>
		<label class="grid gap-1 text-sm" for="st-land">
			{m.staette_land()}
			<NativeSelect class="w-full"
				id="st-land"
				bind:value={land}
				onchange={() => loadPlaces()}
			>
				{#each lands as row (row.land_iso)}
					<option value={row.land_iso}>{row.land_name_de}</option>
				{/each}
			</NativeSelect>
		</label>
		<label class="grid gap-1 text-sm" for="st-place">
			{m.staette_place()}
			<NativeSelect class="w-full" id="st-place" bind:value={satzort}>
				<option value="">{m.staette_rest()}</option>
				{#each places.filter((place) => place.satzort) as place (place.satzort)}
					<option value={place.satzort}>{place.ort_name || place.satzort}</option>
				{/each}
			</NativeSelect>
		</label>
		<label class="grid gap-1 text-sm" for="st-kunde">
			{m.staette_customer()}
			<Input id="st-kunde" bind:value={kunde}  />
		</label>
		<Button type="submit">{m.create()}</Button>
		{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
	</fieldset>
</form>
{#if items.length === 0}
	<p class="mt-6 text-sm text-muted-foreground">{m.staette_empty()}</p>
{:else}
	<ul class="mt-6 grid gap-2 text-sm">
		{#each items as row (row.id)}
			<li class="flex items-center justify-between gap-2 bg-card rounded-xl border p-3">
				<span>{row.bezeichnung} · {row.land_iso}{row.satzort ? ` · ${row.satzort}` : ` · ${m.staette_rest()}`}</span>
				<Button variant="link" size="sm" type="button" onclick={() => remove(row)}>{m.staette_delete()}</Button>
			</li>
		{/each}
	</ul>
{/if}
