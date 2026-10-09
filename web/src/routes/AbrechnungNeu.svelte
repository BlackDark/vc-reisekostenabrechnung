<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	type Arbeitgeber = { id: string; name: string };

	let employers = $state<Arbeitgeber[]>([]);
	let arbeitgeber = $state("");
	let art = $state("monat");
	let von = $state("");
	let bis = $state("");
	let titel = $state("");
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const ag = await api.GET("/api/v1/arbeitgeber");
		employers = (ag.data?.items ?? []).map((row) => ({ id: row.id, name: row.name }));
		if (!arbeitgeber && employers[0]) arbeitgeber = employers[0].id;
	}

	function pad(n: number): string {
		return String(n).padStart(2, "0");
	}

	function iso(date: Date): string {
		return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;
	}

	function bounds(kind: string, start: string): { von: string; bis: string } {
		const [y, month, day] = start.split("-").map(Number);
		if (!y || !month || !day) return { von: start, bis: start };
		if (kind === "tag") return { von: start, bis: start };
		if (kind === "monat") {
			const last = new Date(Date.UTC(y, month, 0)).getUTCDate();
			const vonDate = `${y}-${pad(month)}-01`;
			return { von: vonDate, bis: `${y}-${pad(month)}-${pad(last)}` };
		}
		if (kind === "quartal") {
			const startMonth = Math.floor((month - 1) / 3) * 3 + 1;
			const endMonth = startMonth + 2;
			const last = new Date(Date.UTC(y, endMonth, 0)).getUTCDate();
			return { von: `${y}-${pad(startMonth)}-01`, bis: `${y}-${pad(endMonth)}-${pad(last)}` };
		}
		if (kind === "woche") {
			const date = new Date(Date.UTC(y, month - 1, day));
			const wd = date.getUTCDay();
			const monday = new Date(date);
			monday.setUTCDate(day - (wd === 0 ? 6 : wd - 1));
			const sunday = new Date(monday);
			sunday.setUTCDate(monday.getUTCDate() + 6);
			return { von: iso(monday), bis: iso(sunday) };
		}
		return { von: start, bis: start };
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const period = art === "frei" ? { von, bis: bis || von } : bounds(art, von);
		const res = await api.POST("/api/v1/abrechnungen", {
			body: {
				arbeitgeber_id: arbeitgeber,
				zeitraum_art: art,
				von: period.von,
				bis: period.bis,
				titel,
				sprache: session.locale === "en" ? "en" : "de",
			},
		});
		if (!res.data) {
			error = m.save_failed();
			return;
		}
		await navigate("/abrechnungen/:id", { params: { id: res.data.id } });
	}
</script>

<h1 class="text-2xl font-semibold">{m.abrechnung_new()}</h1>
<form class="mt-4 grid gap-3" onsubmit={save}>
	<label class="grid gap-1 text-sm">
		{m.reise_employer()}
		<select id="abrechnung-ag" class="rounded border px-3 py-3" bind:value={arbeitgeber}>
			{#each employers as row (row.id)}
				<option value={row.id}>{row.name}</option>
			{/each}
		</select>
	</label>
	<label class="grid gap-1 text-sm">
		{m.abrechnung_period()}
		<select id="abrechnung-art" class="rounded border px-3 py-3" bind:value={art}>
			<option value="tag">{m.abrechnung_art_tag()}</option>
			<option value="woche">{m.abrechnung_art_woche()}</option>
			<option value="monat">{m.abrechnung_art_monat()}</option>
			<option value="quartal">{m.abrechnung_art_quartal()}</option>
			<option value="frei">{m.abrechnung_art_frei()}</option>
		</select>
	</label>
	<label class="grid gap-1 text-sm">
		{m.abrechnung_von()}
		<input id="abrechnung-von" class="rounded border px-3 py-3" type="date" bind:value={von} required />
	</label>
	{#if art === "frei"}
		<label class="grid gap-1 text-sm">
			{m.abrechnung_bis()}
			<input id="abrechnung-bis" class="rounded border px-3 py-3" type="date" bind:value={bis} required />
		</label>
	{/if}
	<label class="grid gap-1 text-sm">
		{m.abrechnung_titel()}
		<input id="abrechnung-titel" class="rounded border px-3 py-3" bind:value={titel} />
	</label>
	{#if error}
		<p class="text-sm" role="alert">{error}</p>
	{/if}
	<button class="rounded bg-blue-800 px-3 py-3 text-white" type="submit" data-testid="abrechnung-create">
		{m.create()}
	</button>
</form>
