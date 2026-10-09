<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Row = { id: string; status: string; typ: string; belegnummer?: string | null; seiten: number };

	let items = $state<Row[]>([]);
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

<h1 class="text-2xl font-semibold">{m.beleg_title()}</h1>
<p class="mt-3">
	<a class="underline" href={p("/belege/neu")}>{m.beleg_new()}</a>
</p>
<form
	class="mt-4"
	onsubmit={(event) => {
		event.preventDefault();
		void load();
	}}
>
	<label class="grid gap-1 text-sm">
		{m.beleg_status()}
		<select class="rounded border px-3 py-2" bind:value={status}>
			<option value=""> </option>
			<option value="in_aufbereitung">in_aufbereitung</option>
			<option value="zur_bestaetigung">zur_bestaetigung</option>
			<option value="bestaetigt">bestaetigt</option>
			<option value="fehlgeschlagen">fehlgeschlagen</option>
		</select>
	</label>
	<button class="mt-3 rounded border px-3 py-2" type="submit">{m.reise_filter()}</button>
</form>
{#if items.length === 0}
	<p class="mt-4 text-sm">{m.beleg_empty()}</p>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each items as item (item.id)}
			<li>
				<a class="block rounded border px-3 py-3" href={p("/belege/:id", { params: { id: item.id } })}>
					<span class="font-medium">{item.belegnummer || item.typ}</span>
					<span class="mt-1 block text-sm">{item.status}</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
