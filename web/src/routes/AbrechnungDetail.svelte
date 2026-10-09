<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import DateField from "$lib/components/date-field.svelte";
	import StatCard from "$lib/components/stat-card.svelte";
	import StatusBadge from "$lib/components/status-badge.svelte";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { formatWhen } from "$lib/dates";
	import { euroAmount } from "$lib/money";
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
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h1 class="truncate text-2xl font-semibold tracking-tight">{row.titel}</h1>
			<p class="text-muted-foreground mt-1 text-sm">
				{formatWhen(row.von, session.locale, "date")} – {formatWhen(row.bis, session.locale, "date")}
				{#if row.abrechnungsnummer} · {row.abrechnungsnummer}{/if}
				{#if row.aktuelle_export_version > 0} · v{row.aktuelle_export_version}{/if}
			</p>
		</div>
		<StatusBadge status={row.status} />
	</div>

	<div class="mt-4 grid grid-cols-3 gap-2">
		<StatCard label={m.abrechnung_erstattung()} value={euroAmount(erstattung, session.locale)} />
		<StatCard label={m.abrechnung_vorschuss_sum()} value={euroAmount(vorschuss, session.locale)} />
		<StatCard label={m.abrechnung_auszahlung()} value={euroAmount(auszahlung, session.locale)} />
	</div>

	<div class="mt-4 grid items-start gap-4 md:grid-cols-2">
		<div class="grid gap-4">
			{#if row.status === "entwurf" && !row.einreichung_laeuft}
				<section class="grid gap-2">
					<h2 class="text-sm font-medium">{m.abrechnung_reisen()}</h2>
					{#each reisen as trip (trip.id)}
						<label class="bg-card flex items-center gap-2 rounded-lg border px-3 py-2 text-sm">
							<input
								type="checkbox"
								checked={selectedReisen.includes(trip.id)}
								onchange={(event) => {
									const on = (event.currentTarget as HTMLInputElement).checked;
									selectedReisen = toggle(selectedReisen, trip.id, on);
								}}
							/>
							<span class="min-w-0">{trip.anlass}</span>
						</label>
					{/each}
					<h2 class="mt-2 text-sm font-medium">{m.abrechnung_vorschuesse()}</h2>
					{#each advances as item (item.id)}
						<label class="bg-card flex items-center gap-2 rounded-lg border px-3 py-2 text-sm">
							<input
								type="checkbox"
								checked={selectedAdvances.includes(item.id)}
								onchange={(event) => {
									const on = (event.currentTarget as HTMLInputElement).checked;
									selectedAdvances = toggle(selectedAdvances, item.id, on);
								}}
							/>
							<span>{formatWhen(item.datum, session.locale, "date")}</span>
							<span class="ml-auto tabular-nums">{euroAmount(item.betrag_cent, session.locale)}</span>
						</label>
					{/each}
					<Button variant="outline" size="sm" class="w-fit" type="button" onclick={saveSelection}>
						{m.save()}
					</Button>
				</section>
			{/if}

			{#if blocker.length > 0}
				<ul class="grid gap-1 text-sm" data-testid="abrechnung-blocker">
					{#each blocker as point (`${point.code}-${point.objekt_id}`)}
						<li class="bg-card rounded-lg border px-3 py-2">{point.code}: {warnLabel(point.code)}</li>
					{/each}
				</ul>
			{/if}
			{#if warnungen.length > 0 && row.status === "entwurf"}
				<ul class="grid gap-2 text-sm">
					{#each warnungen as point (`${point.code}-${point.objekt_id}`)}
						<li>
							<label class="bg-card flex items-start gap-2 rounded-lg border px-3 py-2">
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
		</div>

		<div class="grid gap-3">
			{#if error}
				<p class="text-sm" role="alert">{error}</p>
			{/if}
			{#if row.einreichung_laeuft || busy}
				<p class="text-sm" role="status">{m.abrechnung_submitting()}</p>
			{/if}
			{#if row.status === "entwurf" && !row.einreichung_laeuft}
				<div class="bg-background/95 sticky bottom-16 z-30 -mx-4 border-t px-4 py-3 md:static md:mx-0 md:border-0 md:bg-transparent md:p-0">
					<Button
						type="button"
						data-testid="abrechnung-submit"
						disabled={busy || blocker.length > 0}
						onclick={submit}
					>
						{m.abrechnung_submit()}
					</Button>
				</div>
			{/if}

			{#if ready.length > 0}
				<ul class="grid gap-2 text-sm">
					{#each ready as item (item.id)}
						<li class="bg-card flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2" data-export-version={item.version}>
							<span>
								{m.abrechnung_version()} {item.version}
								{#if item.ersetzt_durch_version} · {m.abrechnung_replaced()}{/if}
							</span>
							<Button
								variant="link"
								size="sm"
								type="button"
								data-testid="abrechnung-pdf"
								onclick={() => download(`/api/v1/exporte/${item.id}/pdf`, `abrechnung-v${item.version}.pdf`)}
							>
								PDF
							</Button>
							<Button
								variant="link"
								size="sm"
								type="button"
								onclick={() => download(`/api/v1/exporte/${item.id}/zip`, `abrechnung-v${item.version}.zip`)}
							>
								ZIP
							</Button>
						</li>
					{/each}
				</ul>
			{/if}

			{#if row.status === "eingereicht"}
				<form class="grid gap-2" onsubmit={markPaid}>
					<label class="grid gap-1 text-sm" for="bezahlt-am">
						{m.abrechnung_paid_date()}
						<DateField id="bezahlt-am" type="date" bind:value={paidDate} required />
					</label>
					<Button class="w-fit" type="submit" data-testid="abrechnung-paid">
						{m.abrechnung_mark_paid()}
					</Button>
				</form>
				<form class="grid gap-2" onsubmit={unlock}>
					<label class="grid gap-1 text-sm" for="entsperr-grund">
						{m.abrechnung_reason()}
						<Input id="entsperr-grund" bind:value={reason} minlength={10} required />
					</label>
					<Button variant="outline" class="w-fit" type="submit" data-testid="abrechnung-unlock">
						{m.abrechnung_unlock()}
					</Button>
				</form>
			{/if}
			{#if row.status === "bezahlt"}
				<form class="grid gap-2" onsubmit={withdraw}>
					<label class="grid gap-1 text-sm" for="zurueck-grund">
						{m.abrechnung_reason()}
						<Input id="zurueck-grund" bind:value={reason} required />
					</label>
					<Button variant="outline" class="w-fit" type="submit" data-testid="abrechnung-withdraw">
						{m.abrechnung_withdraw()}
					</Button>
				</form>
			{/if}
		</div>
	</div>
{/if}
