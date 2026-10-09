<script lang="ts">
	import { api } from "$lib/api";
	import { Input } from "$lib/components/ui/input";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	let items = $state<{ jahr: number; status: string; quelle: string; auslandssaetze?: number | null }[]>([]);
	let importYear = $state("2027");
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/satztabellen");
		if (res.data) items = res.data.items;
	}

	async function upload(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		error = "";
		const body = new FormData();
		body.set("datei", file);
		const jahr = Number(importYear);
		const res = await fetch(`/api/v1/satztabellen/${jahr}/import`, { method: "POST", body, credentials: "same-origin" });
		input.value = "";
		if (!res.ok) {
			const problem = (await res.json()) as { errors?: { pointer: string; code: string }[] };
			error = `${m.satz_import_errors()} ${(problem.errors ?? []).map((item) => item.code).join(", ")}`;
			return;
		}
		await load();
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">{m.satz_title()}</h1>
<ul class="mt-4 grid gap-2 text-sm">
	{#each items as row (row.jahr)}
		<li>
			<a class="underline" href={p("/satztabellen/:jahr", { params: { jahr: String(row.jahr) } })}>
				{row.jahr}
			</a>
			· {row.status === "aktiv" ? m.satz_active() : m.satz_draft()}
			{#if row.auslandssaetze != null}· {row.auslandssaetze}{/if}
		</li>
	{/each}
</ul>
{#if session.nutzer?.ist_admin}
	<label class="mt-6 grid max-w-sm gap-1 text-sm" for="import-year">
		{m.satz_title()}
		<Input id="import-year" bind:value={importYear}  />
	</label>
	<label class="mt-2 grid max-w-sm gap-1 text-sm" for="import-csv">
		{m.satz_csv()}
		<Input id="import-csv" type="file" accept="text/csv,.csv" onchange={upload}  />
	</label>
	{#if error}<p class="mt-2 text-sm text-destructive" role="alert">{error}</p>{/if}
{/if}
