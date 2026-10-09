<script lang="ts">
	let { label, value, stat }: { label: string; value: string; stat?: string } = $props();
</script>

<div class="bg-card min-w-0 rounded-xl border px-3 py-2" data-stat={stat}>
	<p class="text-muted-foreground truncate text-xs">{label}</p>
	<p class="mt-1 text-right text-sm font-medium tabular-nums tracking-tight">{value}</p>
</div>
