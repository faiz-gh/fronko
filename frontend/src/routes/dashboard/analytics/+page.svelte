<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import ArrowDownRightIcon from '@lucide/svelte/icons/arrow-down-right';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import IdCardIcon from '@lucide/svelte/icons/id-card';
	import ImageIcon from '@lucide/svelte/icons/image';
	import MousePointerClickIcon from '@lucide/svelte/icons/mouse-pointer-click';
	import UserIcon from '@lucide/svelte/icons/user';
	import {
		SOURCES,
		change,
		formatCount,
		formatDuration,
		getAnalyticsCards,
		getAnalyticsContent,
		getAnalyticsMembers,
		getAnalyticsSummary,
		getAnalyticsTeams,
		getAnalyticsTimeseries,
		lastDays,
		rate,
		type AnalyticsPoint,
		type AnalyticsSummary,
		type CardStat,
		type ContentStat,
		type MemberStat,
		type TeamStat
	} from '$lib/features/analytics/api';
	import { teamColor } from '$lib/features/teams/api';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import BarList from '$lib/components/shared/charts/bar-list.svelte';
	import Funnel from '$lib/components/shared/charts/funnel.svelte';
	import Heatmap from '$lib/components/shared/charts/heatmap.svelte';
	import SeriesChart from '$lib/components/shared/charts/series-chart.svelte';
	import SplitBar from '$lib/components/shared/charts/split-bar.svelte';
	import StatTile from '$lib/components/shared/charts/stat-tile.svelte';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import FilterSelect from '$lib/components/shared/filter-select.svelte';
	import TeamPicker from '$lib/features/teams/components/team-picker.svelte';
	import UserAvatar from '$lib/components/shared/user-avatar.svelte';
	import { normalizeCard } from '$lib/features/cards/card';
	import { cards } from '$lib/features/cards/store.svelte';
	import { formatDateTime, timeAgo } from '$lib/core/format';
	import { session } from '$lib/core/session.svelte';
	import { teams } from '$lib/features/teams/store.svelte';
	import { cn } from '$lib/utils';

	const RANGES = [
		{ value: '7', label: '7 days' },
		{ value: '30', label: '30 days' },
		{ value: '90', label: '90 days' },
		{ value: '365', label: '12 months' }
	];
	const INACTIVE_DAYS = 30;

	// ---- Filters, kept in the URL so a view can be shared or bookmarked --------
	const params = $derived(page.url.searchParams);
	const range = $derived(RANGES.some((r) => r.value === params.get('range')) ? params.get('range')! : '30');
	const idParam = (name: string) => {
		const n = Number(params.get(name));
		return Number.isInteger(n) && n > 0 ? n : null;
	};
	const teamId = $derived(idParam('team'));
	const userId = $derived(idParam('user'));
	const profileId = $derived(idParam('card'));

	const tabs = $derived([
		{ value: 'engagement', label: 'Engagement' },
		{ value: 'content', label: 'Content' },
		{ value: 'cards', label: 'Cards' },
		...(session.seesOthers ? [{ value: 'teams', label: 'Teams' }] : [])
	]);
	const tab = $derived(tabs.some((t) => t.value === params.get('tab')) ? params.get('tab')! : 'engagement');

	function setParam(name: string, value: string | number | null) {
		const next = new URLSearchParams(page.url.search);
		if (value === null || value === '') next.delete(name);
		else next.set(name, String(value));
		const qs = next.toString();
		goto(`/dashboard/analytics${qs ? `?${qs}` : ''}`, { replace: true, reset: false });
	}

	const teamOptions = $derived(session.isAdmin ? (teams.list ?? []) : session.ledTeams);
	const cardOptions = $derived(
		(cards.list ?? []).map((p) => ({ value: p.id, label: normalizeCard(p.data).name || p.slug }))
	);

	// ---- Data ------------------------------------------------------------------
	let summary = $state<AnalyticsSummary | null>(null);
	let points = $state<AnalyticsPoint[]>([]);
	let content = $state<ContentStat[]>([]);
	let cardStats = $state<CardStat[]>([]);
	let teamStats = $state<TeamStat[]>([]);
	let members = $state<MemberStat[]>([]);
	let people = $state<MemberStat[]>([]);
	let loading = $state(true);
	let error = $state('');

	let loadedKey = '';
	$effect(() => {
		const key = [range, teamId, userId, profileId].join('|');
		if (key === loadedKey) return;
		loadedKey = key;
		load(key);
	});

	async function load(key: string) {
		loading = true;
		error = '';
		const period = lastDays(Number(range));
		const q = { ...period, teamId: teamId ?? undefined, userId: userId ?? undefined, profileId: profileId ?? undefined };
		try {
			const [s, ts, c, cs, ts2, ms, ps] = await Promise.all([
				getAnalyticsSummary(q),
				getAnalyticsTimeseries(q),
				getAnalyticsContent(q),
				getAnalyticsCards(q),
				session.seesOthers ? getAnalyticsTeams({ ...period, teamId: teamId ?? undefined }) : Promise.resolve([]),
				session.seesOthers ? getAnalyticsMembers({ ...period, teamId: teamId ?? undefined }) : Promise.resolve([]),
				session.seesOthers ? getAnalyticsMembers(period) : Promise.resolve([])
			]);
			if (key !== loadedKey) return; // a newer filter won
			summary = s;
			points = ts;
			content = c;
			cardStats = cs;
			teamStats = ts2;
			members = ms;
			people = ps;
		} catch (e) {
			if (key === loadedKey) error = e instanceof Error ? e.message : 'Failed to load analytics';
		} finally {
			if (key === loadedKey) loading = false;
		}
	}

	// ---- Derived numbers -------------------------------------------------------
	const cur = $derived(summary?.current);
	const prev = $derived(summary?.previous);
	const dates = $derived(points.map((p) => p.date));
	const pct = (n: number | null) => (n === null ? '–' : `${Math.round(n)}%`);

	const kpis = $derived(
		cur && prev
			? [
					{
						label: 'Views',
						value: formatCount(cur.views),
						delta: change(cur.views, prev.views),
						trend: points.map((p) => p.views)
					},
					{
						label: 'Unique visitors',
						value: formatCount(cur.unique_visitors),
						delta: change(cur.unique_visitors, prev.unique_visitors),
						trend: points.map((p) => p.unique_visitors)
					},
					{
						label: 'Contacts saved',
						value: formatCount(cur.saves),
						delta: change(cur.saves, prev.saves),
						trend: points.map((p) => p.saves)
					},
					{
						label: 'Leads',
						value: formatCount(cur.leads),
						delta: change(cur.leads, prev.leads),
						trend: points.map((p) => p.leads)
					},
					{
						label: 'Engaged visits',
						value: pct(rate(cur.engaged_sessions, cur.sessions)),
						delta: null,
						hint: `${formatCount(cur.engaged_sessions)} of ${formatCount(cur.sessions)} visits`
					},
					{
						label: 'Time on card',
						value: formatDuration(cur.median_time_ms),
						delta: null,
						hint: 'Median per visit'
					}
				]
			: []
	);

	const sourceSeries = $derived(
		SOURCES.map((s) => ({
			key: s.key,
			label: s.label,
			color: s.color,
			values: points.map((p) => (s.key === 'nfc' ? p.nfc_views : s.key === 'qr' ? p.qr_views : p.link_views))
		}))
	);
	const sourceParts = $derived(
		cur
			? SOURCES.map((s) => ({
					key: s.key,
					label: s.label,
					color: s.color,
					value: s.key === 'nfc' ? cur.nfc_views : s.key === 'qr' ? cur.qr_views : cur.link_views
				}))
			: []
	);
	const outcomeSeries = $derived([
		{ key: 'saves', label: 'Contacts saved', color: 'var(--viz-1)', values: points.map((p) => p.saves) },
		{ key: 'leads', label: 'Leads', color: 'var(--viz-2)', values: points.map((p) => p.leads) }
	]);

	const funnel = $derived(
		cur
			? [
					{ label: 'Visits', value: cur.sessions },
					{ label: 'Engaged', value: cur.engaged_sessions, hint: 'Clicked, opened, saved, shared or scrolled half way' },
					{ label: 'Saved the contact or opened the form', value: cur.action_sessions },
					{ label: 'Sent their details (leads)', value: cur.leads }
				]
			: []
	);

	const deviceParts = $derived([
		{ key: 'mobile', label: 'Phone', value: summary?.devices.mobile ?? 0, color: 'var(--viz-1)' },
		{ key: 'tablet', label: 'Tablet', value: summary?.devices.tablet ?? 0, color: 'var(--viz-2)' },
		{ key: 'desktop', label: 'Computer', value: summary?.devices.desktop ?? 0, color: 'var(--viz-3)' }
	]);
	const leadSourceParts = $derived([
		...SOURCES.map((s) => ({ key: s.key, label: s.label, color: s.color, value: summary?.lead_sources[s.key] ?? 0 })),
		...(summary?.lead_sources.unknown
			? [{ key: 'unknown', label: 'Before tracking', color: 'var(--muted-foreground)', value: summary.lead_sources.unknown }]
			: [])
	]);
	const depthLabels = ['Less than 25%', '25%', '50%', '75%', 'The whole card'];
	const timeLabels = ['Under 10s', '10–30s', '30s–1m', '1–3m', 'Over 3m'];

	// Content
	const QUICK_LABELS: Record<string, string> = { email: 'Email', call: 'Call', website: 'Website', booking: 'Book a meeting' };
	const isUrl = (t: string) => t.includes('://');
	const links = $derived(content.filter((c) => c.type === 'click' && isUrl(c.target)));
	const quick = $derived(content.filter((c) => c.type === 'click' && !isUrl(c.target)));
	const docs = $derived(content.filter((c) => c.type === 'doc_open'));
	const images = $derived(content.filter((c) => c.type === 'gallery_open'));
	const perVisitor = (c: ContentStat) => (c.unique > 0 ? (c.count / c.unique).toFixed(1) : '–');

	// Cards
	type CardSort = 'views' | 'unique_visitors' | 'saves' | 'form_opens' | 'leads' | 'doc_opens';
	let cardSort = $state<CardSort>('views');
	const sortedCards = $derived([...cardStats].sort((a, b) => b[cardSort] - a[cardSort]));
	const inactive = $derived(
		cardStats.filter(
			(c) => !c.last_viewed_at || Date.now() - new Date(c.last_viewed_at).getTime() > INACTIVE_DAYS * 86_400_000
		)
	);

	// Teams
	type MemberSort = 'views' | 'saves' | 'leads';
	let memberSort = $state<MemberSort>('views');
	const sortedMembers = $derived([...members].sort((a, b) => b[memberSort] - a[memberSort]));
	const maxTeamViews = $derived(Math.max(1, ...teamStats.map((t) => t.views)));
