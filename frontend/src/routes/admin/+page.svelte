<script lang="ts">
	import { getPlatformTrend, getSummary, type PlatformSummary, type UsagePoint } from '$lib/features/admin/api';
	import { formatBytes } from '$lib/features/files/api';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import RangeToggle from '$lib/features/admin/components/range-toggle.svelte';
	import TrendChart from '$lib/features/admin/components/trend-chart.svelte';
	import { plural } from '$lib/core/format';

	let summary = $state<PlatformSummary | null>(null);
	let error = $state('');
	let days = $state(30);
	let trend = $state<UsagePoint[] | null>(null);
	let trendError = $state('');

	async function loadSummary() {
		error = '';
		try {
			summary = await getSummary();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		}
	}
	loadSummary();

	$effect(() => {
		const range = days;
		trendError = '';
		getPlatformTrend(range)
			.then((res) => {
				if (range === days) trend = res.points;
			})
			.catch((e) => (trendError = e instanceof Error ? e.message : 'Failed to load trends'));
	});

	const stats = $derived(
		summary
			? [
					{
						label: 'Organisations',
						value: summary.org_count.toLocaleString(),
						note: `${summary.new_orgs_30d} new in 30 days${summary.suspended_org_count ? `, ${summary.suspended_org_count} suspended` : ''}`,
						href: '/admin/orgs'
					},
					{
						label: 'Active organisations',
						value: summary.active_orgs_30d.toLocaleString(),
						note: 'Someone signed in within 30 days'
					},
					{ label: 'Users', value: summary.user_count.toLocaleString(), note: 'Across every organisation' },
					{
						label: 'Teams',
						value: summary.team_count.toLocaleString(),
						note: `${summary.orgs_with_teams} of ${summary.org_count} organisations use teams`
					},
					{ label: 'Cards', value: summary.card_count.toLocaleString(), note: plural(summary.lead_count, 'lead') + ' captured' },
					{
						label: 'Storage used',
						value: formatBytes(summary.storage_used_bytes),
						note: `${summary.orgs_with_storage} of ${summary.org_count} connected a bucket`
					},
					{
						label: 'Branding',
						value: summary.orgs_with_logo.toLocaleString(),
						note: `of ${summary.org_count} organisations set a logo`
					},
					{
						label: 'New feedback',
						value: summary.new_feedback.toLocaleString(),
						note: 'Waiting to be read',
						href: '/admin/feedback'
					}
				]
			: null
	);

	const series = (key: keyof UsagePoint) => (trend ?? []).map((p) => ({ date: p.date, value: Number(p[key] ?? 0) }));
	const count = (n: number) => Math.round(n).toLocaleString();
</script>

<svelte:head>
	<title>Overview · Fronko admin</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-8 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Overview</h1>
		<p class="text-muted-foreground text-sm">
			How Fronko is used, in totals only. Card contents, leads, files and team members stay private to each organisation.
		</p>
	</header>

	{#if error}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load the overview</p>
			<p class="text-muted-foreground text-sm">{error}</p>
			<Button variant="outline" onclick={loadSummary}>Try again</Button>
		</div>
	{:else}
		<dl class="bg-border grid grid-cols-2 gap-px overflow-hidden rounded-xl border lg:grid-cols-4">
			{#each stats ?? Array.from({ length: 8 }, (_, i) => ({ label: String(i), value: '', note: '', href: undefined })) as stat (stat.label)}
				<div class="bg-card relative flex flex-col gap-1 px-4 py-4 sm:px-6 sm:py-5">
					{#if stats}
						<dt class="text-muted-foreground text-xs sm:text-sm">
							{#if stat.href}
								<a href={stat.href} class="hover:text-foreground after:absolute after:inset-0">{stat.label}</a>
							{:else}
								{stat.label}
							{/if}
						</dt>
						<dd class="text-2xl font-semibold tracking-tight sm:text-3xl">{stat.value}</dd>
						<dd class="text-muted-foreground text-xs">{stat.note}</dd>
					{:else}
						<Skeleton class="h-3 w-20" />
						<Skeleton class="mt-2 h-7 w-14" />
						<Skeleton class="mt-1 h-2.5 w-28" />
					{/if}
				</div>
			{/each}
		</dl>
	{/if}

	<section class="flex flex-col gap-4" aria-labelledby="trends-heading">
		<div class="flex flex-wrap items-end justify-between gap-3">
			<div class="flex flex-col gap-0.5">
				<h2 id="trends-heading" class="text-lg font-semibold tracking-tight">Trends</h2>
				<p class="text-muted-foreground text-sm">Totals at the end of each day (UTC). Today's point updates hourly.</p>
			</div>
			<RangeToggle bind:days />
		</div>
		{#if trendError}
			<p class="text-destructive text-sm">{trendError}</p>
		{:else if trend === null}
			<div class="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
				{#each [1, 2, 3, 4, 5, 6, 7, 8] as i (i)}
					<Skeleton class="h-[250px] rounded-xl" />
				{/each}
			</div>
		{:else}
			<div class="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
				<TrendChart label="Organisations" points={series('org_count')} format={count} />
				<TrendChart label="Users" points={series('user_count')} format={count} />
				<TrendChart label="Cards" points={series('card_count')} format={count} />
				<TrendChart label="Leads" points={series('lead_count')} format={count} />
				<TrendChart label="Storage used" points={series('storage_used_bytes')} format={formatBytes} />
				<TrendChart label="Organisations with storage" points={series('orgs_with_storage')} format={count} />
				<TrendChart label="Teams" points={series('team_count')} format={count} />
				<TrendChart label="Organisations with a logo" points={series('orgs_with_logo')} format={count} />
			</div>
		{/if}
	</section>
</div>
