<script lang="ts">
	import { toast } from 'svelte-sonner';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import XIcon from '@lucide/svelte/icons/x';
	import { listAllLeads, listLeads, type Lead } from '$lib/features/leads/api';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Table from '$lib/components/ui/table';
	import CardFilter from '$lib/features/cards/components/card-filter.svelte';
	import LeadSheet from './lead-sheet.svelte';
	import Pagination from '$lib/components/shared/pagination.svelte';
	import UserAvatar from '$lib/components/shared/user-avatar.svelte';
	import TeamPicker from '$lib/features/teams/components/team-picker.svelte';
	import UserPicker from '$lib/features/orgs/components/user-picker.svelte';
	import { ACCENTS, downloadBlob, initials, normalizeCard } from '$lib/features/cards/card';
	import { cards } from '$lib/features/cards/store.svelte';
	import { formatDateTime, plural, timeAgo } from '$lib/core/format';
	import { e164, formatPhone } from '$lib/core/phone';
	import { session } from '$lib/core/session.svelte';
	import { teams } from '$lib/features/teams/store.svelte';
	import { cn } from '$lib/utils';

	let {
		profileId,
		card = null,
		oncardchange,
		user = null,
		onuserchange,
		team = null,
		onteamchange,
		filename = 'leads'
	}: {
		/** Lock the table to one card's leads and hide the card filter. */
		profileId?: number;
		/** Selected card filter when not locked; null means all cards. */
		card?: number | null;
		oncardchange?: (card: number | null) => void;
		/** Admins: leads that arrived while this user held the card; 'none' for the organisation's; null for all. */
		user?: number | 'none' | null;
		onuserchange?: (user: number | 'none' | null) => void;
		/** Admins and team leads: leads of the people in this team; null for all. */
		team?: number | null;
		onteamchange?: (team: number | null) => void;
		/** CSV file name, without extension. */
		filename?: string;
	} = $props();

	// Admins and team leads see other people's leads, so they get the user column;
	// admins filter by anyone, leads by the teams they lead.
	const showUsers = $derived(session.seesOthers);
	const teamOptions = $derived(session.isAdmin ? (teams.list ?? []) : session.ledTeams);

	const SEARCH_DEBOUNCE_MS = 250;

	const lockedToCard = $derived(profileId !== undefined);
	const effectiveCard = $derived(profileId ?? card);

	let query = $state('');
	let search = $state('');
	let page = $state(1);
	let pageSize = $state(25);

	let leads = $state<Lead[] | null>(null);
	let total = $state(0);
	let loading = $state(false);
	let error = $state('');
	let exporting = $state(false);
	let reloadToken = $state(0);
	/** The lead whose details are open. */
	let openLead = $state<Lead | null>(null);

	// Debounce typing so each keystroke doesn't hit the server.
	$effect(() => {
		const q = query;
		const timer = setTimeout(() => (search = q.trim()), SEARCH_DEBOUNCE_MS);
		return () => clearTimeout(timer);
	});

	let lastFilterKey = '';
	let requestId = 0;
	$effect(() => {
		void reloadToken;
		const filterKey = JSON.stringify([effectiveCard, user, team, search, pageSize]);
		// A new filter starts from the first page.
		if (filterKey !== lastFilterKey) {
			const changed = lastFilterKey !== '';
			lastFilterKey = filterKey;
			if (changed && page !== 1) {
				page = 1;
				return;
			}
		}

		const id = ++requestId;
		loading = true;
		error = '';
		listLeads({
			profileId: effectiveCard ?? undefined,
			userId: user ?? undefined,
			teamId: team ?? undefined,
			q: search,
			page,
			pageSize
		})
			.then((res) => {
				if (id !== requestId) return;
				// The data shrank under us (e.g. deleted card); step back to the last page.
				if (res.leads.length === 0 && res.total > 0 && page > 1) {
					page = Math.ceil(res.total / pageSize);
					return;
				}
				leads = res.leads;
				total = res.total;
			})
			.catch((e) => {
				if (id === requestId) error = e instanceof Error ? e.message : 'Failed to load leads';
			})
			.finally(() => {
				if (id === requestId) loading = false;
			});
	});

	/** Refetches this page of leads, and the cards so the lead-count badges catch up too. */
	function refresh() {
		reloadToken++;
		if (session.username) cards.load(session.username, true);
	}

	const filtered = $derived(search !== '' || (!lockedToCard && card !== null) || user !== null || team !== null);
	const showCardColumn = $derived(!lockedToCard && card === null);

	// After paging, bring the top of the table back into view if it scrolled away.
	let tableTop: HTMLElement | undefined = $state();
	let lastPage = 1;
	$effect(() => {
		if (page === lastPage) return;
		lastPage = page;
		if (tableTop && tableTop.getBoundingClientRect().top < 0) {
			const smooth = !matchMedia('(prefers-reduced-motion: reduce)').matches;
			tableTop.scrollIntoView({ block: 'start', behavior: smooth ? 'smooth' : 'auto' });
		}
	});
	const cardById = $derived(new Map((cards.list ?? []).map((p) => [p.id, p])));

	function cardInfo(id: number) {
		const profile = cardById.get(id);
		if (!profile) return null;
		const c = normalizeCard(profile.data);
		return { name: c.name || profile.slug, slug: profile.slug, color: ACCENTS[c.accent] };
	}

	// Leads come from anonymous visitors; prefix cells that a spreadsheet would
	// treat as formulas so opening the export can't execute anything.
	function csvCell(value: string): string {
		const safe = /^[=+\-@\t\r]/.test(value) ? `'${value}` : value;
		return `"${safe.replace(/"/g, '""')}"`;
	}

	async function exportCsv() {
		exporting = true;
		try {
			const all = await listAllLeads({
				profileId: effectiveCard ?? undefined,
				userId: user ?? undefined,
				teamId: team ?? undefined,
				q: search
			});
			const header = ['Name', 'Email', 'Phone', 'Message'];
			if (!lockedToCard) header.push('Card');
			if (showUsers) header.push('User');
			header.push('Received');
			const rows = all.map((l) => {
				const base = [l.name, l.email, formatPhone(l.phone_country_code ?? '', l.phone_number ?? ''), l.notes];
				if (!lockedToCard) base.push(cardInfo(l.profile_id)?.name ?? '');
				if (showUsers) base.push(l.assigned_user?.username ?? '');
				return [...base, new Date(l.created_at).toISOString()];
			});
			const csv = [header, ...rows].map((r) => r.map(csvCell).join(',')).join('\r\n');
			downloadBlob(new Blob([csv], { type: 'text/csv;charset=utf-8' }), `${filename}.csv`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Export failed');
		} finally {
			exporting = false;
		}
	}
</script>

<div class="flex scroll-mt-4 flex-col gap-4" bind:this={tableTop}>
	<div class="flex flex-wrap items-center gap-2">
		{#if !lockedToCard}
			<CardFilter value={card} onchange={(v) => oncardchange?.(v)} />
		{/if}
		{#if onteamchange && teamOptions.length > 0}
			<TeamPicker value={team} options={teamOptions} onchange={onteamchange} />
		{/if}
		{#if session.isAdmin && onuserchange}
			<UserPicker value={user} onchange={onuserchange} noneLabel="Organisation" />
		{/if}
		<div class="relative order-last min-w-0 basis-full sm:order-none sm:max-w-sm sm:flex-1 sm:basis-auto">
			<SearchIcon class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
			<Input
				bind:value={query}
				placeholder="Search name, email, phone or message"
				class="pr-9 pl-9"
				aria-label="Search leads"
			/>
			{#if query}
				<button
					type="button"
					class="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2"
					onclick={() => (query = '')}
					aria-label="Clear search"
				>
					<XIcon class="size-4" />
				</button>
			{/if}
		</div>
		<div class="ml-auto flex items-center gap-3">
			<Button variant="outline" onclick={refresh} disabled={loading} aria-label="Refresh leads">
				<RefreshCwIcon data-icon="inline-start" class={cn(loading && 'animate-spin')} />
				<span class="max-sm:sr-only">Refresh</span>
			</Button>
			<Button variant="outline" onclick={exportCsv} disabled={exporting || total === 0}>
				{#if exporting}
					<Spinner data-icon="inline-start" />
				{:else}
					<DownloadIcon data-icon="inline-start" />
				{/if}
				<span class="max-sm:sr-only">Export CSV</span>
			</Button>
		</div>
	</div>

	{#if error && !leads}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load leads</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={() => reloadToken++}>Try again</Button>
		</div>
	{:else if leads === null}
		<div class="bg-card flex flex-col gap-5 rounded-xl border p-5">
			{#each [1, 2, 3, 4, 5] as i (i)}
				<div class="flex items-center gap-3">
					<Skeleton class="size-8 rounded-full" />
					<div class="flex flex-1 flex-col gap-1.5">
						<Skeleton class="h-3 w-40" />
						<Skeleton class="h-2.5 w-56" />
					</div>
					<Skeleton class="h-3 w-16" />
				</div>
			{/each}
		</div>
	{:else if total === 0 && !filtered}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-20">
			<Empty.Header>
				<Empty.Media variant="icon">
					<InboxIcon />
				</Empty.Media>
				<Empty.Title>No leads yet</Empty.Title>
				<Empty.Description>
					When someone shares their contact details from {lockedToCard
						? 'this card'
						: showUsers
							? 'one of your organisation’s cards'
							: 'one of your cards'}, they'll appear here.
				</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{:else}
		<div class={cn('bg-card overflow-hidden rounded-xl border transition-opacity', loading && 'opacity-60')}>
			<Table.Root>
				<Table.Header>
					<Table.Row class="bg-muted/40 hover:bg-muted/40">
						<Table.Head class="h-10 pl-5">Name</Table.Head>
						{#if showCardColumn}
							<Table.Head class="hidden h-10 md:table-cell">Card</Table.Head>
						{/if}
						{#if showUsers}
							<Table.Head class="hidden h-10 sm:table-cell">User</Table.Head>
						{/if}
						<Table.Head class="hidden h-10 lg:table-cell">Message</Table.Head>
						<Table.Head class="h-10 pr-5 text-right">Received</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each leads as lead (lead.id)}
						{@const info = showCardColumn ? cardInfo(lead.profile_id) : null}
						<Table.Row class="cursor-pointer" onclick={() => (openLead = lead)}>
							<Table.Cell class="py-3 pl-5">
								<div class="flex items-center gap-3">
									<Avatar.Root class="size-8 text-xs">
										<Avatar.Fallback class="bg-muted font-semibold">{initials(lead.name)}</Avatar.Fallback>
									</Avatar.Root>
									<div class="flex min-w-0 flex-col">
										<button
											type="button"
											class="truncate text-left font-medium hover:underline"
											onclick={(e) => {
												e.stopPropagation();
												openLead = lead;
											}}
											aria-label="View {lead.name}'s details"
										>
											{lead.name}
										</button>
										<a
											href="mailto:{lead.email}"
											onclick={(e) => e.stopPropagation()}
											class="text-muted-foreground hover:text-foreground truncate text-xs hover:underline"
										>
											{lead.email}
										</a>
										{#if lead.phone_number}
											<a
												href="tel:{e164(lead.phone_country_code ?? '', lead.phone_number)}"
												onclick={(e) => e.stopPropagation()}
												class="text-muted-foreground hover:text-foreground truncate text-xs tabular-nums hover:underline"
											>
												{formatPhone(lead.phone_country_code ?? '', lead.phone_number)}
											</a>
										{/if}
										{#if info}
											<span class="text-muted-foreground mt-1 flex items-center gap-1.5 text-xs md:hidden">
												<span class="size-2 shrink-0 rounded-full" style="background: {info.color}"></span>
												{info.name}
											</span>
										{/if}
										{#if lead.notes}
											<span class="text-foreground/80 mt-1 line-clamp-2 text-xs whitespace-normal lg:hidden"
												>{lead.notes}</span
											>
										{/if}
									</div>
								</div>
							</Table.Cell>
							{#if showCardColumn}
								<Table.Cell class="hidden py-3 md:table-cell">
									{#if info}
										<button
											type="button"
											class="hover:bg-muted -ml-1.5 flex max-w-52 items-start gap-2 rounded-md px-1.5 py-1 text-left"
											onclick={(e) => {
												e.stopPropagation();
												oncardchange?.(lead.profile_id);
											}}
											title="Show only this card's leads"
										>
											<span class="mt-1.5 size-2 shrink-0 rounded-full" style="background: {info.color}"></span>
											<span class="flex min-w-0 flex-col">
												<span class="truncate text-sm">{info.name}</span>
												<span class="text-muted-foreground truncate font-mono text-[11px]"
													>/p/{session.orgHandle}/{info.slug}</span
												>
											</span>
										</button>
									{/if}
								</Table.Cell>
							{/if}
							{#if showUsers}
								<Table.Cell class="hidden py-3 sm:table-cell">
									{#if lead.assigned_user}
										{@const held = lead.assigned_user}
										<button
											type="button"
											class="hover:bg-muted -ml-1.5 flex max-w-40 items-center gap-2 rounded-md px-1.5 py-1 text-left"
											onclick={(e) => {
												e.stopPropagation();
												onuserchange?.(held.id);
											}}
											title="Show only {held.username}’s leads"
										>
											<UserAvatar username={held.username} class="size-5 text-[9px]" />
											<span class="truncate text-sm">{held.username}</span>
										</button>
									{:else}
										<span class="text-muted-foreground text-sm">Organisation</span>
									{/if}
								</Table.Cell>
							{/if}
							<Table.Cell class="text-foreground/80 hidden max-w-xl py-3 whitespace-normal lg:table-cell">
								{#if lead.notes}
									<span class="line-clamp-2">{lead.notes}</span>
								{:else}
									<span class="text-muted-foreground">–</span>
								{/if}
							</Table.Cell>
							<Table.Cell
								class="text-muted-foreground py-3 pr-5 text-right text-xs whitespace-nowrap"
								title={formatDateTime(lead.created_at)}
							>
								{timeAgo(lead.created_at)}
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell
								colspan={3 + (showCardColumn ? 1 : 0) + (showUsers ? 1 : 0)}
								class="text-muted-foreground h-32 text-center"
							>
								No leads match{search ? ` “${search}”` : ' this filter'}.
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>

		{#if error}
			<p class="text-destructive text-sm">{error}</p>
		{/if}

		<Pagination bind:page bind:pageSize {total} disabled={loading} />
	{/if}
</div>

<LeadSheet bind:lead={openLead} card={openLead ? cardInfo(openLead.profile_id) : null} ondeleted={refresh} />

<span class="sr-only" aria-live="polite">{leads ? plural(total, 'lead') : ''}</span>