</script>

<svelte:head>
	<title>Analytics · Fronko</title>
</svelte:head>

{#snippet delta(curV: number, prevV: number)}
	{@const d = change(curV, prevV)}
	{#if d !== null && Math.round(d) !== 0}
		<span
			class={cn(
				'inline-flex items-center text-xs',
				d > 0 ? 'text-emerald-700 dark:text-emerald-400' : 'text-rose-700 dark:text-rose-400'
			)}
			title="vs previous period"
		>
			{#if d > 0}<ArrowUpRightIcon class="size-3" />{:else}<ArrowDownRightIcon class="size-3" />{/if}{Math.abs(Math.round(d))}%
		</span>
	{/if}
{/snippet}

{#snippet sortHead(label: string, active: boolean, onclick: () => void)}
	<button type="button" class={cn('inline-flex items-center gap-1', active && 'text-foreground')} {onclick}>
		{label}
		{#if active}<ArrowDownIcon class="size-3" />{/if}
	</button>
{/snippet}

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<Tabs.Root value={tab} onValueChange={(v) => setParam('tab', v === 'engagement' ? null : v)} class="gap-6">
		<header class="flex flex-col gap-4 border-b">
			<div class="flex flex-wrap items-end justify-between gap-4">
				<div class="flex flex-col gap-1">
					<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Analytics</h1>
					<p class="text-muted-foreground text-sm">
						How people find and use
						{session.isAdmin ? `${session.orgName}'s cards` : session.isLead ? 'your and your teams’ cards' : 'your cards'}:
						taps, scans, clicks, saves and leads.
					</p>
				</div>
				<ToggleGroup.Root
					type="single"
					variant="outline"
					size="sm"
					value={range}
					onValueChange={(v) => v && setParam('range', v === '30' ? null : v)}
					aria-label="Period"
				>
					{#each RANGES as r (r.value)}
						<ToggleGroup.Item value={r.value} class="px-3">{r.label}</ToggleGroup.Item>
					{/each}
				</ToggleGroup.Root>
			</div>

			<!-- Filters sit in one row above every chart they affect. -->
			<div class="flex flex-wrap items-center gap-2">
				{#if session.seesOthers && teamOptions.length > 0}
					<TeamPicker size="sm" value={teamId} options={teamOptions} onchange={(v) => setParam('team', v)} />
				{/if}
				{#if session.seesOthers && people.length > 1}
					<FilterSelect
						size="sm"
						icon={UserIcon}
						allLabel="Everyone"
						value={userId}
						options={people.map((p) => ({ value: p.user_id, label: p.username }))}
						onchange={(v) => setParam('user', v)}
					/>
				{/if}
				{#if cardOptions.length > 1}
					<FilterSelect
						size="sm"
						icon={IdCardIcon}
						allLabel="All cards"
						value={profileId}
						options={cardOptions}
						onchange={(v) => setParam('card', v)}
					/>
				{/if}
				{#if teamId || userId || profileId}
					<Button
						variant="ghost"
						size="sm"
						onclick={() => goto(`/dashboard/analytics${tab !== 'engagement' ? `?tab=${tab}` : ''}`, { replace: true, reset: false })}
					>
						Clear filters
					</Button>
				{/if}
			</div>

			<Tabs.List variant="line" class="-mb-px h-10 gap-4 p-0">
				{#each tabs as t (t.value)}
					<Tabs.Trigger value={t.value} class="flex-none px-0.5">{t.label}</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</header>

		{#if error}
			<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
				<p class="font-medium">Couldn't load analytics</p>
				<p class="text-muted-foreground text-sm">{error}</p>
				<Button variant="outline" onclick={() => load(loadedKey)}>Try again</Button>
			</div>
		{:else if !summary}
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-6">
				{#each Array.from({ length: 6 }, (_, i) => i) as i (i)}<Skeleton class="h-32 rounded-xl" />{/each}
			</div>
			<Skeleton class="h-80 rounded-xl" />
		{:else}
			<div class={cn('flex flex-col gap-6 transition-opacity', loading && 'opacity-60')}>
				<Tabs.Content value="engagement" class="flex flex-col gap-6">
					<div class="grid grid-cols-2 gap-4 md:grid-cols-3 2xl:grid-cols-6">
						{#each kpis as k (k.label)}
							<StatTile label={k.label} value={k.value} delta={k.delta} hint={k.hint} trend={k.trend} />
						{/each}
					</div>

					<div class="grid items-start gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
						<section class="bg-card flex flex-col gap-4 rounded-xl border p-5" aria-labelledby="sources-heading">
							<div class="flex flex-col gap-1">
								<h2 id="sources-heading" class="font-semibold">Views by source</h2>
								<p class="text-muted-foreground text-sm">NFC taps and QR scans come from the tag and code; links are everything else.</p>
							</div>
							<SeriesChart {dates} series={sourceSeries} label="Views by source" />
						</section>
						<section class="bg-card flex flex-col gap-4 rounded-xl border p-5" aria-labelledby="funnel-heading">
							<div class="flex flex-col gap-1">
								<h2 id="funnel-heading" class="font-semibold">From visit to lead</h2>
								<p class="text-muted-foreground text-sm">
									{pct(rate(cur!.leads, cur!.sessions))} of visits ended in a lead.
								</p>
							</div>
							<Funnel steps={funnel} />
						</section>
					</div>

					<div class="grid items-start gap-6 lg:grid-cols-2 2xl:grid-cols-3">
						<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
							<h2 class="font-semibold">How visitors arrive</h2>
							<SplitBar parts={sourceParts} label="Views by source" />
							<h3 class="mt-2 text-sm font-medium">Devices</h3>
							<SplitBar parts={deviceParts} label="Visits by device" />
							<h3 class="mt-2 text-sm font-medium">Where leads came from</h3>
							<SplitBar parts={leadSourceParts} label="Leads by source" />
						</section>
						<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
							<div class="flex flex-col gap-1">
								<h2 class="font-semibold">How far they scroll</h2>
								<p class="text-muted-foreground text-sm">
									Average {Math.round(cur!.avg_scroll_depth)}% of the card, on cards taller than the screen.
								</p>
							</div>
							<BarList
								label="Visits by scroll depth"
								items={summary.scroll_depths.map((v, i) => ({ key: String(i), label: depthLabels[i], value: v }))}
							/>
							<h3 class="mt-2 text-sm font-medium">Time on card</h3>
							<BarList
								label="Visits by time on card"
								color="var(--viz-2)"
								items={summary.time_buckets.map((v, i) => ({ key: String(i), label: timeLabels[i], value: v }))}
							/>
						</section>
						<section class="bg-card flex flex-col gap-4 rounded-xl border p-5 lg:col-span-2 2xl:col-span-1">
							<div class="flex flex-col gap-1">
								<h2 class="font-semibold">When cards are viewed</h2>
								<p class="text-muted-foreground text-sm">In your time zone.</p>
							</div>
							<Heatmap data={summary.heatmap} />
							<dl class="mt-2 grid grid-cols-3 gap-3 text-sm">
								<div><dt class="text-muted-foreground text-xs">Came back same day</dt><dd class="font-medium tabular">{formatCount(cur!.repeat_visitors)}</dd></div>
								<div><dt class="text-muted-foreground text-xs">Shares</dt><dd class="font-medium tabular">{formatCount(cur!.shares)}</dd></div>
								<div><dt class="text-muted-foreground text-xs">Forms opened</dt><dd class="font-medium tabular">{formatCount(cur!.form_opens)}</dd></div>
							</dl>
						</section>
					</div>

					<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
						<h2 class="font-semibold">Saves and leads</h2>
						<SeriesChart {dates} series={outcomeSeries} label="Contacts saved and leads" height={180} />
					</section>
				</Tabs.Content>

				<Tabs.Content value="content" class="grid items-start gap-6 lg:grid-cols-2">
					<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
						<div class="flex items-center gap-2">
							<MousePointerClickIcon class="text-muted-foreground size-4" />
							<h2 class="font-semibold">Links</h2>
							<span class="text-muted-foreground ml-auto text-xs">clicks · unique</span>
						</div>
						<BarList
							label="Clicks per link"
							empty="No link clicks in this period"
							items={links.map((c) => ({ key: c.target, label: c.label || c.target, value: c.count, sub: `${c.unique} unique` }))}
						>
							{#snippet icon(item)}<BrandIcon url={item.key} class="text-muted-foreground size-3.5 shrink-0 self-center" />{/snippet}
						</BarList>
					</section>
					<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
						<div class="flex items-center gap-2">
							<MousePointerClickIcon class="text-muted-foreground size-4" />
							<h2 class="font-semibold">Quick actions</h2>
							<span class="text-muted-foreground ml-auto text-xs">taps · unique</span>
						</div>
						<BarList
							label="Taps per quick action"
							color="var(--viz-2)"
							empty="No one used Email, Call, Website or Book a meeting yet"
							items={quick.map((c) => ({ key: c.target, label: QUICK_LABELS[c.target] ?? c.label, value: c.count, sub: `${c.unique} unique` }))}
						/>
						<p class="text-muted-foreground text-xs">
							Contacts saved: <span class="text-foreground font-medium">{formatCount(cur!.saves)}</span> · Shares:
							<span class="text-foreground font-medium">{formatCount(cur!.shares)}</span>
						</p>
					</section>
					<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
						<div class="flex items-center gap-2">
							<FileTextIcon class="text-muted-foreground size-4" />
							<h2 class="font-semibold">Brochures</h2>
						</div>
						{#if docs.length === 0}
							<p class="text-muted-foreground py-6 text-center text-sm">No brochure opens in this period</p>
						{:else}
							<Table.Root>
								<Table.Header>
									<Table.Row>
										<Table.Head>Brochure</Table.Head>
										<Table.Head class="text-right">Opens</Table.Head>
										<Table.Head class="text-right">People</Table.Head>
										<Table.Head class="text-right">Opens each</Table.Head>
									</Table.Row>
								</Table.Header>
								<Table.Body>
									{#each docs as d (d.target)}
										<Table.Row>
											<Table.Cell class="max-w-64 truncate font-medium">{d.label || 'Brochure'}</Table.Cell>
											<Table.Cell class="text-right tabular">{d.count}</Table.Cell>
											<Table.Cell class="text-right tabular">{d.unique}</Table.Cell>
											<Table.Cell class="text-right tabular">{perVisitor(d)}</Table.Cell>
										</Table.Row>
									{/each}
								</Table.Body>
							</Table.Root>
						{/if}
					</section>
					<section class="bg-card flex flex-col gap-4 rounded-xl border p-5">
						<div class="flex items-center gap-2">
							<ImageIcon class="text-muted-foreground size-4" />
							<h2 class="font-semibold">Gallery images</h2>
						</div>
						<BarList
							label="Opens per image"
							color="var(--viz-3)"
							empty="No image opens in this period"
							items={images.map((c) => ({ key: c.target, label: c.label || 'Image', value: c.count, sub: `${c.unique} unique` }))}
						/>
					</section>
				</Tabs.Content>

				<Tabs.Content value="cards" class="flex flex-col gap-6">
					{#if inactive.length > 0}
						<div class="bg-card flex flex-col gap-2 rounded-xl border border-dashed p-5">
							<p class="font-medium">
								{inactive.length === 1 ? '1 card hasn’t' : `${inactive.length} cards haven’t`} been viewed in {INACTIVE_DAYS} days
							</p>
							<p class="text-muted-foreground text-sm">
								{inactive
									.slice(0, 6)
									.map((c) => c.name || c.slug)
									.join(', ')}{inactive.length > 6 ? ', …' : ''}. Check the NFC tag and QR code are being handed out.
							</p>
						</div>
					{/if}
					<div class="bg-card overflow-hidden rounded-xl border">
						<Table.Root>
							<Table.Header>
								<Table.Row>
									<Table.Head>Card</Table.Head>
									{#if session.seesOthers}<Table.Head>Held by</Table.Head>{/if}
									<Table.Head class="text-right">{@render sortHead('Views', cardSort === 'views', () => (cardSort = 'views'))}</Table.Head>
									<Table.Head class="text-right">{@render sortHead('Unique', cardSort === 'unique_visitors', () => (cardSort = 'unique_visitors'))}</Table.Head>
									<Table.Head class="text-right">NFC · QR · Link</Table.Head>
									<Table.Head class="text-right">{@render sortHead('Saves', cardSort === 'saves', () => (cardSort = 'saves'))}</Table.Head>
									<Table.Head class="text-right">{@render sortHead('Brochures', cardSort === 'doc_opens', () => (cardSort = 'doc_opens'))}</Table.Head>
									<Table.Head class="text-right">{@render sortHead('Forms', cardSort === 'form_opens', () => (cardSort = 'form_opens'))}</Table.Head>
									<Table.Head class="text-right">{@render sortHead('Leads', cardSort === 'leads', () => (cardSort = 'leads'))}</Table.Head>
									<Table.Head class="text-right">Conversion</Table.Head>
									<Table.Head class="text-right">Last viewed</Table.Head>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each sortedCards as c (c.profile_id)}
									<Table.Row>
										<Table.Cell>
											<a href="/dashboard/analytics?card={c.profile_id}{range !== '30' ? `&range=${range}` : ''}" class="font-medium hover:underline">
												{c.name || c.slug}
											</a>
											<div class="text-muted-foreground font-mono text-[11px]">/p/{session.orgHandle}/{c.slug}</div>
										</Table.Cell>
										{#if session.seesOthers}
											<Table.Cell class="text-muted-foreground">{c.assigned_user?.username ?? 'Organisation'}</Table.Cell>
										{/if}
										<Table.Cell class="text-right font-medium tabular">{c.views}</Table.Cell>
										<Table.Cell class="text-right tabular">{c.unique_visitors}</Table.Cell>
										<Table.Cell class="text-muted-foreground text-right tabular">{c.nfc_views} · {c.qr_views} · {c.link_views}</Table.Cell>
										<Table.Cell class="text-right tabular">{c.saves}</Table.Cell>
										<Table.Cell class="text-right tabular">{c.doc_opens}</Table.Cell>
										<Table.Cell class="text-right tabular">{c.form_opens}</Table.Cell>
										<Table.Cell class="text-right tabular">{c.leads}</Table.Cell>
										<Table.Cell class="text-right tabular">{pct(rate(c.leads + c.saves, c.views))}</Table.Cell>
										<Table.Cell class="text-muted-foreground text-right text-xs" title={c.last_viewed_at ? formatDateTime(c.last_viewed_at) : undefined}>
											{c.last_viewed_at ? timeAgo(c.last_viewed_at) : 'Never'}
										</Table.Cell>
									</Table.Row>
								{:else}
									<Table.Row><Table.Cell colspan={11} class="text-muted-foreground py-10 text-center">No cards match these filters</Table.Cell></Table.Row>
								{/each}
							</Table.Body>
						</Table.Root>
					</div>
					<p class="text-muted-foreground text-xs">Conversion counts contacts saved and leads per view.</p>
				</Tabs.Content>

				{#if session.seesOthers}
					<Tabs.Content value="teams" class="flex flex-col gap-6">
						{#if teamStats.length === 0}
							<div class="bg-card flex flex-col items-center gap-2 rounded-xl border px-6 py-12 text-center">
								<p class="font-medium">No teams to compare</p>
								<p class="text-muted-foreground max-w-sm text-sm">
									{session.isAdmin ? 'Group people into teams to compare how each one is doing.' : 'You don’t lead any teams.'}
								</p>
								{#if session.isAdmin}<Button variant="outline" href="/dashboard/teams">Go to Teams</Button>{/if}
							</div>
						{:else}
							<section class="bg-card overflow-hidden rounded-xl border" aria-labelledby="teams-heading">
								<div class="flex flex-col gap-1 border-b px-5 py-4">
									<h2 id="teams-heading" class="font-semibold">Teams compared</h2>
									<p class="text-muted-foreground text-sm">Activity on cards held by each team's people, with the change from the previous period.</p>
								</div>
								<Table.Root>
									<Table.Header>
										<Table.Row>
											<Table.Head>Team</Table.Head>
											<Table.Head class="text-right">People</Table.Head>
											<Table.Head class="text-right">Active cards</Table.Head>
											<Table.Head class="min-w-40">Views</Table.Head>
											<Table.Head class="text-right">Per person</Table.Head>
											<Table.Head class="text-right">Engaged</Table.Head>
											<Table.Head class="text-right">Saves</Table.Head>
											<Table.Head class="text-right">Leads</Table.Head>
											<Table.Head class="text-right">Visit → lead</Table.Head>
										</Table.Row>
									</Table.Header>
									<Table.Body>
										{#each teamStats as t (t.id)}
											<Table.Row>
												<Table.Cell>
													<button type="button" class="flex items-center gap-2 font-medium hover:underline" onclick={() => setParam('team', t.id)}>
														<span class="size-2.5 rounded-full" style="background: {teamColor(t)}"></span>{t.name}
													</button>
												</Table.Cell>
												<Table.Cell class="text-right tabular">{t.members}</Table.Cell>
												<Table.Cell class="text-right tabular">{t.active_cards} / {t.cards}</Table.Cell>
												<Table.Cell>
													<div class="flex items-center gap-2">
														<div class="bg-muted h-1.5 flex-1 rounded-full">
															<div class="h-full rounded-full" style="width: {(t.views / maxTeamViews) * 100}%; background: var(--viz-1)"></div>
														</div>
														<span class="w-10 text-right font-medium tabular">{t.views}</span>
														<span class="w-10">{@render delta(t.views, t.prev_views)}</span>
													</div>
												</Table.Cell>
												<Table.Cell class="text-right tabular">{t.members ? (t.views / t.members).toFixed(1) : '–'}</Table.Cell>
												<Table.Cell class="text-right tabular">{pct(rate(t.engaged_sessions, t.sessions))}</Table.Cell>
												<Table.Cell class="text-right tabular">{t.saves} {@render delta(t.saves, t.prev_saves)}</Table.Cell>
												<Table.Cell class="text-right tabular">{t.leads} {@render delta(t.leads, t.prev_leads)}</Table.Cell>
												<Table.Cell class="text-right tabular">{pct(rate(t.leads, t.sessions))}</Table.Cell>
											</Table.Row>
										{/each}
									</Table.Body>
								</Table.Root>
							</section>
						{/if}

						<section class="bg-card overflow-hidden rounded-xl border" aria-labelledby="board-heading">
							<div class="flex flex-wrap items-center justify-between gap-3 border-b px-5 py-4">
								<div class="flex flex-col gap-1">
									<h2 id="board-heading" class="font-semibold">Leaderboard</h2>
									<p class="text-muted-foreground text-sm">
										{teamId ? `People in ${teamOptions.find((t) => t.id === teamId)?.name ?? 'this team'}` : 'Everyone you can see'}, ranked by
										their cards.
									</p>
								</div>
								<ToggleGroup.Root type="single" variant="outline" size="sm" value={memberSort} onValueChange={(v) => v && (memberSort = v as MemberSort)} aria-label="Rank by">
									<ToggleGroup.Item value="views" class="px-3">Views</ToggleGroup.Item>
									<ToggleGroup.Item value="saves" class="px-3">Saves</ToggleGroup.Item>
									<ToggleGroup.Item value="leads" class="px-3">Leads</ToggleGroup.Item>
								</ToggleGroup.Root>
							</div>
							<ol class="divide-y">
								{#each sortedMembers as m, i (m.user_id)}
									<li class="flex items-center gap-3 px-5 py-3">
										<span class="text-muted-foreground w-5 text-right text-sm tabular">{i + 1}</span>
										<UserAvatar username={m.username} class="size-8 text-xs" />
										<button type="button" class="flex min-w-0 flex-1 flex-col text-left" onclick={() => setParam('user', m.user_id)}>
											<span class="truncate text-sm font-medium hover:underline">{m.username}</span>
											<span class="text-muted-foreground text-xs">
												{m.cards} {m.cards === 1 ? 'card' : 'cards'} · {m.doc_opens} brochure opens · {m.form_opens} forms opened
											</span>
										</button>
										<dl class="grid grid-cols-3 gap-4 text-right text-sm sm:gap-8">
											<div><dt class="text-muted-foreground text-xs">Views</dt><dd class={cn('tabular', memberSort === 'views' && 'font-semibold')}>{m.views}</dd></div>
											<div><dt class="text-muted-foreground text-xs">Saves</dt><dd class={cn('tabular', memberSort === 'saves' && 'font-semibold')}>{m.saves}</dd></div>
											<div><dt class="text-muted-foreground text-xs">Leads</dt><dd class={cn('tabular', memberSort === 'leads' && 'font-semibold')}>{m.leads}</dd></div>
										</dl>
									</li>
								{:else}
									<li class="text-muted-foreground px-5 py-10 text-center text-sm">No one to rank yet</li>
								{/each}
							</ol>
						</section>
					</Tabs.Content>
				{/if}
			</div>
		{/if}
	</Tabs.Root>

	<p class="text-muted-foreground text-xs">
		Visitors aren't identified: there are no cookies, and unique visitors are counted per day. Visits by people signed in to
		{session.orgName || 'your organisation'} aren't counted.
	</p>
</div>
