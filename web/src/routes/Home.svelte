<script lang="ts">
	import { api } from "$lib/api";
	import type { components } from "$lib/api/schema";
	import StatCard from "$lib/components/stat-card.svelte";
	import StatusBadge from "$lib/components/status-badge.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Empty from "$lib/components/ui/empty";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { euroAmount } from "$lib/money";
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
	type Trip = components["schemas"]["Reise"];
	type Receipt = components["schemas"]["Beleg"];

	let todos = $state<Todo[] | null>(null);
	let openTrips = $state<Trip[] | null>(null);
	let receipts = $state<Receipt[] | null>(null);
	let periodCents = $state<number | null>(null);

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const today = new Date().toLocaleDateString("en-CA", { timeZone: "Europe/Berlin" });
		const [warn, trips, files, claims] = await Promise.all([
			api.GET("/api/v1/warnungen"),
			api.GET("/api/v1/reisen"),
			api.GET("/api/v1/belege", { params: { query: { limit: 20 } } }),
			api.GET("/api/v1/abrechnungen"),
		]);
		todos = warn.data?.items ?? [];
		openTrips = (trips.data?.items ?? [])
			.filter((row) => row.status === "offen" || row.status === "in_entwurf")
			.sort((a, b) => (a.beginn < b.beginn ? 1 : -1))
			.slice(0, 6);
		receipts = [...(files.data?.items ?? [])]
			.sort((a, b) => (a.erstellt_am < b.erstellt_am ? 1 : -1))
			.slice(0, 5);
		const period = (claims.data?.items ?? []).filter((row) => row.von <= today && row.bis >= today).slice(0, 8);
		if (period.length === 0) {
			periodCents = 0;
			return;
		}
		const checks = await Promise.all(
			period.map((row) => api.GET("/api/v1/abrechnungen/{id}/pruefung", { params: { path: { id: row.id } } })),
		);
		periodCents = checks.reduce((sum, res) => sum + (res.data?.erstattung_cent ?? 0), 0);
	}

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

	<div class="mt-4 grid grid-cols-2 gap-2 md:grid-cols-3" data-dashboard={openTrips === null ? "loading" : "ready"}>
		<StatCard label={m.home_period()} value={periodCents === null ? "…" : euroAmount(periodCents, session.locale)} />
		<StatCard label={m.home_open()} value={openTrips === null ? "…" : String(openTrips.length)} />
		<StatCard label={m.home_receipts()} value={receipts === null ? "…" : String(receipts.length)} />
	</div>

	<div class="mt-4 grid gap-3 md:grid-cols-2">
		<section aria-label={m.home_open()}>
			<p class="text-sm font-medium">{m.home_open()}</p>
			{#if openTrips === null}
				<Skeleton class="mt-2 h-24 w-full" />
			{:else if openTrips.length === 0}
				<Empty.Root class="mt-2 border">
					<Empty.Header>
						<Empty.Title>{m.home_no_open()}</Empty.Title>
					</Empty.Header>
				</Empty.Root>
			{:else}
				<ul class="mt-2 grid gap-2">
					{#each openTrips as trip (trip.id)}
						<li>
							<a
								class="bg-card hover:bg-muted block rounded-xl border px-3 py-2 transition-colors"
								href={p("/reisen/:id", { params: { id: trip.id } })}
							>
								<span class="flex items-center justify-between gap-2">
									<span class="min-w-0 truncate text-sm font-medium">{trip.anlass}</span>
									<StatusBadge status={trip.status} />
								</span>
								<span class="text-muted-foreground mt-1 block text-xs">{trip.beginn.slice(0, 10)} – {trip.ende.slice(0, 10)}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section aria-label={m.home_receipts()}>
			<p class="text-sm font-medium">{m.home_receipts()}</p>
			{#if receipts === null}
				<Skeleton class="mt-2 h-24 w-full" />
			{:else if receipts.length === 0}
				<Empty.Root class="mt-2 border">
					<Empty.Header>
						<Empty.Title>{m.home_no_receipts()}</Empty.Title>
					</Empty.Header>
				</Empty.Root>
			{:else}
				<ul class="mt-2 grid gap-2">
					{#each receipts as file (file.id)}
						<li>
							<a
								class="bg-card hover:bg-muted flex items-center justify-between gap-2 rounded-xl border px-3 py-2 text-sm"
								href={p("/belege/:id", { params: { id: file.id } })}
							>
								<span class="min-w-0 truncate">{file.belegnummer || file.id}</span>
								<span class="text-muted-foreground shrink-0 text-xs tabular-nums">{file.erstellt_am.slice(0, 10)}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	</div>

	<section class="mt-6" aria-label={m.home_todo()}>
		<h2 class="text-sm font-medium">{m.home_todo()}</h2>
		{#if todos === null}
			<div class="mt-2 grid gap-2">
				<Skeleton class="h-12 w-full" />
				<Skeleton class="h-12 w-full" />
			</div>
		{:else if todos.length === 0}
			<Empty.Root class="mt-2 border">
				<Empty.Header>
					<Empty.Title>{m.home_todo_empty()}</Empty.Title>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<ul class="mt-2 grid gap-2">
				{#each todos as item (`${item.beleg_id ?? ""}-${item.ausgabe_id ?? ""}-${item.reise_id ?? ""}-${item.code}-${item.anlass ?? ""}-${item.datum ?? ""}`)}
					<li>
						<a class="bg-card hover:bg-muted block rounded-xl border px-3 py-2 text-sm transition-colors" href={href(item)}>
							{label(item.code)}{#if item.code !== "W04" && item.anlass}: {item.anlass}{/if}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}
