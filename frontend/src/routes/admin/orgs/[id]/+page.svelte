<script lang="ts">
	import { page } from '$app/state';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import BanIcon from '@lucide/svelte/icons/ban';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import { getOrg, getOrgTrend, type OrgUsage, type UsagePoint } from '$lib/api/admin';
	import { ApiError } from '$lib/api/client';
	import { formatBytes, PURPOSES, type FilePurpose } from '$lib/api/files';
	import * as Alert from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import RangeToggle from '$lib/components/app/admin/range-toggle.svelte';
	import StorageCell from '$lib/components/app/admin/storage-cell.svelte';
	import SuspendOrgDialog from '$lib/components/app/admin/suspend-org-dialog.svelte';
	import TrendChart from '$lib/components/app/admin/trend-chart.svelte';
	import { formatDateTime, plural, timeAgo } from '$lib/format';

	const id = $derived(Number(page.params.id));

	let org = $state<OrgUsage | null>(null);
	let error = $state('');
	let notFound = $state(false);
	let days = $state(30);
	let trend = $state<UsagePoint[] | null>(null);
	let dialogOpen = $state(false);

	async function load() {
		error = '';
		try {
			org = await getOrg(id);
		} catch (e) {
			if (e instanceof ApiError && e.status === 404) notFound = true;
			else error = e instanceof Error ? e.message : 'Failed to load';
		}
	}

	$effect(() => {
		void id;
		load();
	});

	$effect(() => {
		const range = days;
		getOrgTrend(id, range)
			.then((res) => range === days && (trend = res.points))
			.catch(() => (trend = []));
	});

	const series = (key: keyof UsagePoint) => (trend ?? []).map((p) => ({ date: p.date, value: Number(p[key] ?? 0) }));
	const count = (n: number) => Math.round(n).toLocaleString();

	// "5 brochures, 2 logos": the biggest purposes first, counts only.
	function purposeNote(byPurpose: Record<string, number>): string {
		return Object.entries(byPurpose)
			.sort((a, b) => b[1] - a[1])
			.map(([p, n]) => `${n} ${(PURPOSES[p as FilePurpose]?.[n === 1 ? 'label' : 'plural'] ?? p).toLowerCase()}`)
			.join(', ');
	}

	const facts = $derived(
		org
			? [
					{
						label: 'Users',
						value: org.user_count.toLocaleString(),
						note: [
							'1 owner',
							org.admin_count && plural(org.admin_count, 'admin'),
							org.member_count && plural(org.member_count, 'member'),
							org.suspended_user_count && `${org.suspended_user_count} suspended`
						]
							.filter(Boolean)
							.join(', ')
					},
					{ label: 'Teams', value: org.team_count.toLocaleString(), note: org.team_count ? '' : 'Not using teams' },
					{ label: 'Cards', value: org.card_count.toLocaleString(), note: '' },
					{ label: 'Leads', value: org.lead_count.toLocaleString(), note: 'Captured on its cards' },
					{
						label: 'Files',
						value: org.file_count.toLocaleString(),
						note: [formatBytes(org.storage_used_bytes) + ' in total', purposeNote(org.files_by_purpose)].filter(Boolean).join(' · ')
					},
					{
						label: 'Branding',
						value: org.logo_set ? 'Logo set' : 'No logo',
						note: org.logo_set
							? `${org.logo_policy === 'required' ? 'Required' : 'Optional'} on cards${org.signature_locked ? ', signature template locked' : ''}`
							: org.signature_locked
								? 'Signature template locked'
								: ''
					}
				]
			: []
	);
</script>

