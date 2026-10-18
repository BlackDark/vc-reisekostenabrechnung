<script lang="ts">
	import { DropdownMenu as Primitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		variant = "default",
		...restProps
	}: Primitive.ItemProps & { variant?: "default" | "destructive" } = $props();
</script>

<Primitive.Item
	bind:ref
	class={cn(
		"data-highlighted:bg-muted flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 outline-hidden select-none",
		variant === "destructive" && "text-destructive",
		className,
	)}
	{...restProps}
/>
