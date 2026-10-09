<script lang="ts">
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Dialog from "$lib/components/ui/dialog";
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

<Card.Root>
	<Card.Header>
		<h1 class="text-2xl font-semibold tracking-tight">{m.app_title()}</h1>
		<Card.Description>{m.about_pipeline()}: {pipeline}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-3">
		<p class="text-sm">{m.about_sources()}</p>
		<Dialog.Root>
			<Dialog.Trigger>
				{#snippet child({ props })}
					<Button {...props} variant="outline">{m.about_details()}</Button>
				{/snippet}
			</Dialog.Trigger>
			<Dialog.Content>
				<Dialog.Header>
					<Dialog.Title>{m.about_details()}</Dialog.Title>
					<Dialog.Description>{m.about_sources()}</Dialog.Description>
				</Dialog.Header>
				<p class="text-sm">{m.about_pipeline()}: {pipeline}</p>
			</Dialog.Content>
		</Dialog.Root>
	</Card.Content>
</Card.Root>
