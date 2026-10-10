<script lang="ts">
	import { api } from "$lib/api";
	import StatusBadge from "$lib/components/status-badge.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Empty from "$lib/components/ui/empty";
	import * as Field from "$lib/components/ui/field";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { statusLabel } from "$lib/labels";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Row = { id: string; status: string; typ: string; belegnummer?: string | null; seiten: number };

	let items = $state<Row[] | null>(null);
	let status = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/belege", { params: { query: { status, eingang: false } } });
		items = res.data?.items ?? [];
	}
</script>

<div class="flex flex-wrap items-end justify-between gap-3">
	<h1 class="text-2xl font-semibold tracking-tight">{m.beleg_title()}</h1>
	<Button href={p("/belege/neu")}>{m.beleg_new()}</Button>
</div>
<form
	class="mt-4 flex flex-wrap items-end gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		void load();
	}}
>
	<fieldset class="flex flex-wrap items-end gap-3" disabled={!session.online}>
		<Field.Field class="min-w-48">
			<Field.Label for="beleg-status">{m.beleg_status()}</Field.Label>
			<NativeSelect id="beleg-status" bind:value={status} class="w-full">
				<option value=""> </option>
				<option value="in_aufbereitung">{statusLabel("in_aufbereitung")}</option>
				<option value="zur_bestaetigung">{statusLabel("zur_bestaetigung")}</option>
				<option value="bestaetigt">{statusLabel("bestaetigt")}</option>
				<option value="fehlgeschlagen">{statusLabel("fehlgeschlagen")}</option>
			</NativeSelect>
		</Field.Field>
		<Button type="submit">{m.reise_filter()}</Button>
	</fieldset>
</form>
{#if items === null}
	<div class="mt-4 grid gap-2">
		<Skeleton class="h-14 w-full" />
		<Skeleton class="h-14 w-full" />
	</div>
{:else if items.length === 0}
	<Empty.Root class="mt-4 border">
		<Empty.Header>
			<Empty.Title>{m.beleg_empty()}</Empty.Title>
		</Empty.Header>
	</Empty.Root>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each items as item (item.id)}
			<li>
				<a
					class="bg-card hover:bg-muted flex items-center justify-between gap-3 rounded-xl border px-4 py-3"
					href={p("/belege/:id", { params: { id: item.id } })}
				>
					<span class="font-medium">{item.belegnummer || item.typ}</span>
					<StatusBadge status={item.status} />
				</a>
			</li>
		{/each}
	</ul>
{/if}
