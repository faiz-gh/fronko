<script lang="ts">
	import { page } from '$app/state';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import * as Avatar from '$lib/components/ui/avatar';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import CardAvatar from '$lib/features/cards/components/card-avatar.svelte';
	import FeedbackDialog from '$lib/features/feedback/components/feedback-dialog.svelte';
	import Logo from './logo.svelte';
	import ThemeToggle from './theme-toggle.svelte';
	import { initials, normalizeCard } from '$lib/features/cards/card';
	import { ROLE_LABEL } from '$lib/features/orgs/api';
	import { cards } from '$lib/features/cards/store.svelte';
	import { DASHBOARD_NAV } from '$lib/core/nav';
	import { session } from '$lib/core/session.svelte';
	import { cn } from '$lib/utils';

	/** Called after a navigation so the mobile drawer can close. */
	let { onnavigate }: { onnavigate?: () => void } = $props();

	let feedbackOpen = $state(false);

	const onCard = $derived(page.route.id === '/dashboard/[id]');
	const activeId = $derived(onCard ? Number(page.params.id) || null : null);
	const path = $derived(page.url.pathname);
	const onSettings = $derived(path === '/dashboard/settings');
	const nav = $derived(
		DASHBOARD_NAV.filter((item) => item.visible?.() ?? true).map((item) => ({
			...item,
			active: item.active ? item.active(path, page.route.id) : path === item.href,
			count: item.count?.() ?? 0
		}))
	);

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

	<div class="mx-3 mb-4 flex items-center gap-2.5 rounded-lg border px-2.5 py-2">
		<span
			class="bg-brand-soft text-brand grid size-8 shrink-0 place-items-center rounded-md text-xs font-semibold"
			aria-hidden="true"
		>
			{initials(session.orgName, '?')}
		</span>
		<span class="flex min-w-0 flex-col">
			<span class="truncate text-sm font-medium">{session.orgName}</span>
			<span class="text-muted-foreground truncate text-xs">
				{session.role ? ROLE_LABEL[session.role] : ''}{session.isLead && !session.isAdmin ? ' · Team lead' : ''}
			</span>
		</span>
	</div>

	<nav class="flex min-h-0 flex-1 flex-col gap-6 px-3 pb-3" aria-label="Dashboard">
		<div class="flex flex-col gap-0.5">
			{#each nav as item (item.href)}
				<a
					href={item.href}
					onclick={onnavigate}
					aria-current={item.active ? 'page' : undefined}
					class={navItem(item.active)}
				>
					<item.icon class="size-4" />
					{item.label}
					{#if item.count}
						<span class="text-muted-foreground tabular ml-auto text-xs">{item.count}</span>
					{/if}
				</a>
			{/each}
		</div>

		{#if !session.seesOthers}
			<div class="flex min-h-0 flex-1 flex-col gap-1">
				<span class="text-muted-foreground pl-2.5 text-xs font-medium">
					Your cards
					{#if cards.list}<span class="tabular ml-1 opacity-70">{cards.list.length}</span>{/if}
				</span>

				<ul class="-mx-1 flex min-h-0 flex-col gap-0.5 overflow-y-auto px-1">
					{#if cards.list === null}
						{#each [1, 2] as i (i)}
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
										<span class="text-muted-foreground truncate font-mono text-[11px]"
											>/p/{session.orgHandle}/{profile.slug}</span
										>
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
							<li class="text-muted-foreground px-2.5 py-2 text-sm text-pretty">
								No cards yet. {session.orgName} will assign you one.
							</li>
						{/each}
					{/if}
				</ul>
			</div>
		{/if}
	</nav>

	<div class="border-sidebar-border flex shrink-0 flex-col gap-2 border-t p-3">
		<a
			href="/dashboard/settings"
			onclick={onnavigate}
			aria-current={onSettings ? 'page' : undefined}
			class={navItem(onSettings)}
		>
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
					<span class="text-muted-foreground truncate text-xs">{session.email ?? 'Signed in'}</span>
				</span>
				<ChevronsUpDownIcon class="text-muted-foreground size-4" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content side="top" align="start" class="w-(--bits-dropdown-menu-anchor-width) min-w-52">
				<DropdownMenu.Group>
					<DropdownMenu.Item onSelect={() => (feedbackOpen = true)}>
						<MessageSquareIcon />
						Send feedback
					</DropdownMenu.Item>
					<DropdownMenu.Separator />
					<DropdownMenu.Item onSelect={() => session.signOut()}>
						<LogOutIcon />
						Sign out
					</DropdownMenu.Item>
				</DropdownMenu.Group>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>
</div>

<FeedbackDialog bind:open={feedbackOpen} />
