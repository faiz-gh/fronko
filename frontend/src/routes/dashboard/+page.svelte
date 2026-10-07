<script lang="ts">
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleIcon from '@lucide/svelte/icons/circle';
	import IdCardIcon from '@lucide/svelte/icons/id-card';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import {
		SOURCES,
		change,
		formatCount,
		getAnalyticsActivity,
		getAnalyticsCards,
		getAnalyticsMembers,
		getAnalyticsSummary,
		getAnalyticsTeams,
		getAnalyticsTimeseries,
		lastDays,
		rate,
		type ActivityItem,
		type AnalyticsPoint,
		type AnalyticsSummary,
		type CardStat,
		type MemberStat,
		type TeamStat
	} from '$lib/api/analytics';
	import { listLeads, type Lead } from '$lib/api/lead';
	import { teamColor } from '$lib/api/teams';
	import { STATUS_LABEL, userStatus } from '$lib/api/org';
	import type { Profile } from '$lib/api/profile';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import ActivityFeed from '$lib/components/app/activity-feed.svelte';
	import SeriesChart from '$lib/components/app/charts/series-chart.svelte';
	import SplitBar from '$lib/components/app/charts/split-bar.svelte';
	import StatTile from '$lib/components/app/charts/stat-tile.svelte';
	import CardAvatar from '$lib/components/app/card-avatar.svelte';
	import CardTile from '$lib/components/app/card-tile.svelte';
	import CreateUserDialog from '$lib/components/app/create-user-dialog.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import QrDialog from '$lib/components/app/qr-dialog.svelte';
	import RecentLeads from '$lib/components/app/recent-leads.svelte';
	import UserAvatar from '$lib/components/app/user-avatar.svelte';
	import { emptyCard, normalizeCard } from '$lib/card/card';
	import { qrContrastIssue } from '$lib/card/qr';
	import { cards } from '$lib/cards.svelte';
	import { plural } from '$lib/format';
	import { orgUsers } from '$lib/org-users.svelte';
	import { session } from '$lib/session.svelte';
	import { storage } from '$lib/storage.svelte';
	import { teams } from '$lib/teams.svelte';

	const WEEK_MS = 7 * 24 * 3600 * 1000;
	const RECENT_LIMIT = 8;

	type RecentLead = Lead & { profile: Profile };

	let recent = $state<RecentLead[] | null>(null);
	let leadsThisWeek = $state<number | null>(null);
	let createUserOpen = $state(false);

	const totalLeads = $derived(cards.list?.reduce((sum, p) => sum + p.lead_count, 0) ?? 0);

	// Refetch when cards or their lead counts change (e.g. after a delete).
	let fetchedFor = '';
	$effect(() => {
		const list = cards.list;
		if (!list) return;
		const key = list.map((p) => `${p.id}:${p.lead_count}`).join(',');
		if (key === fetchedFor) return;
		fetchedFor = key;
		const byId = new Map(list.map((p) => [p.id, p]));
		Promise.all([
			listLeads({ pageSize: RECENT_LIMIT }),
			listLeads({ since: new Date(Date.now() - WEEK_MS), pageSize: 1 })
		])
			.then(([latest, week]) => {
				recent = latest.leads.flatMap((l) => {
					const profile = byId.get(l.profile_id);
					return profile ? [{ ...l, profile }] : [];
				});
				leadsThisWeek = week.total;
			})
			.catch(() => {
				recent = [];
				leadsThisWeek = null;
			});
	});

	// Organisation view
	const people = $derived(orgUsers.assignable);
	const unassigned = $derived((cards.list ?? []).filter((p) => !p.assigned_user));
	const settingUp = $derived(people.filter((u) => ['unverified', 'temporary_password'].includes(userStatus(u))));

	// ---- Card analytics for the chosen period -----------------------------------
	const INACTIVE_DAYS = 30;
	let range = $state<'7' | '30'>('7');
	let summary = $state<AnalyticsSummary | null>(null);
	let points = $state<AnalyticsPoint[]>([]);
	let activity = $state<ActivityItem[] | null>(null);
	let cardStats = $state<CardStat[] | null>(null);
	let teamStats = $state<TeamStat[]>([]);
	let topPeople = $state<MemberStat[]>([]);
	let analyticsError = $state(false);

	let analyticsFor = '';
	$effect(() => {
		const key = `${range}|${session.username}|${session.seesOthers}`;
		if (key === analyticsFor || !session.username) return;
		analyticsFor = key;
		const period = lastDays(Number(range));
		const others = session.seesOthers;
		Promise.all([
			getAnalyticsSummary(period),
			getAnalyticsTimeseries(period),
			getAnalyticsActivity(lastDays(INACTIVE_DAYS), 10),
			getAnalyticsCards(lastDays(INACTIVE_DAYS)),
			others ? getAnalyticsTeams(period) : Promise.resolve([]),
			others ? getAnalyticsMembers(period) : Promise.resolve([])
		])
			.then(([s, ts, act, cs, tms, ms]) => {
				if (key !== analyticsFor) return;
				summary = s;
				points = ts;
				activity = act;
				cardStats = cs;
				teamStats = tms.slice(0, 5);
				topPeople = ms.slice(0, 5);
				analyticsError = false;
			})
			.catch(() => {
				if (key !== analyticsFor) return;
				analyticsError = true;
				activity = [];
				cardStats = [];
			});
	});

	const cur = $derived(summary?.current);
	const prev = $derived(summary?.previous);
	const periodLabel = $derived(range === '7' ? 'vs previous 7 days' : 'vs previous 30 days');
	const kpis = $derived(
		cur && prev
			? [
					{ label: 'Views', value: formatCount(cur.views), delta: change(cur.views, prev.views), trend: points.map((p) => p.views) },
					{
						label: 'Unique visitors',
						value: formatCount(cur.unique_visitors),
						delta: change(cur.unique_visitors, prev.unique_visitors),
						trend: points.map((p) => p.unique_visitors)
					},
					{ label: 'Contacts saved', value: formatCount(cur.saves), delta: change(cur.saves, prev.saves), trend: points.map((p) => p.saves) },
					{
						label: 'Leads',
						value: formatCount(cur.leads),
						delta: change(cur.leads, prev.leads),
						trend: points.map((p) => p.leads),
						hint: cur.sessions ? `${Math.round(rate(cur.leads, cur.sessions) ?? 0)}% of visits` : undefined
					}
				]
			: null
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
	const statsById = $derived(new Map((cardStats ?? []).map((c) => [c.profile_id, c])));
	// Assigned cards nobody has opened lately: their tag or QR code may not be in use.
	const inactive = $derived(
		(cardStats ?? []).filter(
			(c) =>
				c.assigned_user &&
				(!c.last_viewed_at || Date.now() - new Date(c.last_viewed_at).getTime() > INACTIVE_DAYS * 86_400_000)
		)
	);
	// Styled codes whose colours may not scan everywhere.
	const riskyQr = $derived((cards.list ?? []).filter((p) => qrContrastIssue(normalizeCard(p.data).qr)));
	const maxTeamViews = $derived(Math.max(1, ...teamStats.map((t) => t.views)));
	const firstCard = $derived(cards.list?.[0] ?? null);

	// First steps, until the organisation is set up.
	const steps = $derived([
		...(session.isOwner
			? [{ done: !!storage.status?.configured, label: 'Connect storage for photos and brochures', href: '/dashboard/settings?tab=storage' }]
			: []),
		{ done: (cards.list?.length ?? 0) > 0, label: 'Create a card', action: () => (cards.createOpen = true) },
		{ done: people.length > 0, label: 'Add someone from your team', action: () => (createUserOpen = true) },
		{
			done: (cards.list ?? []).some((p) => p.assigned_user),
			label: 'Assign a card to them',
			href: '/dashboard/cards'
		},
		{ done: (teams.list?.length ?? 0) > 0, label: 'Group people into teams', href: '/dashboard/teams' }
	]);
	const setUp = $derived(cards.list !== null && orgUsers.list !== null && steps.every((s) => s.done));

	let qrTarget = $state<Profile | null>(null);
	let qrOpen = $state(false);

	const sample = {
		...emptyCard('Your name'),
		title: 'What you do',
		company: 'Where you work',
		email: 'you@example.com',
		phone_country: 'US',
		phone_country_code: '+1',
		phone_number: '5550100000',
		links: [{ id: '1', label: 'LinkedIn', url: 'https://linkedin.com' }]
	};
</script>

<svelte:head>
	<title>Overview · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-6 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-wrap items-end justify-between gap-4">
		<div class="flex flex-col gap-1">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Overview</h1>
			<p class="text-muted-foreground text-sm">
				{#if session.isAdmin}
					Welcome back, {session.username}. Here's how {session.orgName}'s cards are doing.
				{:else}
					Welcome back, {session.username}. Here's how your {cards.list?.length === 1 ? 'card is' : 'cards are'} doing.
				{/if}
			</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			{#if session.isAdmin}
				<Button variant="outline" size="sm" onclick={() => (createUserOpen = true)}>
					<UserPlusIcon data-icon="inline-start" />
					Add person
				</Button>
				<Button size="sm" onclick={() => (cards.createOpen = true)}>
					<PlusIcon data-icon="inline-start" />
					New card
				</Button>
			{:else if firstCard}
				<Button
					variant="outline"
					size="sm"
					onclick={() => {
						qrTarget = firstCard;
						qrOpen = true;
					}}
				>
					<QrCodeIcon data-icon="inline-start" />
					Share
				</Button>
				<Button size="sm" href="/dashboard/{firstCard.id}">
					<PencilIcon data-icon="inline-start" />
					Edit card
				</Button>
			{/if}
		</div>
	</header>

	{#if cards.error}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load your cards</p>
			<p class="text-muted-foreground text-sm">{cards.error}</p>
			<Button variant="outline" onclick={() => session.username && cards.load(session.username, true)}>Try again</Button>
		</div>
	{:else if !session.isAdmin && cards.list && cards.list.length === 0}
		<section class="bg-card grid overflow-hidden rounded-2xl border lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
			<div class="flex flex-col justify-center gap-5 p-8 sm:p-12">
				<span class="text-brand text-sm font-medium">Almost there</span>
				<h2 class="text-3xl font-semibold tracking-tight text-balance">Your card is on its way</h2>
				<p class="text-muted-foreground max-w-md text-pretty">
					{session.orgName} hasn't assigned you a card yet. Once they do, it shows up here and you can make it yours:
					your photo, contact details, links and brochures.
				</p>
				<Button variant="outline" class="w-fit" href="/dashboard/files">Upload your photo in the meantime</Button>
			</div>
			<div class="bg-muted/50 bg-dots relative hidden min-h-[460px] place-items-center border-l p-10 lg:grid">
				<div class="pointer-events-none w-full max-w-xs opacity-90 select-none" aria-hidden="true">
					<ProfileCard card={sample} slug="you" />
				</div>
			</div>
		</section>
	{:else}
		<div class="flex flex-wrap items-center justify-between gap-3">
			<h2 class="text-sm font-semibold">Card activity</h2>
			<ToggleGroup.Root
				type="single"
				variant="outline"
				size="sm"
				value={range}
				onValueChange={(v) => v && (range = v as '7' | '30')}
				aria-label="Period"
			>
				<ToggleGroup.Item value="7" class="px-3">7 days</ToggleGroup.Item>
				<ToggleGroup.Item value="30" class="px-3">30 days</ToggleGroup.Item>
			</ToggleGroup.Root>
		</div>

		<div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
			{#if kpis}
				{#each kpis as k (k.label)}
					<StatTile label={k.label} value={k.value} delta={k.delta} {periodLabel} hint={k.hint} trend={k.trend} />
				{/each}
			{:else if analyticsError}
				<p class="text-muted-foreground col-span-full text-sm">Card activity couldn't be loaded right now.</p>
			{:else}
				{#each [1, 2, 3, 4] as i (i)}<Skeleton class="h-36 rounded-xl" />{/each}
			{/if}
		</div>

		<div class="grid items-start gap-6 2xl:grid-cols-[minmax(0,1fr)_420px]">
			<div class="flex flex-col gap-6">
				<section class="bg-card flex flex-col gap-4 rounded-xl border p-5" aria-labelledby="sources-heading">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<div class="flex flex-col gap-1">
							<h2 id="sources-heading" class="font-semibold">Views by source</h2>
							<p class="text-muted-foreground text-sm">Taps on the NFC tag, scans of the QR code, and everything else.</p>
						</div>
						<Button variant="ghost" size="sm" href="/dashboard/analytics{range === '7' ? '?range=7' : ''}">
							Full analytics
							<ArrowRightIcon data-icon="inline-end" />
						</Button>
					</div>
					{#if summary}
						<SeriesChart dates={points.map((p) => p.date)} series={sourceSeries} label="Views by source" height={200} />
						<SplitBar parts={sourceParts} label="Share of views by source" class="border-t pt-4" />
					{:else}
						<Skeleton class="h-56" />
					{/if}
				</section>

				{#if session.seesOthers}
					<div class="grid items-start gap-6 lg:grid-cols-2">
						<section class="bg-card flex flex-col overflow-hidden rounded-xl border" aria-labelledby="teams-heading">
							<div class="flex items-center justify-between gap-3 border-b px-5 py-3">
								<h2 id="teams-heading" class="text-sm font-semibold">Top teams</h2>
								<a href="/dashboard/analytics?tab=teams" class="text-muted-foreground hover:text-foreground text-sm">Compare</a>
							</div>
							{#if teamStats.length === 0}
								<div class="flex flex-col items-center gap-3 px-6 py-10 text-center">
									<p class="text-muted-foreground max-w-xs text-sm">
										{session.isAdmin ? 'Group people into teams to see which one gets the most out of their cards.' : 'No team activity yet.'}
									</p>
									{#if session.isAdmin && (teams.list?.length ?? 0) === 0}
										<Button variant="outline" size="sm" href="/dashboard/teams">Create a team</Button>
									{/if}
								</div>
							{:else}
								<ul class="divide-y">
									{#each teamStats as t (t.id)}
										<li>
											<a href="/dashboard/analytics?tab=teams&team={t.id}" class="hover:bg-muted/50 flex flex-col gap-2 px-5 py-3">
												<span class="flex items-center gap-2 text-sm">
													<span class="size-2.5 rounded-full" style="background: {teamColor(t)}"></span>
													<span class="min-w-0 flex-1 truncate font-medium">{t.name}</span>
													<span class="text-muted-foreground text-xs tabular">{plural(t.saves, 'save')} · {plural(t.leads, 'lead')}</span>
													<span class="w-12 text-right font-semibold tabular">{t.views}</span>
												</span>
												<span class="bg-muted h-1.5 rounded-full">
													<span class="block h-full rounded-full" style="width: {(t.views / maxTeamViews) * 100}%; background: var(--viz-1)"></span>
												</span>
											</a>
										</li>
									{/each}
								</ul>
							{/if}
						</section>

						<section class="bg-card flex flex-col overflow-hidden rounded-xl border" aria-labelledby="people-heading">
							<div class="flex items-center justify-between gap-3 border-b px-5 py-3">
								<h2 id="people-heading" class="text-sm font-semibold">Top people</h2>
								<a href="/dashboard/analytics?tab=teams" class="text-muted-foreground hover:text-foreground text-sm">Leaderboard</a>
							</div>
							{#if topPeople.length === 0}
								<div class="flex flex-col items-center gap-3 px-6 py-10 text-center">
									<p class="text-muted-foreground max-w-xs text-sm">Add your team, then assign them cards.</p>
									{#if session.isAdmin}
										<Button variant="outline" size="sm" onclick={() => (createUserOpen = true)}>
											<UserPlusIcon data-icon="inline-start" />
											New user
										</Button>
									{/if}
								</div>
							{:else}
								<ol class="divide-y">
									{#each topPeople as m, i (m.user_id)}
										<li>
											<a href="/dashboard/analytics?user={m.user_id}" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-2.5">
												<span class="text-muted-foreground w-4 text-right text-xs tabular">{i + 1}</span>
												<UserAvatar username={m.username} class="size-7 text-[10px]" />
												<span class="flex min-w-0 flex-1 flex-col">
													<span class="truncate text-sm font-medium">{m.username}</span>
													<span class="text-muted-foreground flex items-center gap-1 text-xs">
														<IdCardIcon class="size-3" />{plural(m.cards, 'card')} · {plural(m.saves, 'save')} · {plural(m.leads, 'lead')}
													</span>
												</span>
												<span class="text-right">
													<span class="tabular block text-sm font-semibold">{m.views}</span>
													<span class="text-muted-foreground text-xs">views</span>
												</span>
											</a>
										</li>
									{/each}
								</ol>
							{/if}
						</section>
					</div>
				{/if}

				{#if !session.isAdmin}
					<section class="flex flex-col gap-4" aria-labelledby="cards-heading">
						<h2 id="cards-heading" class="text-sm font-semibold">Your cards</h2>
						<div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,300px),1fr))]">
							{#if cards.list === null}
								{#each [1, 2] as i (i)}
									<Skeleton class="h-56 rounded-xl" />
								{/each}
							{:else}
								{#each cards.list.filter((p) => !session.isLead || p.assigned_user?.username === session.username) as profile (profile.id)}
									{@const st = statsById.get(profile.id)}
									<div class="flex flex-col gap-2">
										<CardTile
											{profile}
											onqr={(p) => {
												qrTarget = p;
												qrOpen = true;
											}}
										/>
										{#if st}
											<a
												href="/dashboard/analytics?card={profile.id}"
												class="text-muted-foreground hover:text-foreground flex justify-between gap-2 px-1 text-xs tabular"
											>
												<span>{INACTIVE_DAYS} days:</span>
												<span>{plural(st.views, 'view')}</span><span>{plural(st.saves, 'save')}</span>
												<span>{plural(st.doc_opens, 'brochure open')}</span><span>{plural(st.leads, 'lead')}</span>
											</a>
										{/if}
									</div>
								{/each}
							{/if}
						</div>
					</section>
				{/if}
			</div>

			<div class="flex flex-col gap-6">
				{#if session.isAdmin && !setUp && cards.list !== null && orgUsers.list !== null}
					<section class="bg-card flex flex-col gap-4 rounded-xl border p-5" aria-labelledby="setup-heading">
						<div class="flex flex-col gap-1">
							<h2 id="setup-heading" class="font-semibold">Set up {session.orgName}</h2>
							<p class="text-muted-foreground text-sm">
								{steps.filter((s) => s.done).length} of {steps.length} done.
							</p>
						</div>
						<ol class="flex flex-col gap-1">
							{#each steps as step (step.label)}
								<li>
									{#if step.done}
										<span class="text-muted-foreground flex items-center gap-3 px-2 py-1.5 text-sm line-through">
											<CircleCheckIcon class="text-brand size-4 shrink-0" />
											{step.label}
										</span>
									{:else if step.href}
										<a href={step.href} class="hover:bg-muted flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium">
											<CircleIcon class="text-muted-foreground size-4 shrink-0" />
											{step.label}
											<ArrowRightIcon class="text-muted-foreground ml-auto size-4" />
										</a>
									{:else}
										<button
											type="button"
											onclick={step.action}
											class="hover:bg-muted flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-left text-sm font-medium"
										>
											<CircleIcon class="text-muted-foreground size-4 shrink-0" />
											{step.label}
											<ArrowRightIcon class="text-muted-foreground ml-auto size-4" />
										</button>
									{/if}
								</li>
							{/each}
						</ol>
					</section>
				{/if}

				{#if (session.isAdmin && (unassigned.length > 0 || settingUp.length > 0)) || (session.seesOthers && inactive.length > 0) || riskyQr.length > 0}
					<section class="bg-card flex flex-col overflow-hidden rounded-xl border" aria-labelledby="attention-heading">
						<h2 id="attention-heading" class="border-b px-5 py-3 text-sm font-semibold">Needs attention</h2>
						<ul class="divide-y">
							{#if session.isAdmin && unassigned.length > 0}
								<li>
									<a href="/dashboard/cards?user=none" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-3">
										<div class="flex -space-x-2">
											{#each unassigned.slice(0, 3) as profile (profile.id)}
												<CardAvatar card={normalizeCard(profile.data)} fallback={profile.slug} class="ring-card size-7 text-[10px] ring-2" />
											{/each}
										</div>
										<span class="min-w-0 flex-1 text-sm">{plural(unassigned.length, 'card')} without a user</span>
										<span class="text-muted-foreground text-xs">Assign</span>
									</a>
								</li>
							{/if}
							{#if session.isAdmin && settingUp.length > 0}
								<li>
									<a href="/dashboard/users" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-3">
										<div class="flex -space-x-2">
											{#each settingUp.slice(0, 3) as user (user.id)}
												<UserAvatar username={user.username} class="ring-card size-7 text-[10px] ring-2" />
											{/each}
										</div>
										<span class="min-w-0 flex-1 text-sm">
											{plural(settingUp.length, 'person', 'people')} still setting up
											<span class="text-muted-foreground block text-xs">
												{settingUp
													.slice(0, 2)
													.map((u) => `${u.username}: ${STATUS_LABEL[userStatus(u)].toLowerCase()}`)
													.join(', ')}
											</span>
										</span>
										<span class="text-muted-foreground text-xs">Users</span>
									</a>
								</li>
							{/if}
							{#if session.seesOthers && inactive.length > 0}
								<li>
									<a href="/dashboard/analytics?tab=cards" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-3">
										<span class="min-w-0 flex-1 text-sm">
											{plural(inactive.length, 'card')} not viewed in {INACTIVE_DAYS} days
											<span class="text-muted-foreground block truncate text-xs">
												{inactive
													.slice(0, 3)
													.map((c) => c.name || c.slug)
													.join(', ')}
											</span>
										</span>
										<span class="text-muted-foreground text-xs">Review</span>
									</a>
								</li>
							{/if}
							{#if riskyQr.length > 0}
								<li>
									<a href="/dashboard/{riskyQr[0].id}#qr" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-3">
										<span class="min-w-0 flex-1 text-sm">
											{riskyQr.length === 1 ? 'A QR code' : `${riskyQr.length} QR codes`} may be hard to scan
											<span class="text-muted-foreground block truncate text-xs">
												{riskyQr
													.slice(0, 3)
													.map((p) => normalizeCard(p.data).name || p.slug)
													.join(', ')}: low contrast or light on dark
											</span>
										</span>
										<span class="text-muted-foreground text-xs">Fix</span>
									</a>
								</li>
							{/if}
						</ul>
					</section>
				{/if}

				<ActivityFeed items={activity} showUser={session.seesOthers} />

				<RecentLeads
					leads={recent}
					total={totalLeads}
					showUser={session.seesOthers}
					empty={session.isAdmin
						? `When someone shares their details from one of ${session.orgName}’s cards, they’ll show up here.`
						: 'When someone shares their details from one of your cards, they’ll show up here.'}
				/>
			</div>
		</div>
	{/if}
</div>

<QrDialog
	bind:open={qrOpen}
	slug={qrTarget?.slug ?? ''}
	name={qrTarget ? normalizeCard(qrTarget.data).name : ''}
	style={qrTarget ? normalizeCard(qrTarget.data).qr : undefined}
/>
<CreateUserDialog bind:open={createUserOpen} />
