<script lang="ts">
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});
</script>

{#if session.ready && session.nutzer}
	<h1 class="text-2xl font-semibold">{m.home_title()}</h1>
	<p class="mt-3 text-sm">{m.home_empty()}</p>
{/if}
