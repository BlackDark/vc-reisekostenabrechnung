<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import { euro } from "$lib/money";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	type Row = components["schemas"]["Vorschuss"];
	type Arbeitgeber = { id: string; name: string };

	let rows = $state<Row[]>([]);
	let employers = $state<Arbeitgeber[]>([]);
	let arbeitgeber = $state("");
	let datum = $state("");
	let betrag = $state("");
	let notiz = $state("");
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const [list, ag] = await Promise.all([
			api.GET("/api/v1/vorschuesse"),
			api.GET("/api/v1/arbeitgeber"),
		]);
		rows = list.data?.items ?? [];
		employers = (ag.data?.items ?? []).map((row) => ({ id: row.id, name: row.name }));
		if (!arbeitgeber && employers[0]) arbeitgeber = employers[0].id;
	}

	function cents(raw: string): number {
		const n = Number(raw.trim().replace(/\s/g, "").replace(",", "."));
		if (!Number.isFinite(n)) return 0;
		return Math.round(n * 100);
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.POST("/api/v1/vorschuesse", {
			body: { arbeitgeber_id: arbeitgeber, datum, betrag_cent: cents(betrag), notiz },
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		betrag = "";
		notiz = "";
		await load();
	}
</script>

<h1 class="text-2xl font-semibold">{m.vorschuss_title()}</h1>
{#if rows.length === 0}
	<p class="mt-3 text-sm">{m.vorschuss_empty()}</p>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each rows as row (row.id)}
			<li class="rounded border px-3 py-3 text-sm">
				{row.datum} · {euro(row.betrag_cent, session.locale)} EUR
				{#if row.notiz} · {row.notiz}{/if}
			</li>
		{/each}
	</ul>
{/if}
<form class="mt-6 grid gap-3" onsubmit={save}>
	<label class="grid gap-1 text-sm">
		{m.reise_employer()}
		<select class="rounded border px-3 py-3" bind:value={arbeitgeber}>
			{#each employers as row (row.id)}
				<option value={row.id}>{row.name}</option>
			{/each}
		</select>
	</label>
	<label class="grid gap-1 text-sm">
		{m.vorschuss_datum()}
		<input class="rounded border px-3 py-3" type="date" bind:value={datum} required />
	</label>
	<label class="grid gap-1 text-sm">
		{m.vorschuss_betrag()}
		<input class="rounded border px-3 py-3" inputmode="decimal" bind:value={betrag} required />
	</label>
	<label class="grid gap-1 text-sm">
		{m.vorschuss_notiz()}
		<input class="rounded border px-3 py-3" bind:value={notiz} />
	</label>
	{#if error}
		<p class="text-sm" role="alert">{error}</p>
	{/if}
	<button class="rounded bg-blue-800 px-3 py-3 text-white" type="submit">{m.create()}</button>
</form>
