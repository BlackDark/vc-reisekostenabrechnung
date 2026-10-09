<script lang="ts">
	import { useRegisterSW } from "virtual:pwa-register/svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import { m } from "$lib/paraglide/messages.js";

	const { needRefresh, updateServiceWorker } = useRegisterSW({ immediate: true });
</script>

{#if $needRefresh}
	<div class="fixed right-4 bottom-20 z-50 md:bottom-4">
		<Card.Root class="w-72 shadow-lg">
			<Card.Content class="flex items-center justify-between gap-3 pt-4">
				<p class="text-sm">{m.update_available()}</p>
				<Button type="button" size="sm" onclick={() => updateServiceWorker(true)}>
					{m.update_reload()}
				</Button>
			</Card.Content>
		</Card.Root>
	</div>
{/if}
