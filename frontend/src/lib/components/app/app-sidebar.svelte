<script lang="ts">
	import { page } from '$app/state';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import LayoutGridIcon from '@lucide/svelte/icons/layout-grid';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import CardAvatar from './card-avatar.svelte';
	import Logo from './logo.svelte';
	import ThemeToggle from './theme-toggle.svelte';
	import { initials, normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { session } from '$lib/session.svelte';
	import { cn } from '$lib/utils';

	/** Called after a navigation so the mobile drawer can close. */
	let { onnavigate }: { onnavigate?: () => void } = $props();

	const activeId = $derived(Number(page.params.id) || null);
	const onOverview = $derived(page.url.pathname === '/dashboard');
	const onLeads = $derived(page.url.pathname === '/dashboard/leads');
	const onFiles = $derived(page.url.pathname === '/dashboard/files');
	const onSettings = $derived(page.url.pathname === '/dashboard/settings');
	const totalLeads = $derived(cards.list?.reduce((sum, p) => sum + p.lead_count, 0) ?? 0);

	const navItem = (active: boolean) =>
		cn(
			'flex h-9 items-center gap-2.5 rounded-lg px-2.5 text-sm font-medium transition-colors',
			active
				? 'bg-sidebar-accent text-sidebar-accent-foreground'
				: 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-foreground'
		);
</script>

<div class="flex h-full flex-col">
	<div class="flex h-16 shrink-0 items-center px-5">
		<Logo href="/dashboard" />
	</div>

	<nav class="flex min-h-0 flex-1 flex-col gap-6 px-3 pb-3" aria-label="Dashboard">
		<div class="flex flex-col gap-0.5">
			<a href="/dashboard" onclick={onnavigate} aria-current={onOverview ? 'page' : undefined} class={navItem(onOverview)}>
				<LayoutGridIcon class="size-4" />
				Overview
			</a>
			<a href="/dashboard/leads" onclick={onnavigate} aria-current={onLeads ? 'page' : undefined} class={navItem(onLeads)}>
				<InboxIcon class="size-4" />
				Leads
				{#if totalLeads > 0}
					<span class="text-muted-foreground tabular ml-auto text-xs">{totalLeads}</span>
				{/if}
			</a>
			<a href="/dashboard/files" onclick={onnavigate} aria-current={onFiles ? 'page' : undefined} class={navItem(onFiles)}>
				<FolderIcon class="size-4" />
				Files
			</a>
		</div>

		<div class="flex min-h-0 flex-1 flex-col gap-1">
			<div class="flex items-center justify-between pr-1 pl-2.5">
				<span class="text-muted-foreground text-xs font-medium">
					Cards
					{#if cards.list}<span class="tabular ml-1 opacity-70">{cards.list.length}</span>{/if}
				</span>
				<Button
					variant="ghost"
					size="icon-sm"
					class="size-7"
					onclick={() => (cards.createOpen = true)}
					aria-label="New card"
				>
					<PlusIcon />
				</Button>
			</div>

			<ul class="-mx-1 flex min-h-0 flex-col gap-0.5 overflow-y-auto px-1">
				{#if cards.list === null}
					{#each [1, 2, 3] as i (i)}
						<li class="flex items-center gap-2.5 px-2.5 py-2">
							<Skeleton class="size-8 rounded-full" />
							<div class="flex flex-1 flex-col gap-1.5">
								<Skeleton class="h-3 w-24" />
								<Skeleton class="h-2.5 w-16" />
							</div>
						</li>
					{/each}
				{:else}
					{#each cards.list as profile (profile.id)}
						{@const card = normalizeCard(profile.data)}
						{@const active = activeId === profile.id}
						<li>
							<a
								href="/dashboard/{profile.id}"
								onclick={onnavigate}
								aria-current={active ? 'page' : undefined}
								class={cn(
									'group flex items-center gap-2.5 rounded-lg px-2.5 py-2 transition-colors',
									active ? 'bg-sidebar-accent' : 'hover:bg-sidebar-accent/60'
								)}
							>
								<CardAvatar {card} fallback={profile.slug} />
								<span class="flex min-w-0 flex-1 flex-col">
									<span class="truncate text-sm font-medium">{card.name || profile.slug}</span>
									<span class="text-muted-foreground truncate font-mono text-[11px]">/p/{profile.slug}</span>
								</span>
								{#if profile.lead_count > 0}
									<span
										class="bg-brand-soft text-brand tabular rounded-full px-1.5 py-0.5 text-[11px] leading-none font-semibold"
										title="{profile.lead_count} {profile.lead_count === 1 ? 'lead' : 'leads'}"
									>
										{profile.lead_count}
									</span>
								{/if}
							</a>
						</li>
					{:else}
						<li class="text-muted-foreground px-2.5 py-2 text-sm">
							No cards yet.
							<button class="text-foreground font-medium underline-offset-4 hover:underline" onclick={() => (cards.createOpen = true)}>
								Create one
							</button>
						</li>
					{/each}
				{/if}
			</ul>
		</div>
	</nav>

	<div class="border-sidebar-border flex shrink-0 flex-col gap-2 border-t p-3">
		<a href="/dashboard/settings" onclick={onnavigate} aria-current={onSettings ? 'page' : undefined} class={navItem(onSettings)}>
			<SettingsIcon class="size-4" />
			Settings
		</a>
		<div class="flex items-center justify-between pr-0.5 pl-2.5">
			<span class="text-muted-foreground text-xs font-medium">Theme</span>
			<ThemeToggle />
		</div>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				class="hover:bg-sidebar-accent/60 aria-expanded:bg-sidebar-accent flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left transition-colors"
			>
				<Avatar.Root class="size-8">
					<Avatar.Fallback class="bg-primary text-primary-foreground text-xs font-semibold">
						{initials(session.username ?? '', '?')}
					</Avatar.Fallback>
				</Avatar.Root>
				<span class="flex min-w-0 flex-1 flex-col">
					<span class="truncate text-sm font-medium">{session.username}</span>
					<span class="text-muted-foreground text-xs">Signed in</span>
				</span>
				<ChevronsUpDownIcon class="text-muted-foreground size-4" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content side="top" align="start" class="w-(--bits-dropdown-menu-anchor-width) min-w-52">
				<DropdownMenu.Group>
					<DropdownMenu.Item onSelect={() => session.signOut()}>
						<LogOutIcon />
						Sign out
					</DropdownMenu.Item>
				</DropdownMenu.Group>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>
</div>
