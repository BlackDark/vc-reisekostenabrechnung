<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Row = { id: string; anlass: string; beginn: string; ende: string; status: string; projekt?: string | null };

	let items = $state<Row[]>([]);
	let q = $state("");
	let status = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/reisen", { params: { query: { q, status } } });
		items = res.data?.items ?? [];
	}
</script>

<h1 class="text-2xl font-semibold">{m.nav_reisen()}</h1>
<p class="mt-3">
	<a class="underline" href={p("/reisen/neu")}>{m.reise_new()}</a>
</p>
<form
	class="mt-4 grid gap-3 sm:grid-cols-2"
	onsubmit={(event) => {
		event.preventDefault();
		void load();
	}}
>
	<label class="grid gap-1 text-sm">
		{m.reise_search()}
		<input class="rounded border px-3 py-2" bind:value={q} />
	</label>
	<label class="grid gap-1 text-sm">
		{m.reise_filter()}
		<select class="rounded border px-3 py-2" bind:value={status}>
			<option value=""> </option>
			<option value="offen">offen</option>
		</select>
	</label>
	<button class="rounded border px-3 py-2 sm:col-span-2" type="submit">{m.reise_search()}</button>
</form>
{#if items.length === 0}
	<p class="mt-4 text-sm">{m.reise_empty()}</p>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each items as item (item.id)}
			<li>
				<a class="block rounded border px-3 py-3" href={p("/reisen/:id", { params: { id: item.id } })}>
					<span class="font-medium">{item.anlass}</span>
					<span class="mt-1 block text-sm">{item.beginn.slice(0, 10)} – {item.ende.slice(0, 10)}</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
