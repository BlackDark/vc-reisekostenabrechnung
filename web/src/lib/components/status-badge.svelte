<script lang="ts">
	import { Badge } from "$lib/components/ui/badge";
	import { statusLabel } from "$lib/labels";

	let { status }: { status: string } = $props();

	const variant = $derived(
		status === "bezahlt" || status === "aktiv" || status === "bestaetigt" || status === "gesperrt"
			? "default"
			: status === "eingereicht" || status === "zur_bestaetigung" || status === "in_entwurf"
				? "secondary"
				: status === "fehlgeschlagen"
					? "destructive"
					: "outline",
	);

	const label = $derived(statusLabel(status));
</script>

<Badge {variant} data-status={status}>{label}</Badge>
