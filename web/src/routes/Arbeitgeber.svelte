<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Row = {
		id: string;
		name: string;
		anschrift: string;
		ist_standard: boolean;
		archiviert: boolean;
		version: number;
	};

	let items = $state<Row[]>([]);
	let name = $state("");
	let anschrift = $state("");
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/arbeitgeber");
		if (res.data) items = res.data.items;
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.POST("/api/v1/arbeitgeber", { body: { name, anschrift } });
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		name = "";
		anschrift = "";
		await load();
	}

	async function patch(row: Row, body: { ist_standard?: boolean; archiviert?: boolean }) {
		error = "";
		const res = await api.PATCH("/api/v1/arbeitgeber/{id}", {
			params: { path: { id: row.id }, header: { "If-Match": String(row.version) } },
			body,
		});
		if (!res.response.ok) error = m.save_failed();
		await load();
	}
</script>

<h1 class="text-2xl font-semibold">{m.arbeitgeber_title()}</h1>
<form class="mt-4 grid max-w-sm gap-3" onsubmit={create}>
	<fieldset class="grid gap-3" disabled={!session.online}>
		<legend class="text-sm font-medium">{m.arbeitgeber_new()}</legend>
		<label class="grid gap-1 text-sm" for="ag-name">
			{m.arbeitgeber_name()}
			<input id="ag-name" class="rounded border px-2 py-1" bind:value={name} required />
		</label>
		<label class="grid gap-1 text-sm" for="ag-address">
			{m.arbeitgeber_address()}
			<textarea id="ag-address" class="rounded border px-2 py-1" bind:value={anschrift} required></textarea>
		</label>
		<Button type="submit">{m.create()}</Button>
		{#if error}<p class="text-sm text-red-700" role="alert">{error}</p>{/if}
	</fieldset>
</form>
{#if items.length === 0}
	<p class="mt-6 text-sm">{m.arbeitgeber_empty()}</p>
{:else}
	<ul class="mt-6 grid gap-3">
		{#each items as row (row.id)}
			<li class="rounded border p-3 text-sm">
				<p class="font-medium">{row.name}</p>
				<p class="whitespace-pre-line">{row.anschrift}</p>
				<p class="mt-1">
					{#if row.ist_standard}{m.arbeitgeber_standard()}{/if}
					{#if row.archiviert} · {m.arbeitgeber_archived()}{/if}
				</p>
				<p class="mt-2 flex flex-wrap gap-2">
					<a class="underline" href={p("/arbeitgeber/:id", { params: { id: row.id } })}>{m.arbeitgeber_edit()}</a>
					{#if !row.archiviert && !row.ist_standard}
						<button type="button" class="underline" onclick={() => patch(row, { ist_standard: true })}>
							{m.arbeitgeber_make_standard()}
						</button>
					{/if}
					{#if !row.archiviert}
						<button type="button" class="underline" onclick={() => patch(row, { archiviert: true })}>
							{m.arbeitgeber_archive()}
						</button>
					{/if}
				</p>
			</li>
		{/each}
	</ul>
{/if}
