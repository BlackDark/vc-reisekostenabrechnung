<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import { euro } from "$lib/money";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, route } from "../router";

	type Row = components["schemas"]["Abrechnung"];
	type Punkt = components["schemas"]["PruefPunkt"];
	type Exp = components["schemas"]["Export"];
	type Reise = components["schemas"]["Reise"];
	type Vorschuss = components["schemas"]["Vorschuss"];

	const id = $derived(route.params.id ?? "");

	let row = $state<Row | null>(null);
	let reisen = $state<Reise[]>([]);
	let advances = $state<Vorschuss[]>([]);
	let selectedReisen = $state<string[]>([]);
	let selectedAdvances = $state<string[]>([]);
	let blocker = $state<Punkt[]>([]);
	let warnungen = $state<Punkt[]>([]);
	let erstattung = $state(0);
	let vorschuss = $state(0);
	let auszahlung = $state(0);
	let exporte = $state<Exp[]>([]);
	let checked = $state<Record<string, boolean>>({});
	let error = $state("");
	let busy = $state(false);
	let paidDate = $state("");
	let reason = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer || !id) return;
		void load();
	});

	function keyOf(point: Punkt): string {
		return `${point.code}\t${point.objekt_id}`;
	}

	function statusLabel(status: string): string {
		if (status === "eingereicht") return m.abrechnung_status_eingereicht();
		if (status === "bezahlt") return m.abrechnung_status_bezahlt();
		return m.abrechnung_status_entwurf();
	}

	function warnLabel(code: string): string {
		const labels: Record<string, () => string> = {
			B01: m.warn_B01,
			B02: m.warn_B02,
			B03: m.warn_B03,
			B04: m.warn_B04,
			B05: m.warn_B05,
			B06: m.warn_B06,
			W01: m.warn_W01,
			W02: m.warn_W02,
			W05: m.warn_W05,
			W06: m.warn_W06,
			W07: m.warn_W07,
			W08: m.warn_W08,
			W09: m.warn_W09,
			W10: m.warn_W10,
			W11: m.warn_W11,
			W13: m.warn_W13,
			W14: m.warn_W14,
		};
		const fn = labels[code];
		return fn ? fn() : code;
	}

	async function load() {
		const [claim, trips, vors, check, files] = await Promise.all([
			api.GET("/api/v1/abrechnungen/{id}", { params: { path: { id } } }),
			api.GET("/api/v1/reisen"),
			api.GET("/api/v1/vorschuesse"),
			api.GET("/api/v1/abrechnungen/{id}/pruefung", { params: { path: { id } } }),
			api.GET("/api/v1/abrechnungen/{id}/exporte", { params: { path: { id } } }),
		]);
		row = claim.data ?? null;
		reisen = (trips.data?.items ?? []).filter((item) => {
			const ids = new Set([...(row?.reise_ids ?? []), ...(row?.vorschlaege_reise ?? [])]);
			return ids.has(item.id);
		});
		advances = (vors.data?.items ?? []).filter((item) => {
			const ids = new Set([...(row?.vorschuss_ids ?? []), ...(row?.vorschlaege_vorschuss ?? [])]);
			return ids.has(item.id);
		});
		selectedReisen = [...(row?.reise_ids ?? [])];
		selectedAdvances = [...(row?.vorschuss_ids ?? [])];
		blocker = check.data?.blocker ?? [];
		warnungen = check.data?.warnungen ?? [];
		erstattung = check.data?.erstattung_cent ?? 0;
		vorschuss = check.data?.vorschuss_cent ?? 0;
		auszahlung = check.data?.auszahlung_cent ?? 0;
		exporte = files.data?.items ?? [];
		if (row?.einreichung_fehler) error = row.einreichung_fehler;
	}

	function toggle(list: string[], value: string, on: boolean): string[] {
		if (on) return list.includes(value) ? list : [...list, value];
		return list.filter((item) => item !== value);
	}

	async function saveSelection() {
		if (!row) return;
		error = "";
		const trips = await api.PUT("/api/v1/abrechnungen/{id}/reisen", {
			params: { path: { id }, header: { "If-Match": String(row.version) } },
			body: { ids: selectedReisen },
		});
		if (!trips.data) {
			error = m.save_failed();
			return;
		}
		row = trips.data;
		const adv = await api.PUT("/api/v1/abrechnungen/{id}/vorschuesse", {
			params: { path: { id }, header: { "If-Match": String(row.version) } },
			body: { ids: selectedAdvances },
		});
		if (!adv.data) {
			error = m.save_failed();
			return;
		}
		row = adv.data;
		await load();
	}

	async function submit() {
		if (!row) return;
		busy = true;
		error = "";
		await saveSelection();
		if (!row || error) {
			busy = false;
			return;
		}
		const quittierte_warnungen = warnungen
			.filter((point) => checked[keyOf(point)])
			.map((point) => ({ code: point.code, objekt_id: point.objekt_id }));
		const res = await api.POST("/api/v1/abrechnungen/{id}/einreichen", {
			params: { path: { id }, header: { "If-Match": String(row.version) } },
			body: { quittierte_warnungen },
		});
		if (!res.data) {
			error = m.abrechnung_submit_failed();
			busy = false;
			await load();
			return;
		}
		row = res.data;
		for (let i = 0; i < 40 && row.einreichung_laeuft; i++) {
			await new Promise((resolve) => setTimeout(resolve, 1000));
			const again = await api.GET("/api/v1/abrechnungen/{id}", { params: { path: { id } } });
			if (again.data) row = again.data;
		}
		busy = false;
		await load();
	}

	async function markPaid(event: SubmitEvent) {
		event.preventDefault();
		if (!row) return;
		error = "";
		const res = await api.POST("/api/v1/abrechnungen/{id}/bezahlt", {
			params: { path: { id }, header: { "If-Match": String(row.version) } },
			body: { bezahlt_am: paidDate },
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		row = res.data;
	}

	async function withdraw(event: SubmitEvent) {
		event.preventDefault();
		if (!row) return;
		error = "";
		const res = await api.POST("/api/v1/abrechnungen/{id}/bezahlt-zuruecknehmen", {
			params: { path: { id }, header: { "If-Match": String(row.version) } },
			body: { grund: reason },
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		reason = "";
		row = res.data;
	}

	async function unlock(event: SubmitEvent) {
		event.preventDefault();
		if (!row) return;
		error = "";
		const res = await api.POST("/api/v1/abrechnungen/{id}/entsperren", {
			params: { path: { id }, header: { "If-Match": String(row.version) } },
			body: { grund: reason },
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		reason = "";
		row = res.data;
		await load();
	}

	const ready = $derived(exporte.filter((item) => item.status === "fertig"));

	async function download(url: string, name: string) {
		error = "";
		const res = await fetch(url);
		if (!res.ok) {
			error = m.abrechnung_submit_failed();
			return;
		}
		const blob = await res.blob();
		const href = URL.createObjectURL(blob);
		const link = document.createElement("a");
		link.href = href;
		link.download = name;
		document.body.append(link);
		link.click();
		link.remove();
		URL.revokeObjectURL(href);
	}
</script>

{#if row}
	<h1 class="text-2xl font-semibold">{row.titel}</h1>
	<p class="mt-2 text-sm" data-status={row.status}>
		{row.von} – {row.bis} · {statusLabel(row.status)}
		{#if row.abrechnungsnummer} · {row.abrechnungsnummer}{/if}
		{#if row.aktuelle_export_version > 0} · v{row.aktuelle_export_version}{/if}
	</p>
	<p class="mt-3 text-sm">
		{m.abrechnung_erstattung()}: {euro(erstattung, session.locale)} EUR ·
		{m.abrechnung_vorschuss_sum()}: {euro(vorschuss, session.locale)} EUR ·
		{m.abrechnung_auszahlung()}: {euro(auszahlung, session.locale)} EUR
	</p>

	{#if row.status === "entwurf" && !row.einreichung_laeuft}
		<section class="mt-4 grid gap-3">
			<h2 class="text-lg font-medium">{m.abrechnung_reisen()}</h2>
			{#each reisen as trip (trip.id)}
				<label class="flex items-center gap-2 text-sm">
					<input
						type="checkbox"
						checked={selectedReisen.includes(trip.id)}
						onchange={(event) => {
							const on = (event.currentTarget as HTMLInputElement).checked;
							selectedReisen = toggle(selectedReisen, trip.id, on);
						}}
					/>
					{trip.anlass}
				</label>
			{/each}
			<h2 class="text-lg font-medium">{m.abrechnung_vorschuesse()}</h2>
			{#each advances as item (item.id)}
				<label class="flex items-center gap-2 text-sm">
					<input
						type="checkbox"
						checked={selectedAdvances.includes(item.id)}
						onchange={(event) => {
							const on = (event.currentTarget as HTMLInputElement).checked;
							selectedAdvances = toggle(selectedAdvances, item.id, on);
						}}
					/>
					{item.datum} · {euro(item.betrag_cent, session.locale)} EUR
				</label>
			{/each}
			<button class="w-fit rounded border px-3 py-3 text-sm" type="button" onclick={saveSelection}>
				{m.save()}
			</button>
		</section>
	{/if}

	{#if blocker.length > 0}
		<ul class="mt-4 grid gap-1 text-sm" data-testid="abrechnung-blocker">
			{#each blocker as point (`${point.code}-${point.objekt_id}`)}
				<li>{point.code}: {warnLabel(point.code)}</li>
			{/each}
		</ul>
	{/if}
	{#if warnungen.length > 0 && row.status === "entwurf"}
		<ul class="mt-4 grid gap-2 text-sm">
			{#each warnungen as point (`${point.code}-${point.objekt_id}`)}
				<li>
					<label class="flex items-start gap-2">
						<input
							type="checkbox"
							data-warn={point.code}
							checked={checked[keyOf(point)] === true}
							onchange={(event) => {
								const on = (event.currentTarget as HTMLInputElement).checked;
							checked = { ...checked, [keyOf(point)]: on };
							}}
						/>
						<span>{point.code}: {warnLabel(point.code)}</span>
					</label>
				</li>
			{/each}
		</ul>
	{/if}

	{#if error}
		<p class="mt-3 text-sm" role="alert">{error}</p>
	{/if}
	{#if row.einreichung_laeuft || busy}
		<p class="mt-3 text-sm" role="status">{m.abrechnung_submitting()}</p>
	{/if}
	{#if row.status === "entwurf" && !row.einreichung_laeuft}
		<button
			class="mt-4 rounded bg-blue-800 px-3 py-3 text-white"
			type="button"
			data-testid="abrechnung-submit"
			disabled={busy || blocker.length > 0}
			onclick={submit}
		>
			{m.abrechnung_submit()}
		</button>
	{/if}

	{#if ready.length > 0}
		<ul class="mt-4 grid gap-2 text-sm">
			{#each ready as item (item.id)}
				<li data-export-version={item.version}>
					{m.abrechnung_version()} {item.version}
					{#if item.ersetzt_durch_version} · {m.abrechnung_replaced()}{/if}
					<button
						class="ml-2 underline"
						type="button"
						data-testid="abrechnung-pdf"
						onclick={() => download(`/api/v1/exporte/${item.id}/pdf`, `abrechnung-v${item.version}.pdf`)}
					>
						PDF
					</button>
					<button
						class="ml-2 underline"
						type="button"
						onclick={() => download(`/api/v1/exporte/${item.id}/zip`, `abrechnung-v${item.version}.zip`)}
					>
						ZIP
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	{#if row.status === "eingereicht"}
		<form class="mt-4 grid gap-2" onsubmit={markPaid}>
			<label class="grid gap-1 text-sm">
				{m.abrechnung_paid_date()}
				<input id="bezahlt-am" class="rounded border px-3 py-3" type="date" bind:value={paidDate} required />
			</label>
			<button class="w-fit rounded bg-blue-800 px-3 py-3 text-white" type="submit" data-testid="abrechnung-paid">
				{m.abrechnung_mark_paid()}
			</button>
		</form>
		<form class="mt-4 grid gap-2" onsubmit={unlock}>
			<label class="grid gap-1 text-sm">
				{m.abrechnung_reason()}
				<input id="entsperr-grund" class="rounded border px-3 py-3" bind:value={reason} minlength={10} required />
			</label>
			<button class="w-fit rounded border px-3 py-3" type="submit" data-testid="abrechnung-unlock">
				{m.abrechnung_unlock()}
			</button>
		</form>
	{/if}
	{#if row.status === "bezahlt"}
		<form class="mt-4 grid gap-2" onsubmit={withdraw}>
			<label class="grid gap-1 text-sm">
				{m.abrechnung_reason()}
				<input id="zurueck-grund" class="rounded border px-3 py-3" bind:value={reason} required />
			</label>
			<button class="w-fit rounded border px-3 py-3" type="submit" data-testid="abrechnung-withdraw">
				{m.abrechnung_withdraw()}
			</button>
		</form>
	{/if}
{/if}
