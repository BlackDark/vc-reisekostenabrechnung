<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	let benutzername = $state("");
	let passwort = $state("");
	let failed = $state(false);
	let config = $state<{
		passwort: boolean;
		oidc: boolean;
		oidc_button_label?: string;
		oidc_auto_redirect?: boolean;
		setup_erforderlich: boolean;
	} | null>(null);

	$effect(() => {
		if (session.ready && session.nutzer) void navigate("/");
	});

	$effect(() => {
		void api.GET("/api/v1/auth/config").then((res) => {
			if (!res.data) return;
			config = res.data;
			if (res.data.oidc && res.data.oidc_auto_redirect && !res.data.passwort) {
				location.assign("/api/v1/auth/oidc/start");
			}
		});
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		failed = false;
		const res = await api.POST("/api/v1/auth/login", {
			body: { benutzername, passwort },
		});
		if (!res.response.ok || !res.data) {
			failed = true;
			return;
		}
		session.nutzer = res.data;
		session.applyLocale(res.data.sprache === "en" ? "en" : "de");
		void navigate("/");
	}
</script>

<h1 class="text-2xl font-semibold">{m.login_title()}</h1>
<form class="mt-4 grid max-w-sm gap-3" onsubmit={submit}>
	<fieldset class="grid gap-3" disabled={!session.online}>
		<label class="grid gap-1 text-sm" for="benutzername">
			{m.login_username()}
			<input id="benutzername" class="rounded border px-2 py-1" autocomplete="username" bind:value={benutzername} />
		</label>
		<label class="grid gap-1 text-sm" for="passwort">
			{m.login_password()}
			<input
				id="passwort"
				class="rounded border px-2 py-1"
				type="password"
				autocomplete="current-password"
				bind:value={passwort}
			/>
		</label>
		{#if failed}
			<p class="text-sm text-red-700" role="alert">{m.login_failed()}</p>
		{/if}
		{#if config?.passwort !== false}
			<Button type="submit">{m.login_submit()}</Button>
		{/if}
	</fieldset>
</form>
{#if config?.oidc}
	<p class="mt-4">
		<a class="underline" href="/api/v1/auth/oidc/start" target="_self">{config.oidc_button_label || m.login_sso()}</a>
	</p>
{/if}
{#if config?.setup_erforderlich}
	<p class="mt-4 text-sm">
		<a class="underline" href="/setup">{m.setup_title()}</a>
	</p>
{/if}
