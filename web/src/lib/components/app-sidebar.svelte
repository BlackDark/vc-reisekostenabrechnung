<script lang="ts">
	import ArchiveIcon from "@lucide/svelte/icons/archive";
	import Building2Icon from "@lucide/svelte/icons/building-2";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import FileTextIcon from "@lucide/svelte/icons/file-text";
	import MapPinIcon from "@lucide/svelte/icons/map-pin";
	import MenuIcon from "@lucide/svelte/icons/menu";
	import PlaneIcon from "@lucide/svelte/icons/plane";
	import ReceiptIcon from "@lucide/svelte/icons/receipt";
	import TableIcon from "@lucide/svelte/icons/table";
	import UserIcon from "@lucide/svelte/icons/user";
	import UsersIcon from "@lucide/svelte/icons/users";
	import WalletIcon from "@lucide/svelte/icons/wallet";
	import type { Component } from "svelte";
	import * as Sidebar from "$lib/components/ui/sidebar";
	import { useSidebar } from "$lib/components/ui/sidebar";
	import { m } from "$lib/paraglide/messages.js";
	import { session } from "$lib/session.svelte";
	import { p, route } from "../../router";

	type Item = {
		href: string;
		label: () => string;
		icon: Component;
		match: string;
	};
	type Group = {
		id: string;
		label: () => string;
		items: Item[];
		collapsible?: boolean;
		admin?: boolean;
	};

	const groups: Group[] = [
		{
			id: "reisekosten",
			label: m.nav_group_reisekosten,
			items: [
				{ href: p("/"), label: m.nav_reisen, icon: PlaneIcon, match: "/" },
				{ href: p("/abrechnungen"), label: m.nav_abrechnungen, icon: FileTextIcon, match: "/abrechnungen" },
				{ href: p("/vorschuesse"), label: m.nav_vorschuesse, icon: WalletIcon, match: "/vorschuesse" },
			],
		},
		{
			id: "belege",
			label: m.nav_group_belege,
			items: [{ href: p("/belege"), label: m.nav_belege, icon: ReceiptIcon, match: "/belege" }],
		},
		{
			id: "stammdaten",
			label: m.nav_group_stammdaten,
			collapsible: true,
			items: [
				{ href: p("/arbeitgeber"), label: m.nav_arbeitgeber, icon: Building2Icon, match: "/arbeitgeber" },
				{ href: p("/taetigkeitsstaetten"), label: m.nav_staetten, icon: MapPinIcon, match: "/taetigkeitsstaetten" },
				{ href: p("/satztabellen"), label: m.nav_saetze, icon: TableIcon, match: "/satztabellen" },
			],
		},
		{
			id: "verwaltung",
			label: m.nav_group_verwaltung,
			collapsible: true,
			admin: true,
			items: [
				{ href: p("/admin"), label: m.admin_title, icon: UsersIcon, match: "/admin" },
				{ href: p("/admin/aufbewahrung"), label: m.nav_aufbewahrung, icon: ArchiveIcon, match: "/admin/aufbewahrung" },
			],
		},
	];
	const profile: Item = { href: p("/profil"), label: m.profile_title, icon: UserIcon, match: "/profil" };
	const primary = ["/", "/belege", "/abrechnungen", "/vorschuesse"];

	const sidebar = useSidebar();
	const visibleGroups = $derived(groups.filter((group) => !group.admin || session.nutzer?.ist_admin));
	const mobileItems = $derived(visibleGroups.flatMap((group) => group.items).filter((item) => primary.includes(item.match)));

	const COLLAPSED_KEY = "nav.collapsed";
	function loadCollapsed(): Record<string, boolean> {
		try {
			return JSON.parse(localStorage.getItem(COLLAPSED_KEY) ?? "{}");
		} catch {
			return {};
		}
	}
	let collapsed = $state(loadCollapsed());

	function active(match: string) {
		const path = route.pathname;
		if (match === "/") return path === "/";
		if (match === "/admin") return path === "/admin";
		return path === match || path.startsWith(`${match}/`);
	}

	function isOpen(group: Group): boolean {
		if (!group.collapsible || sidebar.state === "collapsed") return true;
		if (group.items.some((item) => active(item.match))) return true;
		return !collapsed[group.id];
	}

	function toggle(group: Group) {
		collapsed[group.id] = isOpen(group);
		try {
			localStorage.setItem(COLLAPSED_KEY, JSON.stringify(collapsed));
		} catch {
			// storage unavailable; the choice just lasts for this session
		}
	}
