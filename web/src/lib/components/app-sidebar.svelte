<script lang="ts">
	import ArchiveIcon from "@lucide/svelte/icons/archive";
	import Building2Icon from "@lucide/svelte/icons/building-2";
	import FileTextIcon from "@lucide/svelte/icons/file-text";
	import MapPinIcon from "@lucide/svelte/icons/map-pin";
	import PlaneIcon from "@lucide/svelte/icons/plane";
	import ReceiptIcon from "@lucide/svelte/icons/receipt";
	import TableIcon from "@lucide/svelte/icons/table";
	import UserIcon from "@lucide/svelte/icons/user";
	import UsersIcon from "@lucide/svelte/icons/users";
	import WalletIcon from "@lucide/svelte/icons/wallet";
	import type { Component } from "svelte";
	import * as Sidebar from "$lib/components/ui/sidebar";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { p, route } from "../../router";

	const items: {
		href: string;
		label: () => string;
		icon: Component;
		admin?: boolean;
		match: string;
	}[] = [
		{ href: p("/"), label: m.nav_reisen, icon: PlaneIcon, match: "/" },
		{ href: p("/belege"), label: m.nav_belege, icon: ReceiptIcon, match: "/belege" },
		{ href: p("/abrechnungen"), label: m.nav_abrechnungen, icon: FileTextIcon, match: "/abrechnungen" },
		{ href: p("/vorschuesse"), label: m.nav_vorschuesse, icon: WalletIcon, match: "/vorschuesse" },
		{ href: p("/arbeitgeber"), label: m.nav_arbeitgeber, icon: Building2Icon, match: "/arbeitgeber" },
		{ href: p("/taetigkeitsstaetten"), label: m.nav_staetten, icon: MapPinIcon, match: "/taetigkeitsstaetten" },
		{ href: p("/satztabellen"), label: m.nav_saetze, icon: TableIcon, match: "/satztabellen" },
		{ href: p("/profil"), label: m.profile_title, icon: UserIcon, match: "/profil" },
		{ href: p("/admin"), label: m.admin_title, icon: UsersIcon, match: "/admin", admin: true },
		{
			href: p("/admin/aufbewahrung"),
			label: m.nav_aufbewahrung,
			icon: ArchiveIcon,
			match: "/admin/aufbewahrung",
			admin: true,
		},
	];

	const visible = $derived(items.filter((item) => !item.admin || session.nutzer?.ist_admin));

	function active(match: string) {
		const path = route.pathname;
		if (match === "/") return path === "/";
		if (match === "/admin") return path === "/admin";
		return path === match || path.startsWith(`${match}/`);
	}
</script>

<Sidebar.Root collapsible="icon">
	<Sidebar.Header class="px-3 py-4">
		<a class="truncate text-sm font-semibold" href={p("/")}>{m.app_title()}</a>
	</Sidebar.Header>
	<Sidebar.Content>
		<Sidebar.Group>
			<Sidebar.GroupLabel>{m.nav_label()}</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<Sidebar.Menu>
					{#each visible as item (item.href)}
						<Sidebar.MenuItem>
							<Sidebar.MenuButton isActive={active(item.match)}>
								{#snippet child({ props })}
									<a href={item.href} {...props}>
										<item.icon />
										<span>{item.label()}</span>
									</a>
								{/snippet}
							</Sidebar.MenuButton>
						</Sidebar.MenuItem>
					{/each}
				</Sidebar.Menu>
			</Sidebar.GroupContent>
		</Sidebar.Group>
	</Sidebar.Content>
</Sidebar.Root>

<nav
	class="bg-background/95 fixed inset-x-0 bottom-0 z-40 border-t backdrop-blur md:hidden"
	aria-label={m.nav_label()}
>
	<div class="flex gap-1 overflow-x-auto px-2 py-2">
		{#each visible as item (item.href)}
			<a
				href={item.href}
				class="text-muted-foreground hover:bg-muted hover:text-foreground data-[active=true]:bg-primary data-[active=true]:text-primary-foreground flex shrink-0 items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-medium"
				data-active={active(item.match)}
			>
				<item.icon class="size-4" />
				{item.label()}
			</a>
		{/each}
	</div>
</nav>
