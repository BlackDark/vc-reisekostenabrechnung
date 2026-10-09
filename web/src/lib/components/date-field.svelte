<script lang="ts">
	import { type DateValue, parseDate } from "@internationalized/date";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import { Button } from "$lib/components/ui/button";
	import { Calendar } from "$lib/components/ui/calendar";
	import { Input } from "$lib/components/ui/input";
	import * as Popover from "$lib/components/ui/popover";
	import { m } from "$lib/paraglide/messages.js";

	let {
		id,
		value = $bindable(""),
		type = "date",
		required = false,
		onchange,
	}: {
		id?: string;
		value: string;
		type?: "date" | "datetime-local";
		required?: boolean;
		onchange?: (event: Event) => void;
	} = $props();

	let open = $state(false);
	let picked = $state<DateValue | undefined>(undefined);

	function syncFromValue() {
		const day = value.slice(0, 10);
		if (!/^\d{4}-\d{2}-\d{2}$/.test(day)) {
			picked = undefined;
			return;
		}
		try {
			picked = parseDate(day);
		} catch {
			picked = undefined;
		}
	}

	function apply(next: DateValue | undefined) {
		picked = next;
		if (!next) return;
		const day = next.toString();
		if (type === "datetime-local") {
			const time = value.length >= 16 ? value.slice(11, 16) : "00:00";
			value = `${day}T${time}`;
		} else {
			value = day;
		}
		open = false;
	}
</script>

<div class="flex items-center gap-2">
	<Input {id} class="flex-1" {type} {required} bind:value {onchange} />
	<Popover.Root bind:open>
		<Popover.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					type="button"
					variant="outline"
					size="icon"
					aria-label={m.calendar_open()}
					onclick={syncFromValue}
				>
					<CalendarIcon />
				</Button>
			{/snippet}
		</Popover.Trigger>
		<Popover.Content class="w-auto p-0" align="end">
			<Calendar
				type="single"
				bind:value={picked}
				onValueChange={(next) => apply(next)}
				locale="de-DE"
			/>
		</Popover.Content>
	</Popover.Root>
</div>
