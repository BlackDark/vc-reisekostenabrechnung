<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import * as Empty from "$lib/components/ui/empty";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Todo = {
		code: string;
		reise_id?: string;
		anlass?: string;
		beleg_id?: string | null;
		ausgabe_id?: string | null;
		datum?: string | null;
	};

	let todos = $state<Todo[] | null>(null);

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
		const labels: Record<string, () => string> = {
			B01: m.warn_B01,
			B02: m.warn_B02,
			B03: m.warn_B03,
			B04: m.warn_B04,
			B05: m.warn_B05,
			B06: m.warn_B06,
			W01: m.warn_W01,
			W02: m.warn_W02,
			W03: m.warn_W03,
			W04: m.warn_W04,
			W05: m.warn_W05,
			W06: m.warn_W06,
			W07: m.warn_W07,
			W08: m.warn_W08,
			W09: m.warn_W09,
			W10: m.warn_W10,
			W11: m.warn_W11,
			W12: m.warn_W12,
			W13: m.warn_W13,
			W14: m.warn_W14,
			"H-UST-KURS": m.warn_H_UST_KURS,
		};
		return labels[code]?.() ?? code;
	}

	function href(item: Todo): string {
		if (item.ausgabe_id) return p("/ausgaben/:id", { params: { id: item.ausgabe_id } });
		if (item.beleg_id) return p("/belege/:id", { params: { id: item.beleg_id } });
		return p("/reisen/:id", { params: { id: item.reise_id ?? "" } });
	}
</script>

{#if session.ready && session.nutzer}
	<div class="flex flex-wrap items-end justify-between gap-3">
		<h1 class="text-2xl font-semibold tracking-tight">{m.home_title()}</h1>
		<Button href={p("/reisen/neu")}>{m.reise_new()}</Button>
	</div>
	<section class="mt-6" aria-label={m.home_todo()}>
		<h2 class="text-lg font-medium">{m.home_todo()}</h2>
		{#if todos === null}
			<div class="mt-3 grid gap-2">
				<Skeleton class="h-14 w-full" />
				<Skeleton class="h-14 w-full" />
			</div>
		{:else if todos.length === 0}
			<Empty.Root class="mt-3 border">
				<Empty.Header>
					<Empty.Title>{m.home_todo_empty()}</Empty.Title>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<ul class="mt-3 grid gap-2">
				{#each todos as item (`${item.beleg_id ?? ""}-${item.ausgabe_id ?? ""}-${item.reise_id ?? ""}-${item.code}-${item.anlass ?? ""}-${item.datum ?? ""}`)}
					<li>
						<a
							class="bg-card hover:bg-muted block rounded-xl border px-4 py-3 text-sm transition-colors"
							href={href(item)}
						>
							{label(item.code)}{#if item.code !== "W04" && item.anlass}: {item.anlass}{/if}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}
