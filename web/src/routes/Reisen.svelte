<script lang="ts">
	import { api } from "$lib/api";
	import Combobox from "$lib/components/combobox.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Empty from "$lib/components/ui/empty";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
	import { formatWhen } from "$lib/dates";
	import { statusLabel } from "$lib/labels";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Row = { id: string; anlass: string; beginn: string; ende: string; status: string; projekt?: string | null };

	let items = $state<Row[]>([]);
	let q = $state("");
	let status = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer) void navigate("/login");
	});

	$effect(() => {
		if (!session.nutzer) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/reisen", { params: { query: { q, status } } });
		items = res.data?.items ?? [];
	}
</script>

<div class="flex flex-wrap items-end justify-between gap-3">
	<h1 class="text-2xl font-semibold tracking-tight">{m.nav_reisen()}</h1>
	<Button href={p("/reisen/neu")}>{m.reise_new()}</Button>
</div>
<Card.Root class="mt-4">
	<Card.Content class="pt-6">
		<form
			class="grid gap-3 sm:grid-cols-2"
			onsubmit={(event) => {
				event.preventDefault();
				void load();
			}}
		>
			<Field.Field>
				<Field.Label for="reise-q">{m.reise_search()}</Field.Label>
				<Input id="reise-q" bind:value={q} />
			</Field.Field>
			<Field.Field>
				<Field.Label>{m.reise_filter()}</Field.Label>
				<Combobox
					bind:value={status}
					label={m.reise_filter()}
					placeholder={m.reise_filter()}
					options={[
						{ value: "", label: m.reise_filter() },
						{ value: "offen", label: statusLabel("offen") },
						{ value: "in_entwurf", label: statusLabel("in_entwurf") },
						{ value: "gesperrt", label: statusLabel("gesperrt") },
					]}
				/>
			</Field.Field>
			<Button class="sm:col-span-2" type="submit">{m.reise_search()}</Button>
		</form>
	</Card.Content>
</Card.Root>
{#if items.length === 0}
	<Empty.Root class="mt-4 border">
		<Empty.Header>
			<Empty.Title>{m.reise_empty()}</Empty.Title>
		</Empty.Header>
	</Empty.Root>
{:else}
	<ul class="mt-4 grid gap-2">
		{#each items as item (item.id)}
			<li>
				<a
					class="bg-card hover:bg-muted block rounded-xl border px-4 py-3 transition-colors"
					href={p("/reisen/:id", { params: { id: item.id } })}
				>
					<span class="font-medium">{item.anlass}</span>
					<span class="text-muted-foreground mt-1 block text-sm">{formatWhen(item.beginn, session.locale, "date")} – {formatWhen(item.ende, session.locale, "date")}</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
