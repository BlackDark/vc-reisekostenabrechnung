<script lang="ts">
	import { api } from "$lib/api";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { navigate, p } from "../router";

	type Nutzer = {
		id: string;
		anzeigename: string;
		benutzername: string;
		ist_admin: boolean;
		aktiv: boolean;
		version: number;
	};
	type Identitaet = { id: string; art: string; aussteller: string; subjekt: string };

	let items = $state<Nutzer[]>([]);
	let anzeigename = $state("");
	let benutzername = $state("");
	let passwort = $state("");
	let istAdmin = $state(false);
	let selected = $state("");
	let identities = $state<Identitaet[]>([]);
	let issuer = $state("");
	let subject = $state("");
	let art = $state<"oidc" | "header">("oidc");
	let resetPassword = $state("");
	let error = $state("");

	$effect(() => {
		if (session.ready && !session.nutzer?.ist_admin) void navigate("/");
	});

	$effect(() => {
		if (!session.nutzer?.ist_admin) return;
		void load();
	});

	async function load() {
		const res = await api.GET("/api/v1/admin/nutzer");
		if (res.data) items = res.data.items;
		if (!selected && items[0]) selected = items[0].id;
		if (selected) await loadIdentities();
	}

	async function loadIdentities() {
		if (!selected) return;
		const res = await api.GET("/api/v1/admin/nutzer/{id}/identitaeten", { params: { path: { id: selected } } });
		identities = res.data?.items ?? [];
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.POST("/api/v1/admin/nutzer", {
			body: { anzeigename, benutzername, passwort, ist_admin: istAdmin },
		});
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		anzeigename = "";
		benutzername = "";
		passwort = "";
		await load();
	}

	async function patch(row: Nutzer, body: { aktiv?: boolean; ist_admin_lokal?: boolean; passwort?: string }) {
		error = "";
		const res = await api.PATCH("/api/v1/admin/nutzer/{id}", {
			params: { path: { id: row.id }, header: { "If-Match": String(row.version) } },
			body,
		});
		if (res.response.status === 409) {
			error = m.last_admin();
			return;
		}
		if (!res.response.ok) error = m.save_failed();
		await load();
	}

	async function link(event: SubmitEvent) {
		event.preventDefault();
		error = "";
		const res = await api.POST("/api/v1/admin/nutzer/{id}/identitaeten", {
			params: { path: { id: selected } },
			body: { art, aussteller: issuer, subjekt: subject },
		});
		if (!res.response.ok) {
			error = m.save_failed();
			return;
		}
		issuer = "";
		subject = "";
		await loadIdentities();
	}

	async function unlink(id: string) {
		await api.DELETE("/api/v1/admin/nutzer/{id}/identitaeten", {
			params: { path: { id: selected }, query: { identitaet_id: id } },
		});
		await loadIdentities();
	}
</script>

<h1 class="text-2xl font-semibold tracking-tight">{m.admin_title()}</h1>
<p class="mt-2 text-sm"><a class="underline" href={p("/admin/aufbewahrung")}>{m.nav_aufbewahrung()}</a></p>
<form class="mt-4 grid max-w-sm gap-3" onsubmit={create}>
	<fieldset class="grid gap-3" disabled={!session.online}>
		<legend class="text-sm font-medium">{m.admin_create()}</legend>
		<label class="grid gap-1 text-sm" for="nu-name">
			{m.setup_name()}
			<Input id="nu-name" bind:value={anzeigename} required  />
		</label>
		<label class="grid gap-1 text-sm" for="nu-user">
			{m.login_username()}
			<Input id="nu-user" bind:value={benutzername} required  />
		</label>
		<label class="grid gap-1 text-sm" for="nu-pass">
			{m.admin_password()}
			<Input id="nu-pass" type="password" bind:value={passwort} required  />
		</label>
		<label class="flex items-center gap-2 text-sm" for="nu-admin">
			<input id="nu-admin" type="checkbox" bind:checked={istAdmin} />
			{m.admin_make_admin()}
		</label>
		<Button type="submit">{m.create()}</Button>
	</fieldset>
</form>
{#if error}<p class="mt-3 text-sm text-destructive" role="alert">{error}</p>{/if}
<ul class="mt-6 grid gap-3 text-sm">
	{#each items as row (row.id)}
		<li class="bg-card rounded-xl border p-3">
			<p>{row.anzeigename} ({row.benutzername}){row.ist_admin ? " · Admin" : ""}{row.aktiv ? "" : ` · ${m.admin_deactivate()}`}</p>
			<p class="mt-2 flex flex-wrap gap-2">
				<Button variant="link" size="sm" type="button" onclick={() => patch(row, { aktiv: !row.aktiv })}>
					{row.aktiv ? m.admin_deactivate() : m.admin_active()}
				</Button>
				<Button variant="link" size="sm" type="button" onclick={() => patch(row, { ist_admin_lokal: !row.ist_admin })}>
					{m.admin_make_admin()}
				</Button>
			</p>
			<form
				class="mt-2 flex flex-wrap items-end gap-2"
				onsubmit={(event) => {
					event.preventDefault();
					void patch(row, { passwort: resetPassword });
					resetPassword = "";
				}}
			>
				<label class="grid gap-1" for={`pw-${row.id}`}>
					{m.admin_reset()}
					<Input id={`pw-${row.id}`} bind:value={resetPassword}  />
				</label>
				<Button type="submit" size="sm">{m.save()}</Button>
			</form>
		</li>
	{/each}
</ul>
<h2 class="mt-8 text-lg font-medium">{m.admin_identity()}</h2>
<label class="mt-2 grid max-w-sm gap-1 text-sm" for="id-user">
	{m.admin_title()}
	<NativeSelect class="w-full" id="id-user" bind:value={selected} onchange={() => loadIdentities()}>
		{#each items as row (row.id)}
			<option value={row.id}>{row.benutzername}</option>
		{/each}
	</NativeSelect>
</label>
<form class="mt-3 grid max-w-sm gap-3" onsubmit={link}>
	<label class="grid gap-1 text-sm" for="id-art">
		{m.admin_identity()}
		<NativeSelect class="w-full" id="id-art" bind:value={art}>
			<option value="oidc">oidc</option>
			<option value="header">header</option>
		</NativeSelect>
	</label>
	<label class="grid gap-1 text-sm" for="id-issuer">
		{m.admin_issuer()}
		<Input id="id-issuer" bind:value={issuer} required  />
	</label>
	<label class="grid gap-1 text-sm" for="id-subject">
		{m.admin_subject()}
		<Input id="id-subject" bind:value={subject} required  />
	</label>
	<Button type="submit">{m.admin_link()}</Button>
</form>
<ul class="mt-3 grid gap-1 text-sm">
	{#each identities as row (row.id)}
		<li class="flex items-center justify-between gap-2">
			<span>{row.art} · {row.aussteller} · {row.subjekt}</span>
			<Button variant="link" size="sm" type="button" onclick={() => unlink(row.id)}>{m.admin_unlink()}</Button>
		</li>
	{/each}
</ul>
