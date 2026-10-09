<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
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

<Card.Root>
	<Card.Header>
		<h1 class="text-2xl font-semibold tracking-tight">{m.setup_title()}</h1>
	</Card.Header>
	<Card.Content>
		<form class="grid gap-4" onsubmit={submit}>
			<fieldset class="grid gap-4" disabled={!session.online}>
				<Field.Field>
					<Field.Label for="token">{m.setup_token()}</Field.Label>
					<Input id="token" bind:value={token} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="setup-user">{m.login_username()}</Field.Label>
					<Input id="setup-user" bind:value={benutzername} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="setup-name">{m.setup_name()}</Field.Label>
					<Input id="setup-name" bind:value={anzeigename} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="setup-pass">{m.login_password()}</Field.Label>
					<Input id="setup-pass" type="password" bind:value={passwort} />
				</Field.Field>
				{#if failed}
					<p class="text-destructive text-sm" role="alert">{m.login_failed()}</p>
				{/if}
				<Button type="submit">{m.setup_submit()}</Button>
			</fieldset>
		</form>
	</Card.Content>
</Card.Root>
