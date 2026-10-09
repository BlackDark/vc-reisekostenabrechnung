<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	type Posten = {
		art: "beleg" | "export";
		id: string;
		bezeichnung: string;
		aufbewahren_bis: string;
		abgelaufen: boolean;
		inhalt_geloescht: boolean;
		sha256: string;
	};

	let items = $state<Posten[]>([]);
	let selected = $state<string[]>([]);
	let grund = $state("");
	let ack = $state(false);
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer?.ist_admin) void navigate("/");
	});

	$effect(() => {
		if (!session.nutzer?.ist_admin) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/admin/aufbewahrung");
		if (res.data) items = res.data.items;
	}

	function toggle(id: string, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((item) => item !== id);
	}

	async function purge(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const beleg_ids = items.filter((item) => item.art === "beleg" && selected.includes(item.id)).map((item) => item.id);
		const export_ids = items.filter((item) => item.art === "export" && selected.includes(item.id)).map((item) => item.id);
		const res = await api.POST("/api/v1/admin/aufbewahrung/loeschen", {
			body: { beleg_ids, export_ids, grund, ablaufhemmung_bestaetigt: ack },
		});
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		if (res.data) items = res.data.items;
		selected = [];
		grund = "";
		ack = false;
	}
</script>

<h1 class="text-2xl font-semibold">{m.aufbewahrung_title()}</h1>
<p class="mt-3 text-sm" data-testid="aufbewahrung-hinweis">{m.aufbewahrung_hint()}</p>
{#if items.length === 0}
	<p class="mt-6 text-sm">{m.aufbewahrung_empty()}</p>
{:else}
	<ul class="mt-6 grid gap-3 text-sm">
		{#each items as item (item.art + item.id)}
			<li class="rounded border p-3">
				<p>
					{item.art === "beleg" ? m.aufbewahrung_art_beleg() : m.aufbewahrung_art_export()}
					· {item.bezeichnung} · {item.aufbewahren_bis}
					{#if item.abgelaufen}
						· {m.aufbewahrung_expired()}
					{/if}
					{#if item.inhalt_geloescht}
						· {m.aufbewahrung_deleted()}
					{/if}
				</p>
				<p class="mt-1 break-all text-xs text-neutral-600">{item.sha256}</p>
				{#if item.abgelaufen && !item.inhalt_geloescht}
					<label class="mt-2 flex items-center gap-2">
						<input
							type="checkbox"
							checked={selected.includes(item.id)}
							onchange={(event) => toggle(item.id, event.currentTarget.checked)}
						/>
						{m.aufbewahrung_delete()}
					</label>
				{/if}
			</li>
		{/each}
	</ul>
	<form class="mt-4 grid max-w-lg gap-3" onsubmit={purge}>
		<label class="flex items-start gap-2 text-sm">
			<input type="checkbox" bind:checked={ack} data-testid="aufbewahrung-ack" />
			<span>{m.aufbewahrung_ack()}</span>
		</label>
		<label class="grid gap-1 text-sm" for="aufbewahrung-grund">
			{m.abrechnung_reason()}
			<textarea id="aufbewahrung-grund" class="rounded border px-2 py-1" bind:value={grund} required></textarea>
		</label>
		<Button type="submit" data-testid="aufbewahrung-purge">{m.aufbewahrung_delete()}</Button>
	</form>
{/if}
{#if error}<p class="mt-3 text-sm text-red-700" role="alert">{error}</p>{/if}
