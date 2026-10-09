<script lang="ts">
	import { api } from "$lib/api";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Todo = { code: string; reise_id: string; anlass: string; datum?: string | null };

	let todos = $state<Todo[]>([]);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void api.GET("/api/v1/warnungen").then((res) => {
			todos = res.data?.items ?? [];
		});
	});

	function label(code: string): string {
		if (code === "W03") return m.warn_W03();
		if (code === "W12") return m.warn_W12();
		if (code === "W14") return m.warn_W14();
		return code;
	}
</script>

{#if session.ready && session.nutzer}
	<h1 class="text-2xl font-semibold">{m.home_title()}</h1>
	<p class="mt-3">
		<a class="underline" href={p("/reisen/neu")}>{m.reise_new()}</a>
	</p>
	<section class="mt-6" aria-label={m.home_todo()}>
		<h2 class="text-lg font-medium">{m.home_todo()}</h2>
		{#if todos.length === 0}
			<p class="mt-2 text-sm">{m.home_todo_empty()}</p>
		{:else}
			<ul class="mt-2 grid gap-2">
				{#each todos as item (item.reise_id + item.code + (item.datum ?? ""))}
					<li>
						<a class="block rounded border px-3 py-2" href={p("/reisen/:id", { params: { id: item.reise_id } })}>
							{item.anlass}: {label(item.code)}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}
