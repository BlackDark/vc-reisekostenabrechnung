<script lang="ts">
	import BedIcon from "@lucide/svelte/icons/bed";
	import CoffeeIcon from "@lucide/svelte/icons/coffee";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import SunsetIcon from "@lucide/svelte/icons/sunset";
	import UtensilsIcon from "@lucide/svelte/icons/utensils";
	import type { components } from "$lib/api/schema";
	import { m } from "$lib/paraglide/messages.js";

	let { day }: { day: components["schemas"]["Reisetag"] } = $props();

	const custom = $derived(
		Boolean(
			day.land_manuell ||
				day.verpflegung_ausgeschlossen ||
				day.zuzahlung_fruehstueck ||
				day.zuzahlung_mittag ||
				day.zuzahlung_abend,
		),
	);
	const flags = $derived([
		{ icon: CoffeeIcon, on: day.fruehstueck_gestellt, label: m.reise_breakfast() },
		{ icon: UtensilsIcon, on: day.mittag_gestellt, label: m.reise_lunch() },
		{ icon: SunsetIcon, on: day.abend_gestellt, label: m.reise_dinner() },
		{ icon: BedIcon, on: day.unterkunft !== "keine", label: m.reise_lodging() },
		{ icon: PencilIcon, on: custom, label: m.reise_flag_custom() },
	]);
</script>

<span class="flex items-center gap-1.5">
	{#each flags as flag (flag.label)}
		<flag.icon
			class="size-4 shrink-0 {flag.on ? 'text-primary' : 'text-muted-foreground/30'}"
			role="img"
			aria-label={flag.label}
			data-on={flag.on}
		/>
	{/each}
</span>
