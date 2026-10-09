<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import { euro } from "$lib/money";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p, route } from "../router";

	type Ausgabe = components["schemas"]["Ausgabe"];
	type Share = { satz: number; steuerland: string; brutto: string };

	const editing = $derived(route.pathname.startsWith("/ausgaben/"));
	const reiseId = $derived(editing ? "" : (route.params.id ?? ""));
	const ausgabeId = $derived(editing ? (route.params.id ?? "") : "");
	const belegFromQuery = $derived(String(route.search.beleg ?? ""));

	let loaded = $state<Ausgabe | null>(null);
	let error = $state("");
	let kostenart = $state("fahrtkosten");
	let datum = $state("");
	let leistender = $state("");
	let betrag = $state("");
	let waehrung = $state("EUR");
	let aufArbeitgeber = $state(false);
	let anteile = $state<Share[]>([{ satz: 1900, steuerland: "DE", brutto: "" }]);
	let anlass = $state("");
	let ort = $state("");
	let bewirtender = $state("");
	let teilnehmer = $state<{ name: string; firma: string }[]>([{ name: "", firma: "" }]);
	let tse = $state(false);
	let eigen = $state(false);
	let eigenGrund = $state("");
	let eigenWer = $state("");
	let eigenArt = $state("");
	let saving = $state(false);
	let empfaenger = $state("");
	let employerName = $state("");
	let kiMark = $state(false);
	let kiText = $state("");
	let kiLoaded = $state(false);

	const payeeWarn = $derived(
		empfaenger.trim() !== "" &&
			employerName.trim() !== "" &&
			empfaenger.trim().toLocaleLowerCase() !== employerName.trim().toLocaleLowerCase(),
	);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		const trip = reiseId;
		if (!session.nutzer || !trip) return;
		void api.GET("/api/v1/reisen/{id}", { params: { path: { id: trip } } }).then(async (res) => {
			const ag = res.data?.arbeitgeber_id;
			if (!ag) return;
			const one = await api.GET("/api/v1/arbeitgeber/{id}", { params: { path: { id: ag } } });
			if (one.data?.name && trip === reiseId) employerName = one.data.name;
		});
	});

	$effect(() => {
		const beleg = belegFromQuery;
		if (!session.nutzer || editing || String(route.search.vorschlag ?? "") !== "1" || !beleg || kiLoaded) return;
		kiLoaded = true;
		void api.GET("/api/v1/belege/{id}/ki", { params: { path: { id: beleg } } }).then((res) => {
			const v = res.data?.vorschlag;
			if (res.data?.status !== "vorschlag" || !v) return;
			kiMark = true;
			if (v.leistender) leistender = v.leistender;
			if (v.datum) datum = v.datum;
			if (v.waehrung) waehrung = v.waehrung;
			if (v.betrag_brutto_cent) betrag = (v.betrag_brutto_cent / 100).toFixed(2);
			if (v.kostenart) kostenart = v.kostenart;
			if (v.empfaenger_name) empfaenger = v.empfaenger_name;
			kiText = v.volltext ?? "";
			if (v.steueranteile && v.steueranteile.length > 0) {
				anteile = v.steueranteile.map((row) => ({
					satz: row.satz,
					steuerland: "DE",
					brutto: ((row.brutto_cent ?? 0) / 100).toFixed(2),
				}));
			}
		});
	});

	$effect(() => {
		if (!session.nutzer || !editing || !ausgabeId) return;
		void api.GET("/api/v1/ausgaben/{id}", { params: { path: { id: ausgabeId } } }).then((res) => {
			if (!res.data) {
				error = m.save_failed();
				return;
			}
			loaded = res.data;
		});
	});

	function cents(raw: string): number {
		const n = Number(raw.trim().replace(/\s/g, "").replace(",", "."));
		if (!Number.isFinite(n)) return 0;
		return Math.round(n * 100);
	}

	function yearOf(value: string): number {
		const y = Number(value.slice(0, 4));
		return Number.isFinite(y) && y > 0 ? y : new Date().getFullYear();
	}

	async function applyHelper(art: "gastronomie" | "hotel") {
		const res = await api.POST("/api/v1/mwst-helfer", {
			body: { art, betrag_cent: cents(betrag), jahr: yearOf(datum) },
		});
		const rows = res.data?.anteile ?? [];
		if (rows.length === 0) {
			error = m.save_failed();
			return;
		}
		anteile = rows.map((row) => ({
			satz: row.satz,
			steuerland: row.steuerland,
			brutto: (row.brutto_cent / 100).toFixed(2),
		}));
	}

	async function save() {
		saving = true;
		error = "";
		if (kiMark && belegFromQuery) {
			const text = await api.POST("/api/v1/belege/{id}/texte", {
				params: { path: { id: belegFromQuery } },
				body: {
					volltext: kiText,
					felder: {
						leistender,
						datum,
						waehrung,
						betrag_brutto_cent: cents(betrag),
						kostenart,
						empfaenger_name: empfaenger,
						volltext: kiText,
						steueranteile: anteile.map((row) => ({
							satz: Number(row.satz),
							brutto_cent: cents(row.brutto || betrag),
						})),
					},
				},
			});
			if (!text.response.ok) {
				error = m.save_failed();
				saving = false;
				return;
			}
		}
		const body = {
			kostenart,
			datum,
			waehrung,
			betrag_cent: cents(betrag),
			leistender,
			empfaenger,
			rechnung_auf_arbeitgeber: aufArbeitgeber,
			rechnungsart: eigen ? "eigenbeleg" : "kleinbetragsrechnung",
			tse_beleg: tse,
			anteile: anteile.map((row) => ({
				satz: Number(row.satz),
				steuerland: row.steuerland || "DE",
				brutto_cent: cents(row.brutto || betrag),
			})),
			beleg_ids: belegFromQuery ? [belegFromQuery] : loaded?.beleg_ids ?? [],
			bewirtung:
				kostenart === "bewirtung"
					? {
							anlass,
							ort,
							bewirtender,
							teilnehmer: teilnehmer.filter((row) => row.name.trim()),
						}
					: undefined,
		};
		const res = editing
			? await api.PATCH("/api/v1/ausgaben/{id}", {
					params: { path: { id: ausgabeId }, header: { "If-Match": String(loaded?.version ?? "") } },
					body,
				})
			: await api.POST("/api/v1/reisen/{id}/ausgaben", { params: { path: { id: reiseId } }, body });
		if (!res.data) {
			error = m.save_failed();
			saving = false;
			return;
		}
		if (eigen) {
			await api.PUT("/api/v1/ausgaben/{id}/eigenbeleg", {
				params: { path: { id: res.data.id }, header: { "If-Match": String(res.data.version) } },
				body: { grund: eigenGrund, zahlungsempfaenger: eigenWer, art: eigenArt },
			});
		}
		saving = false;
		await navigate("/ausgaben/:id", { params: { id: res.data.id } });
	}

	async function confirmBewirtung() {
		if (!loaded) return;
		const res = await api.POST("/api/v1/ausgaben/{id}/bewirtung/bestaetigen", {
			params: { path: { id: loaded.id }, header: { "If-Match": String(loaded.version) } },
		});
		if (res.data) loaded = res.data;
		else error = m.save_failed();
	}

	function money(centsValue: number | undefined): string {
		return euro(centsValue ?? 0, session.locale);
	}
