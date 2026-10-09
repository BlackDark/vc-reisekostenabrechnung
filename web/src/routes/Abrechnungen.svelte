<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import StatusBadge from "$lib/components/status-badge.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Empty from "$lib/components/ui/empty";
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
</script>

<div class="flex flex-wrap items-end justify-between gap-3">
	<h1 class="text-2xl font-semibold tracking-tight">{m.abrechnung_title()}</h1>
	<Button href={p("/abrechnungen/neu")}>{m.abrechnung_new()}</Button>
</div>
{#if rows.length === 0}
	<Empty.Root class="mt-4 border">
		<Empty.Header>
			<Empty.Title>{m.abrechnung_empty()}</Empty.Title>
		</Empty.Header>
	</Empty.Root>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each rows as row (row.id)}
			<li>
				<a
					class="bg-card hover:bg-muted block rounded-xl border px-4 py-3 text-sm"
					href={p("/abrechnungen/:id", { params: { id: row.id } })}
				>
					<span class="flex items-center justify-between gap-3">
						<span class="font-medium">{row.titel}</span>
						<StatusBadge status={row.status} />
					</span>
					<span class="text-muted-foreground mt-1 block">
						{row.von} – {row.bis}
						{#if row.abrechnungsnummer} · {row.abrechnungsnummer}{/if}
					</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
