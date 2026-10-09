<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import DateField from "$lib/components/date-field.svelte";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { euro } from "$lib/money";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p, route } from "../router";

	type Reise = components["schemas"]["Reise"];
	type Day = components["schemas"]["Reisetag"];
	type Tag = components["schemas"]["BerechnungTag"];
	type Fahrt = components["schemas"]["Fahrt"];
	type Land = { land_iso: string; land_name_de: string };
	type Place = { satzort: string; ort_name?: string | null };

	let trip = $state<Reise | null>(null);
	let calc = $state<components["schemas"]["Berechnung"] | null>(null);
	let fahrten = $state<Fahrt[]>([]);
	let ausgaben = $state<components["schemas"]["Ausgabe"][]>([]);
	let lands = $state<Land[]>([]);
	let places = $state<Record<string, Place[]>>({});
	let error = $state("");
	let templateName = $state("");
	let templateSaved = $state("");
	let fahrtStart = $state("");
	let fahrtZiel = $state("");
	let fahrtKm = $state(10);
	let fahrtReturn = $state(true);
	let fahrtVehicle = $state("kraftwagen");
	let fahrtDatum = $state("");

	const id = $derived(route.params.id ?? "");
	const lastDatum = $derived(trip?.reisetage.at(-1)?.datum ?? "");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer || !id) return;
		void load(id);
	});

	function tagOf(datum: string): Tag | undefined {
		return calc?.tage.find((row) => row.datum === datum);
	}

	function normalize(row: Reise) {
		for (const day of row.reisetage) {
			day.land_manuell ??= "";
			day.satzort_manuell ??= "";
			day.begruendung ??= "";
			day.ausschluss_grund ??= "";
			day.zuzahlung_fruehstueck ??= 0;
			day.zuzahlung_mittag ??= 0;
			day.zuzahlung_abend ??= 0;
		}
	}

	async function load(current: string) {
		const [tripRes, calcRes, fahrtRes, ausgabeRes] = await Promise.all([
			api.GET("/api/v1/reisen/{id}", { params: { path: { id: current } } }),
			api.GET("/api/v1/reisen/{id}/berechnung", { params: { path: { id: current } } }),
			api.GET("/api/v1/reisen/{id}/fahrten", { params: { path: { id: current } } }),
			api.GET("/api/v1/reisen/{id}/ausgaben", { params: { path: { id: current } } }),
		]);
		if (!tripRes.data) {
			error = m.save_failed();
			return;
		}
		normalize(tripRes.data);
		trip = tripRes.data;
		calc = calcRes.data ?? null;
		fahrten = fahrtRes.data?.items ?? [];
		ausgaben = ausgabeRes.data?.items ?? [];
		if (!fahrtDatum && trip.reisetage[0]) fahrtDatum = trip.reisetage[0].datum;
		await loadLands(Number(trip.beginn.slice(0, 4)) || 2026);
	}

	async function loadLands(year: number) {
		const tables = await api.GET("/api/v1/satztabellen");
		const active = (tables.data?.items ?? []).filter((row) => row.status === "aktiv");
		let use = year;
		if (!active.some((row) => row.jahr === use) && active[0]) {
			use = active.reduce((best, row) => (row.jahr > best ? row.jahr : best), active[0].jahr);
		}
		const detail = await api.GET("/api/v1/satztabellen/{jahr}", { params: { path: { jahr: use } } });
		lands = [{ land_iso: "DE", land_name_de: "Deutschland" }, ...(detail.data?.laender ?? [])];
	}

	async function placesFor(iso: string) {
		if (!iso || iso === "DE" || places[iso] || !trip) return;
		const year = Number(trip.beginn.slice(0, 4)) || 2026;
		const res = await api.GET("/api/v1/satztabellen/{jahr}/auslandssaetze", {
			params: { path: { jahr: year }, query: { land_iso: iso, limit: 200 } },
		});
		places[iso] = res.data?.items ?? [];
	}

	async function saveDay(day: Day) {
		if (!trip) return;
		error = "";
		const res = await api.PATCH("/api/v1/reisen/{id}/reisetage/{datum}", {
			params: { path: { id: trip.id, datum: day.datum }, header: { "If-Match": String(trip.version) } },
			body: {
				unterkunft: day.datum === lastDatum ? "keine" : day.unterkunft,
				fruehstueck_gestellt: day.fruehstueck_gestellt,
				mittag_gestellt: day.mittag_gestellt,
				abend_gestellt: day.abend_gestellt,
				zuzahlung_fruehstueck: day.zuzahlung_fruehstueck ?? 0,
				zuzahlung_mittag: day.zuzahlung_mittag ?? 0,
				zuzahlung_abend: day.zuzahlung_abend ?? 0,
				verpflegung_ausgeschlossen: day.verpflegung_ausgeschlossen ?? false,
				ausschluss_grund: day.ausschluss_grund,
				land_iso: day.land_manuell,
				satzort: day.satzort_manuell,
				begruendung: day.begruendung,
			},
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		normalize(res.data);
		trip = res.data;
		const calcRes = await api.GET("/api/v1/reisen/{id}/berechnung", { params: { path: { id: trip.id } } });
		calc = calcRes.data ?? calc;
	}

	async function exclude(day: Day) {
		day.verpflegung_ausgeschlossen = true;
		await saveDay(day);
	}

	async function addFahrt(event: SubmitEvent) {
		event.preventDefault();
		if (!trip) return;
		error = "";
		const res = await api.POST("/api/v1/reisen/{id}/fahrten", {
			params: { path: { id: trip.id } },
			body: {
				datum: fahrtDatum,
				start: fahrtStart,
				ziel: fahrtZiel,
				fahrzeugart: fahrtVehicle,
				km: fahrtKm,
				hin_und_zurueck: fahrtReturn,
			},
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		await load(trip.id);
	}

	async function removeFahrt(fahrt: Fahrt) {
		if (!trip) return;
		error = "";
		const res = await api.DELETE("/api/v1/fahrten/{id}", { params: { path: { id: fahrt.id } } });
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		await load(trip.id);
	}

	async function saveTemplate() {
		if (!trip || !templateName.trim()) return;
		error = "";
		const start = Date.parse(trip.beginn);
		const end = Date.parse(trip.ende);
		const dauer = Number.isFinite(start) && Number.isFinite(end) ? Math.max(1, Math.round((end - start) / 60000)) : 480;
		const sample = trip.reisetage.find((day) => day.datum !== lastDatum);
		const res = await api.POST("/api/v1/vorlagen", {
			body: {
				name: templateName.trim(),
				art: "reise",
				daten: {
					anlass: trip.anlass,
					projekt: trip.projekt ?? "",
					arbeitgeber_id: trip.arbeitgeber_id,
					notiz: trip.notiz ?? "",
					unterkunft: sample?.unterkunft ?? "keine",
					dauer_minuten: dauer,
					ortswechsel: trip.ortswechsel.map((leg) => ({
						verkehrsmittel: leg.verkehrsmittel,
						land_iso: leg.land_iso,
						satzort: leg.satzort ?? "",
						ort: leg.ort ?? "",
						taetigkeitsstaette_id: leg.taetigkeitsstaette_id ?? "",
						offset_minuten: offsetMinutes(trip?.beginn ?? "", leg.ankunft),
					})),
				},
			},
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		templateSaved = res.data.name;
	}

	function offsetMinutes(beginn: string, ankunft: string): number {
		const a = Date.parse(beginn);
		const b = Date.parse(ankunft);
		if (!Number.isFinite(a) || !Number.isFinite(b)) return 0;
		return Math.max(0, Math.round((b - a) / 60000));
	}

	async function removeTrip() {
		if (!trip) return;
		const res = await api.DELETE("/api/v1/reisen/{id}", { params: { path: { id: trip.id } } });
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		void navigate("/reisen");
	}

	function money(cents: number | undefined): string {
		return euro(cents ?? 0, session.locale);
	}
</script>

<p><a class="text-sm underline" href={p("/reisen")}>{m.back()}</a></p>
{#if trip}
	<h1 class="mt-2 text-2xl font-semibold tracking-tight">{trip.anlass}</h1>
	<p class="mt-1 text-sm">{trip.beginn.slice(0, 16)} – {trip.ende.slice(0, 16)} ({trip.beginn_zone})</p>
	{#if calc?.blocker && calc.blocker.length > 0}
		<p class="mt-3 text-sm" role="alert">{m.reise_blocker()}: {calc.blocker.join(", ")}</p>
	{/if}
	<p class="mt-3">
		<a class="underline" href={`/reisen/${trip.id}/ausgaben/neu`}>{m.ausgabe_new()}</a>
	</p>
	{#if ausgaben.length > 0}
		<ul class="mt-2 grid gap-2">
			{#each ausgaben as row (row.id)}
				<li>
					<a class="block bg-card rounded-xl border px-3 py-3 text-sm" href={p("/ausgaben/:id", { params: { id: row.id } })}>
						{row.kostenart} · {money(row.betrag_eur_cent)} {row.waehrung === "EUR" ? "" : row.waehrung}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
	{#if calc?.warnungen}
		<ul class="mt-2 grid gap-1 text-sm">
			{#each calc.warnungen as code (code)}
				<li role="status">{code}</li>
			{/each}
		</ul>
	{/if}
	{#if calc}
		<p class="mt-3 text-sm">
			{m.reise_sum()}: {money(calc.summe_cent)} · {m.reise_pauschale()}: {money(calc.verpflegung_cent)} · {m.reise_overnight()}:
			{money(calc.uebernachtung_cent)} · {m.reise_fahrt()}: {money(calc.fahrtkosten_cent)}
		</p>
	{/if}
	<div class="mt-4 grid gap-4">
		{#each trip.reisetage as day (day.id)}
			{@const tag = tagOf(day.datum)}
			<article class="bg-card rounded-xl border p-3" data-datum={day.datum}>
				<h2 class="font-medium">{m.reise_day()} {day.datum}</h2>
				<dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-sm">
					<dt>{m.reise_tagesart()}</dt>
					<dd>{tag?.tagesart ?? ""}</dd>
					<dt>{m.reise_land()}</dt>
					<dd>{tag?.land_iso ?? ""} {tag?.satzort ?? ""}</dd>
					<dt>{m.reise_pauschale()}</dt>
					<dd data-field="pauschale">{money(tag?.pauschale_cent)}</dd>
					<dt>{m.reise_kuerzung()}</dt>
					<dd data-field="kuerzung">{money(tag?.kuerzung_cent)}</dd>
					<dt>{m.reise_result()}</dt>
					<dd>{money(tag?.ergebnis_cent)}</dd>
					<dt>{m.reise_overnight()}</dt>
					<dd>{money(tag?.uebernachtung_cent)}</dd>
					<dt>{m.reise_regeln()}</dt>
					<dd>{tag?.regel_ids?.join(" ") ?? ""} {tag?.land_regel ?? ""}</dd>
				</dl>
				{#if tag?.warnungen?.includes("W03")}
					<p class="mt-2 text-sm" role="status">{m.warn_W03()}</p>
				{/if}
				<div class="mt-3 grid gap-2 text-sm">
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={day.fruehstueck_gestellt} />
						{m.reise_breakfast()}
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={day.mittag_gestellt} />
						{m.reise_lunch()}
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={day.abend_gestellt} />
						{m.reise_dinner()}
					</label>
					<label class="grid gap-1" for="f-reisedetail-1">
						{m.reise_copay()}
						<Input id="f-reisedetail-1" type="number" min="0" step="1" bind:value={day.zuzahlung_mittag}  />
					</label>
					<label class="grid gap-1" for="f-reisedetail-2">
						{m.reise_lodging()}
						<NativeSelect id="f-reisedetail-2" class="w-full" bind:value={day.unterkunft} disabled={day.datum === lastDatum}>
							<option value="keine">{m.lodging_keine()}</option>
							<option value="beleg">{m.lodging_beleg()}</option>
							<option value="pauschale">{m.lodging_pauschale()}</option>
							<option value="gestellt">{m.lodging_gestellt()}</option>
							<option value="verkehrsmittel">{m.lodging_verkehrsmittel()}</option>
						</NativeSelect>
					</label>
					<label class="grid gap-1" for="f-reisedetail-3">
						{m.reise_override()}
						<NativeSelect id="f-reisedetail-3" class="w-full"
							bind:value={day.land_manuell}
							onchange={() => void placesFor(day.land_manuell ?? "")}
						>
							<option value=""> </option>
							{#each lands as item (item.land_iso)}
								<option value={item.land_iso}>{item.land_name_de}</option>
							{/each}
						</NativeSelect>
					</label>
					{#if day.land_manuell && places[day.land_manuell]}
						<label class="grid gap-1" for="f-reisedetail-4">
							{m.reise_place()}
							<NativeSelect id="f-reisedetail-4" class="w-full" bind:value={day.satzort_manuell}>
								<option value="">im Übrigen</option>
								{#each places[day.land_manuell] as place (place.satzort)}
									{#if place.satzort}
										<option value={place.satzort}>{place.ort_name || place.satzort}</option>
									{/if}
								{/each}
							</NativeSelect>
						</label>
					{/if}
					<label class="grid gap-1" for="f-reisedetail-5">
						{m.reise_reason()}
						<Input id="f-reisedetail-5" bind:value={day.begruendung}  />
					</label>
					{#if tag?.warnungen?.includes("W03") && !day.verpflegung_ausgeschlossen}
						<label class="grid gap-1" for="f-reisedetail-6">
							{m.reise_exclude()}
							<Input id="f-reisedetail-6" bind:value={day.ausschluss_grund}  />
						</label>
						<Button variant="outline" type="button" onclick={() => void exclude(day)}>{m.reise_exclude()}</Button>
					{/if}
					<Button type="button" onclick={() => void saveDay(day)}>
						{m.save()}
					</Button>
				</div>
			</article>
		{/each}
	</div>

	<section class="mt-6" aria-label={m.reise_fahrt()}>
		<h2 class="text-lg font-medium">{m.reise_fahrt()}</h2>
		{#if fahrten.length > 0}
			<ul class="mt-2 grid gap-2">
				{#each fahrten as fahrt (fahrt.id)}
					<li class="flex items-center justify-between gap-2 bg-card rounded-xl border px-3 py-2 text-sm">
						<span>{fahrt.start} – {fahrt.ziel} · {fahrt.km} km · {money(fahrt.betrag_cent)}</span>
						<Button variant="link" size="sm" type="button" onclick={() => void removeFahrt(fahrt)}>{m.reise_delete()}</Button>
					</li>
				{/each}
			</ul>
		{/if}
		<form id="fahrt-form" class="mt-3 grid gap-2" onsubmit={addFahrt}>
			<label class="grid gap-1 text-sm" for="fahrt-datum">
				{m.reise_day()}
				<DateField id="fahrt-datum" type="date" bind:value={fahrtDatum} required  />
			</label>
			<label class="grid gap-1 text-sm" for="fahrt-start">
				{m.reise_start()}
				<Input id="fahrt-start" bind:value={fahrtStart} required  />
			</label>
			<label class="grid gap-1 text-sm" for="fahrt-ziel">
				{m.reise_ziel()}
				<Input id="fahrt-ziel" bind:value={fahrtZiel} required  />
			</label>
			<label class="grid gap-1 text-sm" for="fahrt-km">
				{m.reise_km()}
				<Input id="fahrt-km" type="number" min="1" max="100000" bind:value={fahrtKm} required  />
			</label>
			<label class="flex items-center gap-2 text-sm" for="fahrt-return">
				<input id="fahrt-return" type="checkbox" bind:checked={fahrtReturn} />
				{m.reise_return()}
			</label>
			<label class="grid gap-1 text-sm" for="fahrt-vehicle">
				{m.reise_vehicle()}
				<NativeSelect class="w-full" id="fahrt-vehicle" bind:value={fahrtVehicle}>
					<option value="kraftwagen">{m.vehicle_car()}</option>
					<option value="anderes_motorfahrzeug">{m.vehicle_other()}</option>
				</NativeSelect>
			</label>
			<Button variant="outline" type="submit">{m.create()}</Button>
		</form>
	</section>

	<section class="mt-6" aria-label={m.reise_template()}>
		<h2 class="text-lg font-medium">{m.reise_template()}</h2>
		<label class="mt-2 grid gap-1 text-sm" for="vorlage-name">
			{m.reise_template_name()}
			<Input id="vorlage-name" bind:value={templateName}  />
		</label>
		<Button variant="outline" class="mt-2" type="button" onclick={() => void saveTemplate()}>{m.reise_save_template()}</Button>
		{#if templateSaved}<p class="mt-2 text-sm" role="status">{templateSaved}</p>{/if}
	</section>
	{#if error}<p class="mt-3 text-sm" role="alert">{error}</p>{/if}
	<Button variant="link" size="sm" class="mt-6" type="button" onclick={() => void removeTrip()}>{m.reise_delete()}</Button>
{/if}
