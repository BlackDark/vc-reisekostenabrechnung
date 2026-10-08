<script lang="ts">
	import { useRegisterSW } from "virtual:pwa-register/svelte";
	import { m } from "$lib/paraglide/messages.js";

	const { needRefresh, updateServiceWorker } = useRegisterSW({ immediate: true });
</script>

{#if $needRefresh}
	<div class="fixed right-4 bottom-4 flex items-center gap-3 rounded-lg border bg-white p-3 shadow">
		<p>{m.update_available()}</p>
		<button type="button" class="underline" onclick={() => updateServiceWorker(true)}>
			{m.update_reload()}
		</button>
	</div>
{/if}
