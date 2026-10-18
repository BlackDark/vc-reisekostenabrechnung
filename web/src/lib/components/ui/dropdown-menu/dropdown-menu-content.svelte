<script lang="ts">
	import { DropdownMenu as Primitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		sideOffset = 4,
		align = "end",
		...restProps
	}: Primitive.ContentProps = $props();
</script>

<Primitive.Portal>
	<Primitive.Content
		bind:ref
		{sideOffset}
		{align}
		class={cn(
			"bg-popover text-popover-foreground ring-foreground/10 z-50 min-w-48 rounded-lg p-1 text-sm shadow-md ring-1 outline-hidden",
			className,
		)}
		{...restProps}
	/>
</Primitive.Portal>
