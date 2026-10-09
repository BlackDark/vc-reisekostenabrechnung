<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
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

<Card.Root>
	<Card.Header>
		<h1 class="text-2xl font-semibold tracking-tight">{m.login_title()}</h1>
		<Card.Description>{m.app_title()}</Card.Description>
	</Card.Header>
	<Card.Content>
		<form class="grid gap-4" onsubmit={submit}>
			<fieldset class="grid gap-4" disabled={!session.online}>
				<Field.Field>
					<Field.Label for="benutzername">{m.login_username()}</Field.Label>
					<Input id="benutzername" autocomplete="username" bind:value={benutzername} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="passwort">{m.login_password()}</Field.Label>
					<Input id="passwort" type="password" autocomplete="current-password" bind:value={passwort} />
				</Field.Field>
				{#if failed}
					<p class="text-destructive text-sm" role="alert">{m.login_failed()}</p>
				{/if}
				{#if config?.passwort !== false}
					<Button type="submit" class="w-full">{m.login_submit()}</Button>
				{/if}
			</fieldset>
		</form>
		{#if config?.oidc}
			<p class="mt-4 text-sm">
				<a class="text-primary underline-offset-4 hover:underline" href="/api/v1/auth/oidc/start" target="_self">
					{config.oidc_button_label || m.login_sso()}
				</a>
			</p>
		{/if}
		{#if config?.setup_erforderlich}
			<p class="text-muted-foreground mt-4 text-sm">
				<a class="underline-offset-4 hover:underline" href="/setup">{m.setup_title()}</a>
			</p>
		{/if}
	</Card.Content>
</Card.Root>
