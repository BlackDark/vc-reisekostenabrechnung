<script lang="ts">
	import { Badge } from "$lib/components/ui/badge";
	import { m } from "$lib/paraglide/messages.js";

	let { status }: { status: string } = $props();

	const claim: Record<string, () => string> = {
		entwurf: m.abrechnung_status_entwurf,
		eingereicht: m.abrechnung_status_eingereicht,
		bezahlt: m.abrechnung_status_bezahlt,
	};

	const variant = $derived(
		status === "bezahlt" || status === "aktiv" || status === "bestaetigt"
			? "default"
			: status === "eingereicht" || status === "zur_bestaetigung"
				? "secondary"
				: status === "fehlgeschlagen"
					? "destructive"
					: "outline",
	);

	const label = $derived(claim[status]?.() ?? status);
</script>

<Badge {variant} data-status={status}>{label}</Badge>
