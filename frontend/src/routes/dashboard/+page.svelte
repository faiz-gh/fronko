<script lang="ts">
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleIcon from '@lucide/svelte/icons/circle';
	import IdCardIcon from '@lucide/svelte/icons/id-card';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import { listLeads, type Lead } from '$lib/api/lead';
	import { STATUS_LABEL, userStatus } from '$lib/api/org';
	import type { Profile } from '$lib/api/profile';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import CardAvatar from '$lib/components/app/card-avatar.svelte';
	import CardTile from '$lib/components/app/card-tile.svelte';
	import CreateUserDialog from '$lib/components/app/create-user-dialog.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import QrDialog from '$lib/components/app/qr-dialog.svelte';
	import RecentLeads from '$lib/components/app/recent-leads.svelte';
	import UserAvatar from '$lib/components/app/user-avatar.svelte';
	import { emptyCard, normalizeCard } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { plural } from '$lib/format';
	import { orgUsers } from '$lib/org-users.svelte';
	import { session } from '$lib/session.svelte';
	import { storage } from '$lib/storage.svelte';

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
	const topPeople = $derived([...people].sort((a, b) => b.lead_count - a.lead_count).slice(0, 6));

	// First steps, until the organisation is set up.
	const steps = $derived([
		...(session.isOwner
			? [{ done: !!storage.status?.configured, label: 'Connect storage for photos and brochures', href: '/dashboard/settings#storage' }]
			: []),
		{ done: (cards.list?.length ?? 0) > 0, label: 'Create a card', action: () => (cards.createOpen = true) },
		{ done: people.length > 0, label: 'Add someone from your team', action: () => (createUserOpen = true) },
		{
			done: (cards.list ?? []).some((p) => p.assigned_user),
			label: 'Assign a card to them',
			href: '/dashboard/cards'
		}
	]);
	const setUp = $derived(cards.list !== null && orgUsers.list !== null && steps.every((s) => s.done));

	const stats = $derived(
		session.isAdmin
			? [
					{ label: 'Team', value: orgUsers.list ? people.length : undefined, href: '/dashboard/users' },
					{ label: 'Cards', value: cards.list?.length, href: '/dashboard/cards' },
					{ label: 'Leads, all time', value: cards.list ? totalLeads : undefined, href: '/dashboard/leads' },
					{ label: 'Leads, last 7 days', value: leadsThisWeek ?? undefined, href: '/dashboard/leads' }
				]
			: [
					{ label: 'Cards', value: cards.list?.length },
					{ label: 'Leads, all time', value: cards.list ? totalLeads : undefined, href: '/dashboard/leads' },
					{ label: 'Leads, last 7 days', value: leadsThisWeek ?? undefined, href: '/dashboard/leads' }
				]
	);

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

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-8 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-wrap items-end justify-between gap-4">
		<div class="flex flex-col gap-1">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Overview</h1>
			<p class="text-muted-foreground text-sm">
				{#if session.isAdmin}
					Welcome back, {session.username}. Here's how {session.orgName} is doing.
				{:else}
					Welcome back, {session.username}. Each card has its own link, QR code and leads.
				{/if}
			</p>
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
		<!-- The 1px gaps over a border-coloured background draw the dividers at any column count. -->
		<dl
			class="bg-border grid gap-px overflow-hidden rounded-xl border {stats.length === 4
				? 'grid-cols-2 lg:grid-cols-4'
				: 'grid-cols-3'}"
		>
			{#each stats as stat (stat.label)}
				<div class="bg-card relative flex flex-col gap-1 px-4 py-4 sm:px-6 sm:py-5">
					<dt class="text-muted-foreground text-xs sm:text-sm">
						{#if stat.href}
							<a href={stat.href} class="hover:text-foreground after:absolute after:inset-0">{stat.label}</a>
						{:else}
							{stat.label}
						{/if}
					</dt>
					<dd class="tabular text-2xl font-semibold tracking-tight sm:text-3xl">
						{#if stat.value === undefined}
							<Skeleton class="mt-1 h-7 w-12" />
						{:else}
							{stat.value}
						{/if}
					</dd>
				</div>
			{/each}
		</dl>

		<div class="grid items-start gap-8 2xl:grid-cols-[minmax(0,1fr)_420px]">
			{#if session.isAdmin}
				<div class="flex flex-col gap-8">
					{#if !setUp && cards.list !== null && orgUsers.list !== null}
						<section class="bg-card flex flex-col gap-4 rounded-xl border p-5 sm:p-6" aria-labelledby="setup-heading">
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
											<a
												href={step.href}
												class="hover:bg-muted flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium"
											>
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

					{#if unassigned.length > 0 || settingUp.length > 0}
						<section class="flex flex-col gap-4" aria-labelledby="attention-heading">
							<h2 id="attention-heading" class="text-sm font-semibold">Needs attention</h2>
							<div class="grid gap-4 lg:grid-cols-2">
								{#if unassigned.length > 0}
									<div class="bg-card flex flex-col overflow-hidden rounded-xl border">
										<div class="flex items-center justify-between gap-3 border-b px-5 py-3">
											<span class="text-sm font-medium">{plural(unassigned.length, 'card')} without a user</span>
											<a href="/dashboard/cards?user=none" class="text-muted-foreground hover:text-foreground text-sm">Assign</a>
										</div>
										<ul class="divide-y">
											{#each unassigned.slice(0, 4) as profile (profile.id)}
												{@const card = normalizeCard(profile.data)}
												<li>
													<a href="/dashboard/{profile.id}" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-2.5">
														<CardAvatar {card} fallback={profile.slug} class="size-7 text-[10px]" />
														<span class="min-w-0 flex-1 truncate text-sm">{card.name || profile.slug}</span>
														<span class="text-muted-foreground font-mono text-[11px]">/p/{profile.slug}</span>
													</a>
												</li>
											{/each}
										</ul>
									</div>
								{/if}
								{#if settingUp.length > 0}
									<div class="bg-card flex flex-col overflow-hidden rounded-xl border">
										<div class="flex items-center justify-between gap-3 border-b px-5 py-3">
											<span class="text-sm font-medium">{plural(settingUp.length, 'person', 'people')} still setting up</span>
											<a href="/dashboard/users" class="text-muted-foreground hover:text-foreground text-sm">Users</a>
										</div>
										<ul class="divide-y">
											{#each settingUp.slice(0, 4) as user (user.id)}
												<li>
													<a href="/dashboard/users/{user.id}" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-2.5">
														<UserAvatar username={user.username} class="size-7 text-[10px]" />
														<span class="min-w-0 flex-1 truncate text-sm">{user.username}</span>
														<span class="text-muted-foreground text-xs">{STATUS_LABEL[userStatus(user)]}</span>
													</a>
												</li>
											{/each}
										</ul>
									</div>
								{/if}
							</div>
						</section>
					{/if}

					<section class="flex flex-col gap-4" aria-labelledby="team-heading">
						<div class="flex items-center justify-between">
							<h2 id="team-heading" class="text-sm font-semibold">Team</h2>
							{#if people.length > 0}
								<a href="/dashboard/users" class="text-muted-foreground hover:text-foreground text-sm">View all</a>
							{/if}
						</div>
						<div class="bg-card overflow-hidden rounded-xl border">
							{#if orgUsers.list === null}
								<div class="p-5"><Skeleton class="h-24" /></div>
							{:else if people.length === 0}
								<div class="flex flex-col items-center gap-3 px-6 py-12 text-center">
									<p class="font-medium">No one else here yet</p>
									<p class="text-muted-foreground max-w-sm text-sm">
										Add your team, then assign them cards. Each person sees only their cards, leads and files.
									</p>
									<Button variant="outline" onclick={() => (createUserOpen = true)}>
										<UserPlusIcon data-icon="inline-start" />
										New user
									</Button>
								</div>
							{:else}
								<ul class="divide-y">
									{#each topPeople as user (user.id)}
										<li>
											<a href="/dashboard/users/{user.id}" class="hover:bg-muted/50 flex items-center gap-3 px-5 py-3">
												<UserAvatar username={user.username} class="size-8 text-xs" />
												<span class="flex min-w-0 flex-1 flex-col">
													<span class="truncate text-sm font-medium">{user.username}</span>
													<span class="text-muted-foreground flex items-center gap-1 text-xs">
														<IdCardIcon class="size-3" />
														{plural(user.card_count, 'card')}
													</span>
												</span>
												<span class="text-right">
													<span class="tabular block text-sm font-semibold">{user.lead_count}</span>
													<span class="text-muted-foreground text-xs">leads</span>
												</span>
											</a>
										</li>
									{/each}
								</ul>
							{/if}
						</div>
					</section>
				</div>
			{:else}
				<section class="flex flex-col gap-4" aria-labelledby="cards-heading">
					<h2 id="cards-heading" class="text-sm font-semibold">Your cards</h2>
					<div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,300px),1fr))]">
						{#if cards.list === null}
							{#each [1, 2] as i (i)}
								<Skeleton class="h-56 rounded-xl" />
							{/each}
						{:else}
							{#each cards.list as profile (profile.id)}
								<CardTile
									{profile}
									onqr={(p) => {
										qrTarget = p;
										qrOpen = true;
									}}
								/>
							{/each}
						{/if}
					</div>
				</section>
			{/if}

			<RecentLeads
				leads={recent}
				total={totalLeads}
				showUser={session.isAdmin}
				empty={session.isAdmin
					? `When someone shares their details from one of ${session.orgName}’s cards, they’ll show up here.`
					: 'When someone shares their details from one of your cards, they’ll show up here.'}
			/>
		</div>
	{/if}
</div>

<QrDialog
	bind:open={qrOpen}
	slug={qrTarget?.slug ?? ''}
	name={qrTarget ? normalizeCard(qrTarget.data).name : ''}
/>
<CreateUserDialog bind:open={createUserOpen} />
