<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	let anzeigename = $state("");
	let personalnummer = $state("");
	let letterhead = $state<{ id: string; name: string; anschrift: string; logo_datei_id?: string | null } | null>(null);
	let sessions = $state<{ id: string; aktuell: boolean; user_agent?: string | null }[]>([]);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
		if (session.nutzer) {
			anzeigename = session.nutzer.anzeigename;
			personalnummer = session.nutzer.personalnummer ?? "";
		}
	});

	$effect(() => {
		if (!session.nutzer) return;
		void api.GET("/api/v1/me/sessions").then((res) => {
			if (res.data) sessions = res.data.items;
		});
		void api.GET("/api/v1/arbeitgeber").then((res) => {
			const standard = res.data?.items.find((row) => row.ist_standard && !row.archiviert);
			letterhead = standard ?? null;
		});
	});

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!session.nutzer) return;
		const sprache = session.locale;
		const res = await api.PATCH("/api/v1/me", {
			params: { header: { "If-Match": String(session.nutzer.version) } },
			body: { anzeigename, sprache, personalnummer },
		});
		if (res.response.ok && res.data) session.nutzer = res.data;
	}

	async function logout() {
		await api.POST("/api/v1/auth/logout");
		session.nutzer = null;
		void navigate("/login");
	}

	async function endSession(id: string) {
		await api.DELETE("/api/v1/me/sessions/{id}", { params: { path: { id } } });
		sessions = sessions.filter((row) => row.id !== id);
	}
</script>

<h1 class="text-2xl font-semibold">{m.profile_title()}</h1>
{#if session.nutzer}
	<form class="mt-4 grid max-w-sm gap-3" onsubmit={save}>
		<fieldset class="grid gap-3" disabled={!session.online}>
			<label class="grid gap-1 text-sm" for="anzeigename">
				{m.setup_name()}
				<input id="anzeigename" class="rounded border px-2 py-1" bind:value={anzeigename} />
			</label>
			<label class="grid gap-1 text-sm" for="personalnummer">
				{m.personalnummer()}
				<input id="personalnummer" class="rounded border px-2 py-1" bind:value={personalnummer} />
			</label>
			<p class="text-sm">{m.language()}: {session.locale}</p>
			<Button type="submit">{m.save()}</Button>
		</fieldset>
	</form>
	{#if letterhead}
		<section class="mt-8 max-w-sm rounded border p-4" aria-label={m.letterhead()}>
			<h2 class="text-lg font-medium">{m.letterhead()}</h2>
			{#if letterhead.logo_datei_id}
				<img class="mt-2 h-12 w-auto" alt={letterhead.name} src={`/api/v1/arbeitgeber/${letterhead.id}/logo`} />
			{/if}
			<p class="mt-2 font-medium">{letterhead.name}</p>
			<p class="whitespace-pre-line text-sm">{letterhead.anschrift}</p>
		</section>
	{/if}
	<h2 class="mt-8 text-lg font-medium">{m.sessions()}</h2>
	<ul class="mt-2 grid gap-2 text-sm">
		{#each sessions as row (row.id)}
			<li class="flex items-center justify-between gap-2">
				<span>{row.user_agent || row.id}{row.aktuell ? ` (${m.current_session()})` : ""}</span>
				<button type="button" class="underline" onclick={() => endSession(row.id)}>{m.end_session()}</button>
			</li>
		{/each}
	</ul>
	<p class="mt-6">
		<Button variant="outline" type="button" onclick={logout}>{m.logout()}</Button>
	</p>
{/if}