</script>

<p><a class="text-sm underline" href={p("/reisen")}>{m.back()}</a></p>
<h1 class="mt-2 text-2xl font-semibold">{m.ausgabe_title()}</h1>
{#if kiMark}
	<p class="mt-2 text-sm" data-testid="ki-marke">{m.ki_mark()}: {m.ki_suggestion()}</p>
{/if}
{#if payeeWarn}
	<p class="mt-2 text-sm" role="status" data-testid="warn-w02">{m.warn_W02()}</p>
{/if}

{#if loaded}
	<p class="mt-3 text-sm" data-testid="betrag-eur">{m.ausgabe_betrag()}: {money(loaded.betrag_eur_cent)} EUR</p>
	{#if loaded.kurs}
		<p class="text-sm">{loaded.waehrung} {loaded.kurs} ({loaded.kurs_datum})</p>
	{/if}
	{#if loaded.ust_kurs}
		<p class="text-sm">{m.ausgabe_ust_kurs()}: {loaded.ust_kurs}</p>
	{/if}
	<ul class="mt-2 grid gap-1 text-sm">
		{#each loaded.anteile as share, i (i)}
			<li data-testid="steuer-anteil">
				{share.satz / 100}% {share.steuerland} {money(share.brutto_eur_cent)} / {money(share.steuer_eur_cent)}
			</li>
		{/each}
	</ul>
	{#if loaded.warnungen}
		<ul class="mt-2 grid gap-1 text-sm">
			{#each loaded.warnungen as code (code)}
				<li role="status">{code}</li>
			{/each}
		</ul>
	{/if}
	{#if loaded.kostenart === "bewirtung" && !loaded.bewirtung?.bestaetigt_am}
		<button class="mt-3 rounded border px-3 py-3" type="button" onclick={() => void confirmBewirtung()}>{m.ausgabe_confirm()}</button>
	{/if}
{/if}

<form class="mt-4 grid gap-3" onsubmit={(event) => { event.preventDefault(); void save(); }}>
	<label class="grid gap-1 text-sm">
		{m.ausgabe_kostenart()}
		<select id="ausgabe-kostenart" class="rounded border px-3 py-3" bind:value={kostenart}>
			<option value="fahrtkosten">{m.kostenart_fahrtkosten()}</option>
			<option value="verpflegung">{m.kostenart_verpflegung()}</option>
			<option value="uebernachtung">{m.kostenart_uebernachtung()}</option>
			<option value="reisenebenkosten">{m.kostenart_reisenebenkosten()}</option>
			<option value="bewirtung">{m.kostenart_bewirtung()}</option>
		</select>
	</label>
	<label class="grid gap-1 text-sm">
		{m.ausgabe_datum()}
		<input id="ausgabe-datum" class="rounded border px-3 py-3" type="date" bind:value={datum} required />
	</label>
	<label class="grid gap-1 text-sm">
		{m.ausgabe_leistender()}
		<input id="ausgabe-leistender" class="rounded border px-3 py-3" bind:value={leistender} />
	</label>
	<label class="grid gap-1 text-sm">
		{m.ausgabe_empfaenger()}
		<input id="ausgabe-empfaenger" class="rounded border px-3 py-3" bind:value={empfaenger} />
	</label>
	<label class="grid gap-1 text-sm">
		{m.ausgabe_betrag()}
		<input id="ausgabe-betrag" class="rounded border px-3 py-3" inputmode="decimal" bind:value={betrag} required />
	</label>
	<label class="grid gap-1 text-sm">
		{m.ausgabe_waehrung()}
		<select id="ausgabe-waehrung" class="rounded border px-3 py-3" bind:value={waehrung}>
			<option>EUR</option>
			<option>USD</option>
			<option>GBP</option>
			<option>CHF</option>
		</select>
	</label>
	<label class="flex items-center gap-2 text-sm">
		<input type="checkbox" bind:checked={aufArbeitgeber} />
		{m.ausgabe_employer()}
	</label>
	{#if kostenart === "reisenebenkosten"}
		<p class="text-sm">{m.ausgabe_neben_hint()}</p>
	{/if}
	{#if kostenart === "bewirtung"}
		<p class="text-sm">{m.ausgabe_privat_hint()}</p>
		<label class="grid gap-1 text-sm">
			{m.ausgabe_bewirtung_anlass()}
			<input class="rounded border px-3 py-3" bind:value={anlass} />
		</label>
		<label class="grid gap-1 text-sm">
			{m.ausgabe_bewirtung_ort()}
			<input class="rounded border px-3 py-3" bind:value={ort} />
		</label>
		<label class="grid gap-1 text-sm">
			{m.ausgabe_bewirtung_wer()}
			<input class="rounded border px-3 py-3" bind:value={bewirtender} />
		</label>
		<p class="text-sm font-medium">{m.ausgabe_teilnehmer()}</p>
		{#each teilnehmer as person, i (i)}
			<div class="grid gap-2 sm:grid-cols-2">
				<input class="rounded border px-3 py-3" placeholder={m.ausgabe_teilnehmer_name()} bind:value={person.name} />
				<input class="rounded border px-3 py-3" placeholder={m.ausgabe_teilnehmer_firma()} bind:value={person.firma} />
			</div>
		{/each}
		<button class="rounded border px-3 py-3 text-sm" type="button" onclick={() => { teilnehmer = [...teilnehmer, { name: "", firma: "" }]; }}>{m.ausgabe_teilnehmer_add()}</button>
		<label class="flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={tse} />
			{m.ausgabe_tse()}
		</label>
	{/if}
	<fieldset class="grid gap-2">
		<legend class="text-sm font-medium">{m.ausgabe_vat()}</legend>
		{#each anteile as share, i (i)}
			<div class="grid gap-2 sm:grid-cols-3">
				<label class="grid gap-1 text-sm">
					{m.ausgabe_satz()}
					<input class="rounded border px-3 py-3" type="number" bind:value={share.satz} />
				</label>
				<label class="grid gap-1 text-sm">
					{m.ausgabe_betrag()}
					<input class="rounded border px-3 py-3" inputmode="decimal" bind:value={share.brutto} />
				</label>
				<label class="grid gap-1 text-sm">
					Land
					<input class="rounded border px-3 py-3" bind:value={share.steuerland} />
				</label>
			</div>
		{/each}
		<div class="flex flex-wrap gap-2">
			<button class="rounded border px-3 py-3 text-sm" type="button" onclick={() => { anteile = [...anteile, { satz: 700, steuerland: "DE", brutto: "" }]; }}>{m.ausgabe_add_vat()}</button>
			<button class="rounded border px-3 py-3 text-sm" type="button" onclick={() => void applyHelper("gastronomie")}>{m.ausgabe_gastro()}</button>
			<button class="rounded border px-3 py-3 text-sm" type="button" onclick={() => void applyHelper("hotel")}>{m.ausgabe_hotel()}</button>
		</div>
	</fieldset>
	<label class="flex items-center gap-2 text-sm">
		<input type="checkbox" bind:checked={eigen} />
		{m.ausgabe_eigenbeleg()}
	</label>
	{#if eigen}
		<label class="grid gap-1 text-sm">
			{m.ausgabe_eigen_grund()}
			<input class="rounded border px-3 py-3" bind:value={eigenGrund} />
		</label>
		<label class="grid gap-1 text-sm">
			{m.ausgabe_eigen_wer()}
			<input class="rounded border px-3 py-3" bind:value={eigenWer} />
		</label>
		<label class="grid gap-1 text-sm">
			{m.ausgabe_eigen_art()}
			<input class="rounded border px-3 py-3" bind:value={eigenArt} />
		</label>
	{/if}
	{#if error}
		<p class="text-sm" role="alert">{error}</p>
	{/if}
	<button id="ausgabe-save" class="rounded bg-blue-800 px-3 py-3 text-white" type="submit" disabled={saving}>{m.create()}</button>
</form>
