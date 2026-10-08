<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate } from "../router";

	let items = $state<{ id: string; anzeigename: string; benutzername: string; ist_admin: boolean }[]>([]);

	$effect(() => {
		if (session.ready && !session.nutzer?.ist_admin) void navigate("/");
	});

	$effect(() => {
		if (!session.nutzer?.ist_admin) return;
		void api.GET("/api/v1/admin/nutzer").then((res) => {
			if (res.data) items = res.data.items;
		});
	});
</script>

<h1 class="text-2xl font-semibold">{m.admin_title()}</h1>
<ul class="mt-4 grid gap-2 text-sm">
	{#each items as row (row.id)}
		<li>{row.anzeigename} ({row.benutzername}){row.ist_admin ? " · Admin" : ""}</li>
	{/each}
</ul>
