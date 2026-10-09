<script lang="ts">
	import { Router } from "sv-router";
	import { onMount } from "svelte";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import ReloadPrompt from "$lib/ReloadPrompt.svelte";
	import { session } from "$lib/session.svelte";
	import { p } from "./router";

	onMount(() => {
		void session.init();
	});
</script>

{#key session.locale}
	<div class="min-h-screen overflow-x-hidden">
		{#if !session.online}
			<p class="bg-amber-100 px-4 py-2 text-sm" role="status">{m.offline()}</p>
		{/if}
		<header class="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
			<a class="font-semibold" href={p("/")}>{m.app_title()}</a>
			<nav class="flex flex-wrap items-center gap-2 text-sm">
				<Button variant="outline" size="sm" type="button" onclick={() => session.applyLocale("de")}>DE</Button>
				<Button variant="outline" size="sm" type="button" onclick={() => session.applyLocale("en")}>EN</Button>
				{#if session.nutzer}
					<a href={p("/reisen")}>{m.nav_reisen()}</a>
					<a href={p("/arbeitgeber")}>{m.nav_arbeitgeber()}</a>
					<a href={p("/taetigkeitsstaetten")}>{m.nav_staetten()}</a>
					<a href={p("/satztabellen")}>{m.nav_saetze()}</a>
					<a href={p("/profil")}>{m.profile_title()}</a>
					{#if session.nutzer.ist_admin}
						<a href={p("/admin")}>{m.admin_title()}</a>
					{/if}
				{:else}
					<a href={p("/login")}>{m.login_title()}</a>
				{/if}
			</nav>
		</header>
		<main class="mx-auto w-full max-w-3xl px-4 py-6">
			<Router />
		</main>
		<ReloadPrompt />
	</div>
{/key}
