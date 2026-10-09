<script lang="ts">
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import InfoIcon from "@lucide/svelte/icons/info";
	import { toast } from "svelte-sonner";
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import DateField from "$lib/components/date-field.svelte";
	import StatCard from "$lib/components/stat-card.svelte";
	import StatusBadge from "$lib/components/status-badge.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import * as Sheet from "$lib/components/ui/sheet";
	import * as Tabs from "$lib/components/ui/tabs";
	import * as Tooltip from "$lib/components/ui/tooltip";
	import { formatWhen } from "$lib/dates";
	import { dayTypeLabel, kostenartLabel, warningLabel } from "$lib/labels";
	import { euroAmount } from "$lib/money";
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
	let belege = $state<components["schemas"]["Beleg"][]>([]);
	let tab = $state("days");
	let confirmOpen = $state(false);
	let pendingDelete = $state<string | null>(null);
	const dayGrid =
		"grid-cols-[8.5rem_7.5rem_minmax(9rem,1.4fr)_repeat(3,5.75rem)] items-center gap-x-3";
	let fahrtOpen = $state(false);
	let vorlageOpen = $state(false);

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
		await Promise.all([loadLands(Number(trip.beginn.slice(0, 4)) || 2026), loadBelege(ausgaben)]);
	}

	async function loadBelege(rows: components["schemas"]["Ausgabe"][]) {
		const ids = new Set(rows.flatMap((row) => row.beleg_ids ?? []));
		if (ids.size === 0) {
			belege = [];
			return;
		}
		const list = await api.GET("/api/v1/belege", { params: { query: { limit: 100 } } });
		belege = (list.data?.items ?? []).filter((item) => ids.has(item.id));
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
		toast.success(m.saved());
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
		fahrtOpen = false;
		tab = "fahrten";
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
		vorlageOpen = false;
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

	function askDelete(id: string) {
		pendingDelete = id;
		confirmOpen = true;
	}

	async function confirmDelete() {
		const target = pendingDelete;
		confirmOpen = false;
		pendingDelete = null;
		if (target === "trip") {
			await removeTrip();
			return;
		}
		const fahrt = fahrten.find((row) => row.id === target);
		if (fahrt) await removeFahrt(fahrt);
	}

	function money(cents: number | undefined): string {
		return euroAmount(cents ?? 0, session.locale);
	}

	const ausgabenCent = $derived((calc?.reisenebenkosten_cent ?? 0) + (calc?.bewirtung_cent ?? 0));
	const overnightOnDays = $derived((calc?.tage ?? []).reduce((sum, tag) => sum + (tag.uebernachtung_cent ?? 0), 0));
	const overnightReceipts = $derived(Math.max(0, (calc?.uebernachtung_cent ?? 0) - overnightOnDays));
</script>

<Button variant="ghost" size="sm" href={p("/reisen")}>
	<ArrowLeftIcon />
	{m.back()}
</Button>
{#if trip}
	{#snippet actions(current: Reise)}
		<Button variant="outline" size="sm" href={`/reisen/${current.id}/ausgaben/neu`}>{m.ausgabe_new()}</Button>
		<Button variant="outline" size="sm" type="button" onclick={() => (fahrtOpen = true)}>{m.reise_add_fahrt()}</Button>
		<Button variant="outline" size="sm" type="button" onclick={() => (vorlageOpen = true)}>{m.reise_open_template()}</Button>
		<Button variant="destructive" size="sm" type="button" onclick={() => askDelete("trip")}>{m.reise_delete()}</Button>
	{/snippet}

	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h1 class="truncate text-2xl font-semibold tracking-tight">{trip.anlass}</h1>
			<p class="text-muted-foreground mt-1 text-sm" data-field="zeitraum">
				{formatWhen(trip.beginn, session.locale)} – {formatWhen(trip.ende, session.locale)}
			</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<StatusBadge status={trip.status} />
			<div
				class="flex max-w-full flex-wrap gap-2 max-md:fixed max-md:inset-x-0 max-md:bottom-16 max-md:z-30 max-md:border-t max-md:bg-background/95 max-md:px-3 max-md:py-2"
			>
				{@render actions(trip)}
			</div>
		</div>
	</div>

	{#if calc?.blocker && calc.blocker.length > 0}
		<p class="mt-3 text-sm" role="alert">{m.reise_blocker()}: {calc.blocker.join(", ")}</p>
	{/if}
	{#if templateSaved}<p class="mt-2 text-sm" role="status">{templateSaved}</p>{/if}
	{#if error}<p class="mt-3 text-sm text-destructive" role="alert">{error}</p>{/if}

	<div class="mt-4 grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
		<StatCard stat="verpflegung" label={m.reise_allowances()} value={money(calc?.verpflegung_cent)} />
		<StatCard stat="uebernachtung" label={m.reise_overnight()} value={money(calc?.uebernachtung_cent)} />
		<StatCard label={m.reise_tab_mileage()} value={money(calc?.fahrtkosten_cent)} />
		<StatCard label={m.reise_expenses()} value={money(ausgabenCent)} />
		<StatCard label={m.reise_sum()} value={money(calc?.summe_cent)} />
		<StatCard label={m.abrechnung_erstattung()} value={money(calc?.summe_cent)} />
	</div>

	<Tabs.Root bind:value={tab} class="mt-4">
		<div class="max-w-full overflow-x-auto">
			<Tabs.List>
				<Tabs.Trigger value="days">{m.reise_tab_days()}</Tabs.Trigger>
				<Tabs.Trigger value="expenses">{m.reise_tab_expenses()}</Tabs.Trigger>
				<Tabs.Trigger value="fahrten">{m.reise_tab_mileage()}</Tabs.Trigger>
				<Tabs.Trigger value="receipts">{m.reise_tab_receipts()}</Tabs.Trigger>
				<Tabs.Trigger value="history">{m.reise_tab_history()}</Tabs.Trigger>
			</Tabs.List>
		</div>

		<Tabs.Content value="days" class="mt-3 min-w-0">
			<div class="min-w-0 max-w-full overflow-x-auto">
			<div class="min-w-[48rem]">
			<div class="text-muted-foreground grid px-3 text-xs {dayGrid}">
				<span>{m.reise_day()}</span>
				<span>{m.reise_tagesart()}</span>
				<span>{m.reise_land()}</span>
				<span class="text-right">{m.reise_pauschale()}</span>
				<span class="text-right">{m.reise_kuerzung()}</span>
				<span class="text-right">{m.reise_result()}</span>
			</div>
			<Tooltip.Provider delayDuration={200}>
				<div class="mt-2 grid gap-2">
					{#each trip.reisetage as day, index (day.id)}
						{@const tag = tagOf(day.datum)}
						<article class="bg-card rounded-xl border" data-datum={day.datum}>
							<details open={index === 0}>
								<summary class="cursor-pointer list-none px-3 py-2 marker:content-none [&::-webkit-details-marker]:hidden">
									<span class="grid {dayGrid}">
										<span class="font-medium">{formatWhen(day.datum, session.locale, "date")}</span>
										<span>{dayTypeLabel(tag?.tagesart)}</span>
										<span class="min-w-0">{tag?.land_iso ?? ""} {tag?.satzort ?? ""}</span>
										<span class="text-right tabular-nums" data-field="pauschale">{money(tag?.pauschale_cent)}</span>
										<span class="text-right tabular-nums" data-field="kuerzung">{money(tag?.kuerzung_cent)}</span>
										<span class="text-right font-medium tabular-nums">{money(tag?.ergebnis_cent)}</span>
									</span>
								</summary>
								<div class="grid gap-3 border-t px-3 py-3">
									<fieldset class="flex flex-wrap gap-1.5">
										<legend class="sr-only">{m.reise_breakfast()}</legend>
										<label class="border-border has-checked:bg-primary has-checked:text-primary-foreground relative inline-flex items-center rounded-md border px-2.5 py-1 text-xs">
											<input class="absolute inset-0 cursor-pointer opacity-0" type="checkbox" bind:checked={day.fruehstueck_gestellt} />
											{m.reise_breakfast()}
										</label>
										<label class="border-border has-checked:bg-primary has-checked:text-primary-foreground relative inline-flex items-center rounded-md border px-2.5 py-1 text-xs">
											<input class="absolute inset-0 cursor-pointer opacity-0" type="checkbox" bind:checked={day.mittag_gestellt} />
											{m.reise_lunch()}
										</label>
										<label class="border-border has-checked:bg-primary has-checked:text-primary-foreground relative inline-flex items-center rounded-md border px-2.5 py-1 text-xs">
											<input class="absolute inset-0 cursor-pointer opacity-0" type="checkbox" bind:checked={day.abend_gestellt} />
											{m.reise_dinner()}
										</label>
									</fieldset>
									{#if tag?.regel_ids?.length || tag?.land_regel}
										<Tooltip.Root>
											<Tooltip.Trigger class="text-muted-foreground w-fit" aria-label={m.reise_regeln()}>
												<InfoIcon class="size-3.5" />
											</Tooltip.Trigger>
											<Tooltip.Content>
												{tag?.regel_ids?.join(" ") ?? ""} {tag?.land_regel ?? ""}
											</Tooltip.Content>
										</Tooltip.Root>
									{/if}
									{#if tag?.warnungen?.includes("W03")}
										<p class="text-sm" role="status">{m.warn_W03()}</p>
									{/if}
									<div class="grid gap-3 sm:grid-cols-2">
										<label class="grid gap-1 text-sm" for={`copay-${day.datum}`}>
											{m.reise_copay()}
											<Input id={`copay-${day.datum}`} type="number" min="0" step="1" bind:value={day.zuzahlung_mittag} />
										</label>
										<label class="grid gap-1 text-sm" for={`lodging-${day.datum}`}>
											{m.reise_lodging()}
											<NativeSelect id={`lodging-${day.datum}`} class="w-full" bind:value={day.unterkunft} disabled={day.datum === lastDatum}>
												<option value="keine">{m.lodging_keine()}</option>
												<option value="beleg">{m.lodging_beleg()}</option>
												<option value="pauschale">{m.lodging_pauschale()}</option>
												<option value="gestellt">{m.lodging_gestellt()}</option>
												<option value="verkehrsmittel">{m.lodging_verkehrsmittel()}</option>
											</NativeSelect>
										</label>
										<label class="grid gap-1 text-sm" for={`land-${day.datum}`}>
											{m.reise_override()}
											<NativeSelect
												id={`land-${day.datum}`}
												class="w-full"
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
											<label class="grid gap-1 text-sm" for={`place-${day.datum}`}>
												{m.reise_place()}
												<NativeSelect id={`place-${day.datum}`} class="w-full" bind:value={day.satzort_manuell}>
													<option value="">im Übrigen</option>
													{#each places[day.land_manuell] as place (place.satzort)}
														{#if place.satzort}
															<option value={place.satzort}>{place.ort_name || place.satzort}</option>
														{/if}
													{/each}
												</NativeSelect>
											</label>
										{/if}
										<label class="grid gap-1 text-sm sm:col-span-2" for={`reason-${day.datum}`}>
											{m.reise_reason()}
											<Input id={`reason-${day.datum}`} bind:value={day.begruendung} />
										</label>
									</div>
									<p class="text-muted-foreground text-right text-xs tabular-nums" data-field="uebernachtung">
										{m.reise_overnight()}: {money(tag?.uebernachtung_cent)}
									</p>
									{#if tag?.warnungen?.includes("W03") && !day.verpflegung_ausgeschlossen}
										<label class="grid gap-1 text-sm" for={`exclude-${day.datum}`}>
											{m.reise_exclude()}
											<Input id={`exclude-${day.datum}`} bind:value={day.ausschluss_grund} />
										</label>
										<Button variant="outline" size="sm" class="w-fit" type="button" onclick={() => void exclude(day)}>{m.reise_exclude()}</Button>
									{/if}
									<Button variant="outline" size="sm" class="w-fit" type="button" onclick={() => void saveDay(day)}>{m.save()}</Button>
								</div>
							</details>
						</article>
					{/each}
				</div>
			</Tooltip.Provider>
			</div>
			</div>
			{#if overnightReceipts > 0}
				<p class="mt-2 flex items-center justify-between gap-3 px-3 text-sm" data-field="uebernachtung-belege">
					<span>{m.reise_overnight_receipts()}</span>
					<span class="tabular-nums">{money(overnightReceipts)}</span>
				</p>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="expenses" class="mt-3">
			{#if ausgaben.length === 0}
				<p class="text-muted-foreground text-sm">{m.reise_no_expenses()}</p>
			{:else}
				<ul class="grid gap-2">
					{#each ausgaben as row (row.id)}
						<li>
							<a class="bg-card hover:bg-muted flex items-center justify-between gap-3 rounded-xl border px-3 py-2 text-sm" href={p("/ausgaben/:id", { params: { id: row.id } })}>
								<span class="min-w-0 truncate">{kostenartLabel(row.kostenart)}{row.waehrung === "EUR" ? "" : ` · ${row.waehrung}`}</span>
								<span class="shrink-0 tabular-nums">{money(row.betrag_eur_cent)}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="fahrten" class="mt-3">
			{#if fahrten.length === 0}
				<p class="text-muted-foreground text-sm">{m.reise_no_mileage()}</p>
			{:else}
				<ul class="grid gap-2">
					{#each fahrten as fahrt (fahrt.id)}
						<li class="bg-card flex items-center justify-between gap-2 rounded-xl border px-3 py-2 text-sm">
							<span class="min-w-0">{fahrt.start} – {fahrt.ziel} · {fahrt.km} km · <span class="tabular-nums">{money(fahrt.betrag_cent)}</span></span>
							<Button variant="destructive" size="sm" type="button" onclick={() => askDelete(fahrt.id)}>{m.reise_delete()}</Button>
						</li>
					{/each}
				</ul>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="receipts" class="mt-3">
			{#if belege.length === 0}
				<p class="text-muted-foreground text-sm">{m.reise_no_receipts()}</p>
			{:else}
				<ul class="grid gap-2">
					{#each belege as beleg (beleg.id)}
						<li>
							<a class="bg-card hover:bg-muted block rounded-xl border px-3 py-2 text-sm" href={p("/belege/:id", { params: { id: beleg.id } })}>
								{beleg.belegnummer || beleg.id} · {formatWhen(beleg.erstellt_am, session.locale, "date")}
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="history" class="mt-3">
			<ul class="grid gap-2 text-sm">
				<li class="bg-card rounded-xl border px-3 py-2">{m.reise_version()} {trip.version}</li>
				{#each calc?.blocker ?? [] as code (code)}
					<li class="bg-card rounded-xl border px-3 py-2">{warningLabel(code)}</li>
				{/each}
				{#each calc?.warnungen ?? [] as code (code)}
					<li class="bg-card rounded-xl border px-3 py-2">{warningLabel(code)}</li>
				{/each}
				{#if (calc?.blocker?.length ?? 0) === 0 && (calc?.warnungen?.length ?? 0) === 0}
					<li class="text-muted-foreground">{m.reise_no_history()}</li>
				{/if}
			</ul>
		</Tabs.Content>
	</Tabs.Root>
	<div class="h-28 md:hidden"></div>

	<Dialog.Root bind:open={confirmOpen}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>{pendingDelete === "trip" ? m.reise_delete_title() : m.reise_delete()}</Dialog.Title>
				<Dialog.Description>
					{pendingDelete === "trip" ? m.reise_delete_body() : m.reise_delete_fahrt()}
				</Dialog.Description>
			</Dialog.Header>
			<Dialog.Footer>
				<Button variant="outline" type="button" onclick={() => (confirmOpen = false)}>{m.cancel()}</Button>
				<Button variant="destructive" type="button" onclick={() => void confirmDelete()}>{m.reise_delete()}</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>

	<Sheet.Root bind:open={fahrtOpen}>
		<Sheet.Content class="overflow-y-auto">
			<Sheet.Header>
				<Sheet.Title>{m.reise_fahrt()}</Sheet.Title>
			</Sheet.Header>
			<form id="fahrt-form" class="grid gap-2 px-4 pb-4" onsubmit={addFahrt}>
				<label class="grid gap-1 text-sm" for="fahrt-datum">
					{m.reise_day()}
					<DateField id="fahrt-datum" type="date" bind:value={fahrtDatum} required />
				</label>
				<label class="grid gap-1 text-sm" for="fahrt-start">
					{m.reise_start()}
					<Input id="fahrt-start" bind:value={fahrtStart} required />
				</label>
				<label class="grid gap-1 text-sm" for="fahrt-ziel">
					{m.reise_ziel()}
					<Input id="fahrt-ziel" bind:value={fahrtZiel} required />
				</label>
				<label class="grid gap-1 text-sm" for="fahrt-km">
					{m.reise_km()}
					<Input id="fahrt-km" type="number" min="1" max="100000" bind:value={fahrtKm} required />
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
		</Sheet.Content>
	</Sheet.Root>

	<Sheet.Root bind:open={vorlageOpen}>
		<Sheet.Content>
			<Sheet.Header>
				<Sheet.Title>{m.reise_template()}</Sheet.Title>
			</Sheet.Header>
			<div class="grid gap-2 px-4 pb-4">
				<label class="grid gap-1 text-sm" for="vorlage-name">
					{m.reise_template_name()}
					<Input id="vorlage-name" bind:value={templateName} />
				</label>
				<Button variant="outline" type="button" onclick={() => void saveTemplate()}>{m.reise_save_template()}</Button>
			</div>
		</Sheet.Content>
	</Sheet.Root>
{/if}
