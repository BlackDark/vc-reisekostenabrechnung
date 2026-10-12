<script lang="ts">
	import BelegErfassen from "$lib/components/beleg-erfassen.svelte";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});
</script>

<h1 class="text-2xl font-semibold tracking-tight">{m.beleg_new()}</h1>
<p class="mt-2 text-sm">{m.beleg_drop()}</p>
<div class="mt-4">
	<BelegErfassen onsaved={(id) => void navigate("/belege/:id", { params: { id } })} />
</div>
