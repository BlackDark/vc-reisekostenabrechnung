<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Row = components["schemas"]["Abrechnung"];

	let rows = $state<Row[]>([]);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/abrechnungen");
		rows = res.data?.items ?? [];
	}

	function statusLabel(status: string): string {
		if (status === "eingereicht") return m.abrechnung_status_eingereicht();
		if (status === "bezahlt") return m.abrechnung_status_bezahlt();
		return m.abrechnung_status_entwurf();
	}
</script>

<h1 class="text-2xl font-semibold">{m.abrechnung_title()}</h1>
<p class="mt-3">
	<a class="rounded bg-blue-800 px-3 py-3 text-white" href={p("/abrechnungen/neu")}>{m.abrechnung_new()}</a>
</p>
{#if rows.length === 0}
	<p class="mt-4 text-sm">{m.abrechnung_empty()}</p>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each rows as row (row.id)}
			<li>
				<a class="block rounded border px-3 py-3 text-sm" href={p("/abrechnungen/:id", { params: { id: row.id } })}>
					<span class="font-medium">{row.titel}</span>
					<span class="mt-1 block">
						{row.von} – {row.bis} · {statusLabel(row.status)}
						{#if row.abrechnungsnummer} · {row.abrechnungsnummer}{/if}
					</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
