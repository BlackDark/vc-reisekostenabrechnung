<script lang="ts">
	import { m } from "$lib/paraglide/messages.js";

	let pipeline = $state("");

	$effect(() => {
		void fetch("/version")
			.then((res) => res.json())
			.then((body: { pipeline_version?: string }) => {
				pipeline = body.pipeline_version ?? "";
			});
	});
</script>

<h1 class="text-2xl font-semibold">{m.app_title()}</h1>
<p class="mt-3 text-sm">{m.about_pipeline()}: {pipeline}</p>
<p class="mt-2 text-sm">{m.about_sources()}</p>
