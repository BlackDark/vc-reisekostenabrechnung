<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import * as Empty from "$lib/components/ui/empty";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import * as Table from "$lib/components/ui/table";
	import { euro } from "$lib/money";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p, route } from "../router";

	type Rate = {
		land_iso: string;
		land_name_de: string;
		satzort: string;
		ort_name?: string | null;
		vma_24h: number;
		vma_8h: number;
		uebernachtung: number;
	};
	type Override = { id: string; feld: string; grund: string; neuer_wert: string; land_iso?: string | null; satzort?: string | null };

	let status = $state("");
	let quelle = $state("");
	let version = $state(0);
	let inland = $state(0);
	let query = $state("");
	let rows = $state<Rate[]>([]);
	let log = $state<Override[]>([]);
	let field = $state("vma_24h");
	let land = $state("");
	let satzort = $state("");
	let euros = $state("");
	let grund = $state("");
	let error = $state("");
	let importNote = $state("");

	const jahr = $derived(Number(route.params.jahr ?? "0"));

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer || !jahr) return;
		void refresh(jahr, query);
	});

	async function refresh(year: number, q: string) {
		const detail = await api.GET("/api/v1/satztabellen/{jahr}", { params: { path: { jahr: year } } });
		if (!detail.data) return;
		status = detail.data.status;
		quelle = detail.data.quelle;
		version = detail.data.version;
		inland = detail.data.vma_inland_24h;
		const rates = await api.GET("/api/v1/satztabellen/{jahr}/auslandssaetze", {
			params: { path: { jahr: year }, query: { q, limit: 50 } },
		});
		rows = rates.data?.items ?? [];
		if (session.nutzer?.ist_admin) {
			const ov = await api.GET("/api/v1/satztabellen/{jahr}/overrides", { params: { path: { jahr: year } } });
			log = ov.data?.items ?? [];
		}
	}

	async function search(event: SubmitEvent) {
		event.preventDefault();
		await refresh(jahr, query);
	}

	async function override(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const amount = Number(euros);
		if (!Number.isInteger(amount)) {
			error = m.save_failed();
			return;
		}
		const inlandField = field.startsWith("vma_inland") || field.startsWith("km_") || field.startsWith("sachbezug");
		const res = await api.PATCH("/api/v1/satztabellen/{jahr}", {
			params: { path: { jahr }, header: { "If-Match": String(version) } },
			body: inlandField
				? { feld: field, neuer_wert: amount * 100, grund }
				: { feld: field, neuer_wert: amount * 100, grund, land_iso: land, satzort },
		});
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		await refresh(jahr, query);
	}

	async function activate() {
		const res = await api.POST("/api/v1/satztabellen/{jahr}/aktivieren", { params: { path: { jahr } } });
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		await refresh(jahr, query);
	}

	async function upload(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		error = "";
		importNote = "";
		const body = new FormData();
		body.set("datei", file);
		const res = await fetch(`/api/v1/satztabellen/${jahr}/import`, { method: "POST", body, credentials: "same-origin" });
		input.value = "";
		if (!res.ok) {
			const problem = (await res.json()) as { errors?: { pointer: string; code: string }[] };
			const lines = (problem.errors ?? []).map((item) => `${item.pointer} ${item.code}`).join(", ");
			error = `${m.satz_import_errors()} ${lines}`;
			return;
		}
		importNote = m.satz_draft();
		await refresh(jahr, query);
	}
</script>

<p><a class="text-sm underline" href={p("/satztabellen")}>{m.back()}</a></p>
<h1 class="mt-2 text-2xl font-semibold tracking-tight">{m.satz_title()} {jahr}</h1>
<p class="mt-2 text-sm">{m.satz_status()}: {status === "aktiv" ? m.satz_active() : m.satz_draft()}</p>
<p class="text-sm">{m.satz_source()}: {quelle}</p>
<p class="text-sm">{m.satz_inland()}: {euro(inland, session.locale)} €</p>

<form class="mt-4 flex flex-wrap items-end gap-2" onsubmit={search}>
	<label class="grid gap-1 text-sm" for="satz-q">
		{m.satz_search()}
		<Input id="satz-q" bind:value={query}  />
	</label>
	<Button type="submit">{m.satz_search()}</Button>
</form>

{#if rows.length === 0}
	<Empty.Root class="mt-4 border">
		<Empty.Header>
			<Empty.Title>{m.satz_empty()}</Empty.Title>
		</Empty.Header>
	</Empty.Root>
{:else}
	<div class="bg-card mt-4 rounded-xl border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{m.satz_country()}</Table.Head>
					<Table.Head>{m.satz_place()}</Table.Head>
					<Table.Head>24 h</Table.Head>
					<Table.Head>8 h</Table.Head>
					<Table.Head>Ü</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each rows as row (`${row.land_iso}-${row.satzort}`)}
					<Table.Row>
						<Table.Cell>{row.land_name_de}</Table.Cell>
						<Table.Cell>{row.ort_name || row.satzort || m.staette_rest()}</Table.Cell>
						<Table.Cell>{euro(row.vma_24h, session.locale)}</Table.Cell>
						<Table.Cell>{euro(row.vma_8h, session.locale)}</Table.Cell>
						<Table.Cell>{euro(row.uebernachtung, session.locale)}</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

{#if session.nutzer?.ist_admin}
	{#if status === "entwurf"}
		<p class="mt-4"><Button type="button" onclick={activate}>{m.satz_activate()}</Button></p>
	{/if}
	<form class="mt-6 grid max-w-sm gap-3" onsubmit={override}>
		<fieldset class="grid gap-3" disabled={!session.online}>
			<legend class="text-sm font-medium">{m.satz_override()}</legend>
			<label class="grid gap-1 text-sm" for="ov-land">
				{m.staette_land()}
				<Input id="ov-land" bind:value={land}  />
			</label>
			<label class="grid gap-1 text-sm" for="ov-place">
				{m.satz_place()}
				<Input id="ov-place" bind:value={satzort}  />
			</label>
			<label class="grid gap-1 text-sm" for="ov-field">
				{m.satz_field()}
				<NativeSelect class="w-full" id="ov-field" bind:value={field}>
					<option value="vma_24h">vma_24h</option>
					<option value="vma_8h">vma_8h</option>
					<option value="uebernachtung">uebernachtung</option>
					<option value="vma_inland_24h">vma_inland_24h</option>
				</NativeSelect>
			</label>
			<label class="grid gap-1 text-sm" for="ov-value">
				{m.satz_value()}
				<Input id="ov-value" bind:value={euros} inputmode="numeric"  />
			</label>
			<label class="grid gap-1 text-sm" for="ov-reason">
				{m.satz_reason()}
				<Input id="ov-reason" bind:value={grund} required  />
			</label>
			<Button type="submit">{m.satz_override()}</Button>
		</fieldset>
	</form>
	<label class="mt-4 grid max-w-sm gap-1 text-sm" for="satz-csv">
		{m.satz_csv()}
		<Input id="satz-csv" type="file" accept="text/csv,.csv" onchange={upload}  />
	</label>
	<h2 class="mt-6 text-lg font-medium">{m.satz_log()}</h2>
	<ul class="mt-2 grid gap-1 text-sm">
		{#each log as row (row.id)}
			<li>{row.feld} {row.land_iso ?? ""} {row.satzort ?? ""} — {row.grund}</li>
		{/each}
	</ul>
{/if}
{#if importNote}<p class="mt-3 text-sm" role="status">{importNote}</p>{/if}
{#if error}<p class="mt-3 text-sm text-destructive" role="alert">{error}</p>{/if}
