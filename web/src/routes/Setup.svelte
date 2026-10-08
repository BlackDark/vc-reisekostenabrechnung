<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	let token = $state("");
	let benutzername = $state("");
	let anzeigename = $state("");
	let passwort = $state("");
	let failed = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		failed = false;
		const res = await api.POST("/api/v1/auth/setup", {
			body: { token, benutzername, anzeigename, passwort },
		});
		if (!res.response.ok || !res.data) {
			failed = true;
			return;
		}
		session.nutzer = res.data;
		void navigate("/");
	}
</script>

<h1 class="text-2xl font-semibold">{m.setup_title()}</h1>
<form class="mt-4 grid max-w-sm gap-3" onsubmit={submit}>
	<fieldset class="grid gap-3" disabled={!session.online}>
		<label class="grid gap-1 text-sm" for="token">
			{m.setup_token()}
			<input id="token" class="rounded border px-2 py-1" bind:value={token} />
		</label>
		<label class="grid gap-1 text-sm" for="setup-user">
			{m.login_username()}
			<input id="setup-user" class="rounded border px-2 py-1" bind:value={benutzername} />
		</label>
		<label class="grid gap-1 text-sm" for="setup-name">
			{m.setup_name()}
			<input id="setup-name" class="rounded border px-2 py-1" bind:value={anzeigename} />
		</label>
		<label class="grid gap-1 text-sm" for="setup-pass">
			{m.login_password()}
			<input id="setup-pass" class="rounded border px-2 py-1" type="password" bind:value={passwort} />
		</label>
		{#if failed}
			<p class="text-sm text-red-700" role="alert">{m.login_failed()}</p>
		{/if}
		<Button type="submit">{m.setup_submit()}</Button>
	</fieldset>
</form>
