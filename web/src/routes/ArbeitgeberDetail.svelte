<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p, route } from "../router";

	let name = $state("");
	let anschrift = $state("");
	let version = $state(0);
	let logoId = $state<string | null>(null);
	let error = $state("");
	let loaded = $state(false);

	const id = $derived(route.params.id ?? "");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer || !id) return;
		void load(id);
	});

	async function load(current: string) {
		const res = await api.GET("/api/v1/arbeitgeber/{id}", { params: { path: { id: current } } });
		if (!res.data) return;
		name = res.data.name;
		anschrift = res.data.anschrift;
		version = res.data.version;
		logoId = res.data.logo_datei_id ?? null;
		loaded = true;
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.PATCH("/api/v1/arbeitgeber/{id}", {
			params: { path: { id }, header: { "If-Match": String(version) } },
			body: { name, anschrift },
		});
		if (!res.response.ok || !res.data) {
			error = m.save_failed();
			return;
		}
		version = res.data.version;
	}

	async function upload(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		error = "";
		const body = new FormData();
		body.set("datei", file);
		const res = await fetch(`/api/v1/arbeitgeber/${id}/logo`, {
			method: "POST",
			headers: { "If-Match": String(version) },
			body,
			credentials: "same-origin",
		});
		if (!res.ok) {
			error = m.save_failed();
			return;
		}
		const data = (await res.json()) as { version: number; logo_datei_id?: string };
		version = data.version;
		logoId = data.logo_datei_id ?? logoId;
		input.value = "";
	}
</script>

<p><a class="text-sm underline" href={p("/arbeitgeber")}>{m.back()}</a></p>
<h1 class="mt-2 text-2xl font-semibold">{m.arbeitgeber_edit()}</h1>
{#if loaded}
	<form class="mt-4 grid max-w-sm gap-3" onsubmit={save}>
		<label class="grid gap-1 text-sm" for="ag-name">
			{m.arbeitgeber_name()}
			<input id="ag-name" class="rounded border px-2 py-1" bind:value={name} required />
		</label>
		<label class="grid gap-1 text-sm" for="ag-address">
			{m.arbeitgeber_address()}
			<textarea id="ag-address" class="rounded border px-2 py-1" bind:value={anschrift} required></textarea>
		</label>
		<Button type="submit">{m.save()}</Button>
	</form>
	<label class="mt-4 grid max-w-sm gap-1 text-sm" for="ag-logo">
		{m.arbeitgeber_logo()}
		<input id="ag-logo" type="file" accept="image/png,image/jpeg" onchange={upload} />
	</label>
	<section class="mt-6 max-w-sm rounded border p-4" aria-label={m.letterhead()}>
		<h2 class="text-sm font-medium">{m.letterhead()}</h2>
		{#if logoId}
			<img class="mt-2 h-12 w-auto" alt={name} src={`/api/v1/arbeitgeber/${id}/logo`} />
		{/if}
		<p class="mt-2 font-medium">{name}</p>
		<p class="whitespace-pre-line text-sm">{anschrift}</p>
	</section>
	{#if error}<p class="mt-3 text-sm text-red-700" role="alert">{error}</p>{/if}
{/if}
