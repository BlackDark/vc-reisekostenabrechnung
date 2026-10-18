<script lang="ts">
	import CalendarCheckIcon from "@lucide/svelte/icons/calendar-check";
	import MapPinIcon from "@lucide/svelte/icons/map-pin";
	import MoonIcon from "@lucide/svelte/icons/moon";
	import PlaneLandingIcon from "@lucide/svelte/icons/plane-landing";
	import PlaneTakeoffIcon from "@lucide/svelte/icons/plane-takeoff";

	
	let { type }: { type: string | undefined } = $props();

	const Icon = $derived(
		{
			anreisetag: PlaneTakeoffIcon,
			abreisetag: PlaneLandingIcon,
			eintaegig: CalendarCheckIcon,
			ueber_nacht: MoonIcon,
		}[type ?? ""] ?? MapPinIcon,
	);
</script>

<Icon class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