<svelte:head>
	<title>{org?.name ?? 'Organisation'} · Fronko admin</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-8 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<a href="/admin/orgs" class="text-muted-foreground hover:text-foreground flex w-fit items-center gap-1.5 text-sm">
		<ArrowLeftIcon class="size-4" />
		Organisations
	</a>

	{#if notFound}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Organisation not found</p>
			<p class="text-muted-foreground text-sm">It may have been deleted.</p>
		</div>
	{:else if error}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load this organisation</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={load}>Try again</Button>
		</div>
	{:else if !org}
		<Skeleton class="h-10 w-64" />
		<Skeleton class="h-28 w-full rounded-xl" />
	{:else}
		<header class="flex flex-wrap items-end justify-between gap-4">
			<div class="flex min-w-0 flex-col gap-1">
				<h1 class="flex flex-wrap items-center gap-3 text-2xl font-semibold tracking-tight sm:text-3xl">
					<span class="truncate">{org.name}</span>
					{#if org.suspended_at}<Badge variant="destructive">Suspended</Badge>{/if}
				</h1>
				<p class="text-muted-foreground text-sm">
					Owner {org.owner_email ?? '(no email)'} · created {formatDateTime(org.created_at)} · last active
					{org.last_active_at ? timeAgo(org.last_active_at) : 'never'}
				</p>
			</div>
			{#if org.suspended_at}
				<Button variant="outline" onclick={() => (dialogOpen = true)}>
					<RotateCcwIcon data-icon="inline-start" />
					Reinstate
				</Button>
			{:else}
				<Button variant="outline" class="text-destructive" onclick={() => (dialogOpen = true)}>
					<BanIcon data-icon="inline-start" />
					Suspend
				</Button>
			{/if}
		</header>

		{#if org.suspended_at}
			<Alert.Root variant="destructive">
				<BanIcon />
				<Alert.Title>Suspended {timeAgo(org.suspended_at)}</Alert.Title>
				<Alert.Description>
					{#if org.suspended_reason}<p>Reason: {org.suspended_reason}</p>{/if}
					<p>Nobody in it can sign in, and its public cards are offline.</p>
				</Alert.Description>
			</Alert.Root>
		{/if}

		<dl class="bg-border grid grid-cols-2 gap-px overflow-hidden rounded-xl border lg:grid-cols-4">
			{#each facts as f (f.label)}
				<div class="bg-card flex flex-col gap-1 px-4 py-4 sm:px-6 sm:py-5">
					<dt class="text-muted-foreground text-xs sm:text-sm">{f.label}</dt>
					<dd class="text-2xl font-semibold tracking-tight sm:text-3xl">{f.value}</dd>
					{#if f.note}<dd class="text-muted-foreground text-xs">{f.note}</dd>{/if}
				</div>
			{/each}
			<div class="bg-card col-span-2 flex flex-col gap-1 px-4 py-4 sm:px-6 sm:py-5">
				<dt class="text-muted-foreground text-xs sm:text-sm">Storage</dt>
				<dd class="text-lg font-semibold tracking-tight"><StorageCell {org} /></dd>
				<dd class="text-muted-foreground text-xs">
					New users get {org.default_quota_bytes === null ? 'unlimited space' : formatBytes(org.default_quota_bytes)}
				</dd>
			</div>
		</dl>

		<section class="flex flex-col gap-4" aria-labelledby="org-trends-heading">
			<div class="flex flex-wrap items-end justify-between gap-3">
				<h2 id="org-trends-heading" class="text-lg font-semibold tracking-tight">Trends</h2>
				<RangeToggle bind:days />
			</div>
			{#if trend === null}
				<div class="grid gap-4 md:grid-cols-2">
					{#each [1, 2, 3, 4, 5] as i (i)}<Skeleton class="h-[250px] rounded-xl" />{/each}
				</div>
			{:else}
				<div class="grid gap-4 md:grid-cols-2">
					<TrendChart label="Users" points={series('user_count')} format={count} />
					<TrendChart label="Cards" points={series('card_count')} format={count} />
					<TrendChart label="Leads" points={series('lead_count')} format={count} />
					<TrendChart label="Storage used" points={series('storage_used_bytes')} format={formatBytes} />
					<TrendChart label="Teams" points={series('team_count')} format={count} />
				</div>
			{/if}
		</section>

		<SuspendOrgDialog {org} bind:open={dialogOpen} onchanged={(o) => (org = o)} />
	{/if}
</div>
