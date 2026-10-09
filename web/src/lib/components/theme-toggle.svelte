<script lang="ts">
	import MonitorIcon from "@lucide/svelte/icons/monitor";
	import MoonIcon from "@lucide/svelte/icons/moon";
	import SunIcon from "@lucide/svelte/icons/sun";
	import { setMode, userPrefersMode } from "mode-watcher";
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";

	const choices = [
		{ id: "dark", label: () => m.theme_dark(), icon: MoonIcon },
		{ id: "light", label: () => m.theme_light(), icon: SunIcon },
		{ id: "system", label: () => m.theme_system(), icon: MonitorIcon },
	] as const;
</script>

<fieldset class="bg-muted m-0 inline-flex min-w-0 rounded-lg border-0 p-0.5" aria-label={m.theme_label()}>
	{#each choices as choice (choice.id)}
		<Button
			type="button"
			size="sm"
			variant={userPrefersMode.current === choice.id ? "default" : "ghost"}
			aria-pressed={userPrefersMode.current === choice.id}
			onclick={() => setMode(choice.id)}
		>
			<choice.icon />
			<span class="max-sm:sr-only">{choice.label()}</span>
		</Button>
	{/each}
</fieldset>
