<script lang="ts">
	import { ModeWatcher } from "mode-watcher";
	import { Router } from "sv-router";
	import { onMount } from "svelte";
	import AppSidebar from "$lib/components/app-sidebar.svelte";
	import ThemeToggle from "$lib/components/theme-toggle.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Sidebar from "$lib/components/ui/sidebar";
	import { Toaster } from "$lib/components/ui/sonner";
	import { m } from "$lib/paraglide/messages.js";
	import ReloadPrompt from "$lib/ReloadPrompt.svelte";
	import { session } from "$lib/session.svelte";
	import { p } from "./router";

	onMount(() => {
		void session.init();
	});
</script>

<ModeWatcher
	defaultMode="dark"
	disableHeadScriptInjection
	disableTransitions={false}
	themeColors={{ dark: "#09090b", light: "#ffffff" }}
/>
<Toaster />

{#key session.locale}
	{#if !session.online}
		<p class="bg-amber-500/15 text-foreground px-4 py-2 text-sm" role="status">{m.offline()}</p>
	{/if}
	{#if session.nutzer}
		<Sidebar.Provider>
			<AppSidebar />
			<Sidebar.Inset>
				<header class="bg-background/90 sticky top-0 z-30 flex flex-wrap items-center justify-end gap-2 border-b px-4 py-3 backdrop-blur">
					<Sidebar.Trigger class="md:hidden" />
					<div class="ml-auto flex flex-wrap items-center gap-2">
						<ThemeToggle />
						<Button variant="outline" size="sm" type="button" onclick={() => session.applyLocale("de")}>DE</Button>
						<Button variant="outline" size="sm" type="button" onclick={() => session.applyLocale("en")}>EN</Button>
					</div>
				</header>
				<main class="mx-auto w-full max-w-5xl flex-1 px-4 py-6 pb-24 md:pb-10">
					<Router />
				</main>
			</Sidebar.Inset>
		</Sidebar.Provider>
	{:else}
		<div class="flex min-h-svh flex-col">
			<header class="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
				<a class="font-semibold tracking-tight" href={p("/")}>{m.app_title()}</a>
				<div class="flex flex-wrap items-center gap-2">
					<ThemeToggle />
					<Button variant="outline" size="sm" type="button" onclick={() => session.applyLocale("de")}>DE</Button>
					<Button variant="outline" size="sm" type="button" onclick={() => session.applyLocale("en")}>EN</Button>
					<a class="text-sm underline-offset-4 hover:underline" href={p("/login")}>{m.login_title()}</a>
				</div>
			</header>
			<main class="mx-auto flex w-full max-w-md flex-1 items-center px-4 py-10">
				<div class="w-full">
					<Router />
				</div>
			</main>
		</div>
	{/if}
	<ReloadPrompt />
{/key}
