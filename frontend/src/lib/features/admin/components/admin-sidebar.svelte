<script lang="ts">
	import { page } from '$app/state';
	import BuildingIcon from '@lucide/svelte/icons/building-2';
	import HistoryIcon from '@lucide/svelte/icons/history';
	import LayoutGridIcon from '@lucide/svelte/icons/layout-grid';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import { Button } from '$lib/components/ui/button';
	import Logo from '$lib/components/shared/logo.svelte';
	import ThemeToggle from '$lib/components/shared/theme-toggle.svelte';
	import { adminSession } from '$lib/features/admin/session.svelte';
	import { adminNav } from '$lib/features/admin/nav.svelte';
	import { cn } from '$lib/utils';

	/** Called after a navigation so the mobile drawer can close. */
	let { onnavigate }: { onnavigate?: () => void } = $props();

	const path = $derived(page.url.pathname);
	const nav = $derived([
		{ href: '/admin', label: 'Overview', icon: LayoutGridIcon, active: path === '/admin' },
		{ href: '/admin/orgs', label: 'Organisations', icon: BuildingIcon, active: path.startsWith('/admin/orgs') },
		{
			href: '/admin/feedback',
			label: 'Feedback',
			icon: MessageSquareIcon,
			active: path.startsWith('/admin/feedback'),
			count: adminNav.newFeedback
		},
		{ href: '/admin/audit', label: 'Audit log', icon: HistoryIcon, active: path === '/admin/audit' }
	]);

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
		<Logo href="/admin" />
	</div>

	<div class="mx-3 mb-4 flex items-center gap-2.5 rounded-lg border px-2.5 py-2">
		<span class="bg-brand-soft text-brand grid size-8 shrink-0 place-items-center rounded-md" aria-hidden="true">
			<ShieldIcon class="size-4" />
		</span>
		<span class="flex min-w-0 flex-col">
			<span class="truncate text-sm font-medium">Platform admin</span>
			<span class="text-muted-foreground truncate text-xs">{adminSession.admin?.email}</span>
		</span>
	</div>

	<nav class="flex min-h-0 flex-1 flex-col gap-0.5 px-3 pb-3" aria-label="Admin">
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
					<span
						class="bg-brand-soft text-brand tabular ml-auto rounded-full px-1.5 py-0.5 text-[11px] leading-none font-semibold"
						title="{item.count} new"
					>
						{item.count}
					</span>
				{/if}
			</a>
		{/each}
	</nav>

	<div class="border-sidebar-border flex shrink-0 flex-col gap-2 border-t p-3">
		<div class="flex items-center justify-between pr-0.5 pl-2.5">
			<span class="text-muted-foreground text-xs font-medium">Theme</span>
			<ThemeToggle />
		</div>
		<Button variant="ghost" class="text-muted-foreground justify-start" onclick={() => adminSession.signOut()}>
			<LogOutIcon data-icon="inline-start" />
			Sign out
		</Button>
	</div>
</div>
