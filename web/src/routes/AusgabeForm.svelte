<script lang="ts">
	import AusgabeEditor from "$lib/components/ausgabe-editor.svelte";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p, route } from "../router";

	// The route serves two shapes: /ausgaben/:id edits, /reisen/:id/ausgaben/neu creates.
	const editing = $derived(route.pathname.startsWith("/ausgaben/"));
	const reiseId = $derived(editing ? "" : (route.params.id ?? ""));
	const ausgabeId = $derived(editing ? (route.params.id ?? "") : "");
	const belegId = $derived(String(route.search.beleg ?? ""));
	const kiVorschlag = $derived(String(route.search.vorschlag ?? "") === "1");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});
</script>

<p><a class="text-sm underline" href={p("/reisen")}>{m.back()}</a></p>
<h1 class="mt-2 text-2xl font-semibold tracking-tight">{m.ausgabe_title()}</h1>
<AusgabeEditor
	{reiseId}
	{ausgabeId}
	{belegId}
	{kiVorschlag}
	onsaved={(row) => void navigate("/ausgaben/:id", { params: { id: row.id } })}
/>