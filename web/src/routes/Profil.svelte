<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { m } from "$lib/paraglide/messages.js";
	import { pwa } from "$lib/pwa.svelte";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	let anzeigename = $state("");
	let personalnummer = $state("");
	let kiErlaubt = $state(false);
	let letterhead = $state<{ id: string; name: string; anschrift: string; logo_datei_id?: string | null } | null>(null);
	let sessions = $state<{ id: string; aktuell: boolean; user_agent?: string | null }[]>([]);
	let tokens = $state<components["schemas"]["ApiToken"][]>([]);
	let tokenName = $state("");
	let createdToken = $state("");
	let createdForward = $state("");
	let tokenScope = $state<"belege:upload" | "mail:ingest">("belege:upload");
	let allowlist = $state<{ wert: string; created_at: string }[]>([]);
	let allowWert = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
		if (session.nutzer) {
			anzeigename = session.nutzer.anzeigename;
			personalnummer = session.nutzer.personalnummer ?? "";
			kiErlaubt = session.nutzer.ki_erlaubt;
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
		void reloadTokens();
		void reloadAllowlist();
	});

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!session.nutzer) return;
		const sprache = session.locale;
		const res = await api.PATCH("/api/v1/me", {
			params: { header: { "If-Match": String(session.nutzer.version) } },
			body: {
				anzeigename,
				sprache,
				personalnummer,
				...(session.aiAktiv ? { ki_erlaubt: kiErlaubt } : {}),
			},
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

	async function reloadTokens() {
		const res = await api.GET("/api/v1/me/tokens");
		if (res.data) tokens = res.data.items;
	}

	async function createToken(event: SubmitEvent) {
		event.preventDefault();
		const name = tokenName.trim();
		if (!name) return;
		const res = await api.POST("/api/v1/me/tokens", { body: { name, scope: tokenScope } });
		if (res.response.ok && res.data) {
			createdToken = res.data.token;
			createdForward = res.data.forwarding_address ?? "";
			tokenName = "";
			await reloadTokens();
		}
	}

	async function reloadAllowlist() {
		const res = await api.GET("/api/v1/me/mail/erlaubt");
		if (res.data) allowlist = res.data.items;
	}

	async function addAllowed(event: SubmitEvent) {
		event.preventDefault();
		const wert = allowWert.trim();
		if (!wert) return;
		const res = await api.POST("/api/v1/me/mail/erlaubt", { body: { wert } });
		if (res.response.ok) {
			allowWert = "";
			await reloadAllowlist();
		}
	}

	async function removeAllowed(wert: string) {
		await api.DELETE("/api/v1/me/mail/erlaubt/{wert}", { params: { path: { wert } } });
		await reloadAllowlist();
	}

	async function revokeToken(id: string) {
		await api.DELETE("/api/v1/me/tokens/{id}", { params: { path: { id } } });
		await reloadTokens();
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">{m.profile_title()}</h1>
{#if session.nutzer}
	<form class="mt-4 grid max-w-sm gap-3" onsubmit={save}>
		<fieldset class="grid gap-3" disabled={!session.online}>
			<label class="grid gap-1 text-sm" for="anzeigename">
				{m.setup_name()}
				<Input id="anzeigename" bind:value={anzeigename}  />
			</label>
			<label class="grid gap-1 text-sm" for="personalnummer">
				{m.personalnummer()}
				<Input id="personalnummer" bind:value={personalnummer}  />
			</label>
			<p class="text-sm">{m.language()}: {session.locale}</p>
			{#if session.aiAktiv}
				<label class="flex items-center gap-2 text-sm" for="ki-erlaubt">
					<input id="ki-erlaubt" type="checkbox" bind:checked={kiErlaubt} />
					{m.ki_opt_in()}
				</label>
				<p class="text-sm" data-testid="ki-privacy">{m.ki_privacy({ url: session.aiBasisURL })}</p>
			{/if}
			<Button type="submit">{m.save()}</Button>
		</fieldset>
	</form>
	{#if letterhead}
		<section class="mt-8 max-w-sm bg-card rounded-xl border p-4" aria-label={m.letterhead()}>
			<h2 class="text-lg font-medium">{m.letterhead()}</h2>
			{#if letterhead.logo_datei_id}
				<img class="mt-2 h-12 w-auto" alt={letterhead.name} src={`/api/v1/arbeitgeber/${letterhead.id}/logo`} />
			{/if}
			<p class="mt-2 font-medium">{letterhead.name}</p>
			<p class="whitespace-pre-line text-sm">{letterhead.anschrift}</p>
		</section>
	{/if}
	{#if pwa.canInstall || (pwa.ios && !pwa.installed)}
		<section class="mt-8 max-w-sm" aria-label={m.install_title()}>
			<h2 class="text-lg font-medium">{m.install_title()}</h2>
			<div class="mt-2 grid gap-2">
				{#if pwa.canInstall}
					<Button
						variant="outline"
						type="button"
						class="w-fit"
						data-testid="install-app"
						onclick={() => void pwa.promptInstall()}
					>
						{m.install_button()}
					</Button>
				{/if}
				{#if pwa.ios && !pwa.installed}
					<p class="text-sm" data-testid="install-ios-hint">{m.install_ios_hint()}</p>
				{/if}
			</div>
		</section>
	{/if}
	<h2 class="mt-8 text-lg font-medium">{m.sessions()}</h2>
	<ul class="mt-2 grid gap-2 text-sm">
		{#each sessions as row (row.id)}
			<li class="flex items-center justify-between gap-2">
				<span>{row.user_agent || row.id}{row.aktuell ? ` (${m.current_session()})` : ""}</span>
				<Button variant="link" size="sm" type="button" onclick={() => endSession(row.id)}>{m.end_session()}</Button>
			</li>
		{/each}
	</ul>
	<h2 class="mt-8 text-lg font-medium">{m.tokens_title()}</h2>
	<form class="mt-2 flex max-w-sm items-end gap-2" onsubmit={createToken}>
		<fieldset class="flex flex-1 items-end gap-2" disabled={!session.online}>
			<label class="grid flex-1 gap-1 text-sm" for="token-name">
				{m.token_name()}
				<Input id="token-name" bind:value={tokenName} />
			</label>
			<label class="grid gap-1 text-sm" for="token-scope">
				{m.token_scope()}
				<NativeSelect id="token-scope" bind:value={tokenScope}>
					<option value="belege:upload">{m.token_scope_upload()}</option>
					<option value="mail:ingest">{m.token_scope_mail()}</option>
				</NativeSelect>
			</label>
			<Button type="submit">{m.token_create()}</Button>
		</fieldset>
	</form>
	{#if createdToken}
		<p class="mt-2 text-sm" role="status">
			{m.token_created()}
			<code class="break-all" data-testid="token-value">{createdToken}</code>
		</p>
	{/if}
	{#if createdForward}
		<p class="mt-2 text-sm" role="status">
			{m.token_forward()}
			<code class="break-all" data-testid="token-forward">{createdForward}</code>
		</p>
	{/if}
	<ul class="mt-2 grid gap-2 text-sm">
		{#each tokens as row (row.id)}
			<li class="flex items-center justify-between gap-2">
				<span>{row.name} · {row.scope}{row.revoked_at ? ` (${m.token_revoked()})` : ""}</span>
				{#if !row.revoked_at}
					<Button variant="link" size="sm" type="button" onclick={() => revokeToken(row.id)}>{m.token_revoke()}</Button>
				{/if}
			</li>
		{/each}
	</ul>
	<section class="mt-8 max-w-sm" aria-label={m.allow_title()}>
		<h2 class="text-lg font-medium">{m.allow_title()}</h2>
		<form class="mt-2 flex items-end gap-2" onsubmit={addAllowed}>
			<fieldset class="flex flex-1 items-end gap-2" disabled={!session.online}>
				<label class="grid flex-1 gap-1 text-sm" for="allow-wert">
					{m.allow_add()}
					<Input id="allow-wert" bind:value={allowWert} placeholder={m.allow_placeholder()} />
				</label>
				<Button type="submit">{m.allow_add()}</Button>
			</fieldset>
		</form>
		{#if allowlist.length === 0}
			<p class="text-muted-foreground mt-2 text-sm">{m.allow_none()}</p>
		{:else}
			<ul class="mt-2 grid gap-2 text-sm">
				{#each allowlist as row (row.wert)}
					<li class="flex items-center justify-between gap-2">
						<span class="break-all">{row.wert}</span>
						<Button variant="link" size="sm" type="button" onclick={() => removeAllowed(row.wert)}>{m.allow_remove()}</Button>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
	<p class="mt-6">
		<Button variant="outline" type="button" onclick={logout}>{m.logout()}</Button>
	</p>
{/if}
