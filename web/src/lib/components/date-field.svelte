<script lang="ts">
	import { untrack } from "svelte";
	import { Input } from "$lib/components/ui/input";
	import { today } from "$lib/dates";

	let {
		id,
		value = $bindable(""),
		type = "date",
		required = false,
		min,
		onchange,
	}: {
		id?: string;
		value: string;
		type?: "date" | "datetime-local";
		required?: boolean;
		min?: string;
		onchange?: (event: Event) => void;
	} = $props();

	if (!value)
		value = untrack(() =>
			type === "datetime-local" ? today("08:00") : today(),
		);

	function openPicker(event: Event) {
		try {
			(event.currentTarget as HTMLInputElement).showPicker();
		} catch {
			// showPicker is unsupported or blocked; the native field still works.
		}
	}
</script>

<Input
	{id}
	{type}
	{required}
	{min}
	bind:value
	{onchange}
	onclick={openPicker}
/>
