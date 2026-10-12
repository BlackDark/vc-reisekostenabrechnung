<script lang="ts">
	import { Button } from "$lib/components/ui/button";
	import { m } from "$lib/paraglide/messages.js";
	import { pwa } from "$lib/pwa.svelte";

	let showHint = $state(false);
</script>

{#if !pwa.installed && (pwa.canInstall || pwa.ios)}
	<div class="flex items-center gap-2">
		<Button
			variant="outline"
			size="sm"
			type="button"
			onclick={() => {
				if (pwa.canInstall) void pwa.promptInstall();
				else showHint = !showHint;
			}}
		>
			{m.pwa_install()}
		</Button>
		{#if showHint && pwa.ios}
			<span class="text-muted-foreground max-w-48 text-xs">{m.pwa_ios_hint()}</span>
		{/if}
	</div>
{/if}
