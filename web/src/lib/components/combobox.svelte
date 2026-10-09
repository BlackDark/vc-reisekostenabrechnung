<script lang="ts">
	import ChevronsUpDownIcon from "@lucide/svelte/icons/chevrons-up-down";
	import { Button } from "$lib/components/ui/button";
	import * as Command from "$lib/components/ui/command";
	import * as Popover from "$lib/components/ui/popover";

	let {
		value = $bindable(""),
		options,
		placeholder,
		label,
	}: {
		value: string;
		options: { value: string; label: string }[];
		placeholder: string;
		label: string;
	} = $props();

	let open = $state(false);
	const current = $derived(options.find((option) => option.value === value)?.label ?? placeholder);
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} type="button" variant="outline" class="w-full justify-between" aria-label={label}>
				{current}
				<ChevronsUpDownIcon />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-(--bits-popover-anchor-width) p-0">
		<Command.Root>
			<Command.Input placeholder={label} />
			<Command.List>
				<Command.Empty>{placeholder}</Command.Empty>
				<Command.Group>
					{#each options as option (option.value)}
						<Command.Item
							value={option.label}
							onSelect={() => {
								value = option.value;
								open = false;
							}}
						>
							{option.label}
						</Command.Item>
					{/each}
				</Command.Group>
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
