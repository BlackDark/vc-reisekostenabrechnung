<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { Textarea } from "$lib/components/ui/textarea";
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
<div class="mt-2 flex flex-wrap items-end justify-between gap-3">
	<div class="min-w-0">
		<h1 class="text-2xl font-semibold tracking-tight">{m.arbeitgeber_edit()}</h1>
		{#if loaded && name}<p class="text-muted-foreground mt-1 truncate text-sm">{name}</p>{/if}
	</div>
</div>
{#if loaded}
	<div class="mt-4 grid items-start gap-4 md:grid-cols-2">
		<form class="grid gap-3" onsubmit={save}>
			<label class="grid gap-1 text-sm" for="ag-name">
				{m.arbeitgeber_name()}
				<Input id="ag-name" bind:value={name} required />
			</label>
			<label class="grid gap-1 text-sm" for="ag-address">
				{m.arbeitgeber_address()}
				<Textarea id="ag-address" bind:value={anschrift} required></Textarea>
			</label>
			<div class="bg-background/95 sticky bottom-16 z-30 -mx-4 border-t px-4 py-3 md:static md:mx-0 md:border-0 md:bg-transparent md:p-0">
				<Button type="submit">{m.save()}</Button>
			</div>
		</form>
		<div class="grid gap-3">
			<label class="grid gap-1 text-sm" for="ag-logo">
				{m.arbeitgeber_logo()}
				<Input id="ag-logo" type="file" accept="image/png,image/jpeg" onchange={upload} />
			</label>
			<section class="bg-card rounded-xl border p-4" aria-label={m.letterhead()}>
				<h2 class="text-sm font-medium">{m.letterhead()}</h2>
				{#if logoId}
					<img class="mt-2 h-12 w-auto" alt={name} src={`/api/v1/arbeitgeber/${id}/logo`} />
				{/if}
				<p class="mt-2 font-medium">{name}</p>
				<p class="text-sm whitespace-pre-line">{anschrift}</p>
			</section>
		</div>
	</div>
	{#if error}<p class="mt-3 text-sm text-destructive" role="alert">{error}</p>{/if}
{/if}