</script>

<Sidebar.Root collapsible="icon">
	<Sidebar.Header class="px-3 py-4">
		<a class="truncate text-sm font-semibold" href={p("/")}>{m.app_title()}</a>
	</Sidebar.Header>
	<Sidebar.Content>
		{#each visibleGroups as group (group.id)}
			<Sidebar.Group>
				{#if group.collapsible}
					<Sidebar.GroupLabel>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class="{props.class} hover:text-sidebar-foreground w-full justify-between"
								aria-expanded={isOpen(group)}
								onclick={() => toggle(group)}
							>
								{group.label()}
								<ChevronDownIcon class="transition-transform {isOpen(group) ? '' : '-rotate-90'}" />
							</button>
						{/snippet}
					</Sidebar.GroupLabel>
				{:else}
					<Sidebar.GroupLabel>{group.label()}</Sidebar.GroupLabel>
				{/if}
				{#if isOpen(group)}
					<Sidebar.GroupContent>
						<Sidebar.Menu>
							{#each group.items as item (item.href)}
								<Sidebar.MenuItem>
									<Sidebar.MenuButton isActive={active(item.match)} tooltipContent={item.label()}>
										{#snippet child({ props })}
											<a href={item.href} {...props} onclick={() => sidebar.setOpenMobile(false)}>
												<item.icon />
												<span>{item.label()}</span>
											</a>
										{/snippet}
									</Sidebar.MenuButton>
								</Sidebar.MenuItem>
							{/each}
						</Sidebar.Menu>
					</Sidebar.GroupContent>
				{/if}
			</Sidebar.Group>
		{/each}
	</Sidebar.Content>
	<Sidebar.Footer>
		<Sidebar.Menu>
			<Sidebar.MenuItem>
				<Sidebar.MenuButton isActive={active(profile.match)} tooltipContent={profile.label()}>
					{#snippet child({ props })}
						<a href={profile.href} {...props} onclick={() => sidebar.setOpenMobile(false)} aria-label={`${profile.label()}: ${session.nutzer?.anzeigename ?? ""}`}>
							<profile.icon />
							<span class="truncate">{session.nutzer?.anzeigename || profile.label()}</span>
						</a>
					{/snippet}
				</Sidebar.MenuButton>
			</Sidebar.MenuItem>
		</Sidebar.Menu>
	</Sidebar.Footer>
</Sidebar.Root>

<nav
	class="bg-background/95 fixed inset-x-0 bottom-0 z-40 border-t backdrop-blur md:hidden"
	aria-label={m.nav_label()}
>
	<div class="grid grid-cols-5 gap-1 px-2 py-2">
		{#each mobileItems as item (item.href)}
			<a
				href={item.href}
				class="text-muted-foreground hover:bg-muted hover:text-foreground data-[active=true]:bg-primary data-[active=true]:text-primary-foreground flex min-w-0 flex-col items-center gap-1 rounded-lg px-1 py-1.5 text-[0.7rem] font-medium"
				data-active={active(item.match)}
			>
				<item.icon class="size-5" />
				<span class="max-w-full truncate">{item.label()}</span>
			</a>
		{/each}
		<button
			type="button"
			class="text-muted-foreground hover:bg-muted hover:text-foreground flex min-w-0 flex-col items-center gap-1 rounded-lg px-1 py-1.5 text-[0.7rem] font-medium"
			onclick={() => sidebar.setOpenMobile(true)}
		>
			<MenuIcon class="size-5" />
			<span>{m.nav_more()}</span>
		</button>
	</div>
</nav>
