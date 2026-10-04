<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import QrCodeIcon from '@lucide/svelte/icons/qr-code';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { listLeads, type Lead } from '$lib/api/lead';
	import type { Profile } from '$lib/api/profile';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import CardAvatar from '$lib/components/app/card-avatar.svelte';
	import DeleteCardDialog from '$lib/components/app/delete-card-dialog.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import QrDialog from '$lib/components/app/qr-dialog.svelte';
	import { ACCENTS, emptyCard, initials, normalizeCard, publicUrl } from '$lib/card/card';
	import { cards } from '$lib/cards.svelte';
	import { plural, timeAgo } from '$lib/format';
	import { session } from '$lib/session.svelte';

	const WEEK_MS = 7 * 24 * 3600 * 1000;
	const RECENT_LIMIT = 8;

	type RecentLead = Lead & { profile: Profile };

	let recent = $state<RecentLead[] | null>(null);
	let leadsThisWeek = $state<number | null>(null);

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

	const stats = $derived([
		{ label: 'Cards', value: cards.list?.length },
		{ label: 'Leads, all time', value: cards.list ? totalLeads : undefined },
		{ label: 'Leads, last 7 days', value: leadsThisWeek ?? undefined }
	]);

	let qrTarget = $state<Profile | null>(null);
	let qrOpen = $state(false);
	let deleteTarget = $state<Profile | null>(null);

	async function copyLink(slug: string) {
		try {
			await navigator.clipboard.writeText(publicUrl(slug));
			toast.success('Link copied');
		} catch {
			toast.error('Could not copy to clipboard');
		}
	}

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
				Welcome back, {session.username}. Each card has its own link, QR code and leads.
			</p>
		</div>
		{#if cards.list && cards.list.length > 0}
			<Button onclick={() => (cards.createOpen = true)}>
				<PlusIcon data-icon="inline-start" />
				New card
			</Button>
		{/if}
	</header>

	{#if cards.error}
		<div class="bg-card flex flex-col items-start gap-3 rounded-xl border p-6">
			<p class="font-medium">Couldn't load your cards</p>
			<p class="text-muted-foreground text-sm">{cards.error}</p>
			<Button variant="outline" onclick={() => session.username && cards.load(session.username, true)}>Try again</Button>
		</div>
	{:else if cards.list && cards.list.length === 0}
		<!-- First run: show what they're about to make. -->
		<section class="bg-card grid overflow-hidden rounded-2xl border lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
			<div class="flex flex-col justify-center gap-5 p-8 sm:p-12">
				<span class="text-brand text-sm font-medium">Get started</span>
				<h2 class="text-3xl font-semibold tracking-tight text-balance">Create your first card</h2>
				<p class="text-muted-foreground max-w-md text-pretty">
					Add your contact details and links, then share one link or QR code. People save you to their
					phone in one tap, and can send their details back.
				</p>
				<Button size="lg" class="w-fit" onclick={() => (cards.createOpen = true)}>
					<PlusIcon data-icon="inline-start" />
					New card
				</Button>
			</div>
			<div class="bg-muted/50 bg-dots relative hidden min-h-[460px] place-items-center border-l p-10 lg:grid">
				<div class="pointer-events-none w-full max-w-xs opacity-90 select-none" aria-hidden="true">
					<ProfileCard card={sample} slug="you" />
				</div>
			</div>
		</section>
	{:else}
		<dl class="bg-card grid grid-cols-3 divide-x overflow-hidden rounded-xl border">
			{#each stats as stat (stat.label)}
				<div class="flex flex-col gap-1 px-4 py-4 sm:px-6 sm:py-5">
					<dt class="text-muted-foreground text-xs sm:text-sm">{stat.label}</dt>
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
			<section class="flex flex-col gap-4" aria-labelledby="cards-heading">
				<h2 id="cards-heading" class="text-sm font-semibold">Your cards</h2>
				<div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,300px),1fr))]">
					{#if cards.list === null}
						{#each [1, 2, 3] as i (i)}
							<Skeleton class="h-56 rounded-xl" />
						{/each}
					{:else}
						{#each cards.list as profile (profile.id)}
							{@const card = normalizeCard(profile.data)}
							<article
								class="bg-card group relative flex flex-col overflow-hidden rounded-xl border transition-shadow hover:shadow-md"
								style="--card-accent: {ACCENTS[card.accent]}"
							>
								<div
									class="h-20 bg-(--card-accent)"
									style="background-image: radial-gradient(120% 160% at 100% 0%, oklch(1 0 0 / 0.28), transparent 55%), radial-gradient(90% 140% at 0% 100%, oklch(0 0 0 / 0.2), transparent 60%)"
								></div>
								<div class="flex flex-1 flex-col gap-3 px-5 pb-4">
									<div class="-mt-7 flex items-end justify-between">
										<CardAvatar {card} fallback={profile.slug} class="ring-card size-14 text-base ring-4" />
										<div class="relative z-10 flex gap-0.5 opacity-100 transition-opacity lg:opacity-0 lg:group-focus-within:opacity-100 lg:group-hover:opacity-100">
											<Button variant="ghost" size="icon-sm" onclick={() => copyLink(profile.slug)} aria-label="Copy link">
												<CopyIcon />
											</Button>
											<Button
												variant="ghost"
												size="icon-sm"
												onclick={() => {
													qrTarget = profile;
													qrOpen = true;
												}}
												aria-label="QR code"
											>
												<QrCodeIcon />
											</Button>
											<DropdownMenu.Root>
												<DropdownMenu.Trigger
													class={buttonVariants({ variant: 'ghost', size: 'icon-sm' })}
													aria-label="More actions"
												>
													<EllipsisIcon />
												</DropdownMenu.Trigger>
												<DropdownMenu.Content align="end" class="w-48">
													<DropdownMenu.Group>
														<DropdownMenu.Item onSelect={() => goto(`/dashboard/${profile.id}`)}>
															<PencilIcon />
															Edit
														</DropdownMenu.Item>
														<DropdownMenu.Item onSelect={() => window.open(`/p/${profile.slug}`, '_blank')}>
															<ExternalLinkIcon />
															View public page
														</DropdownMenu.Item>
														<DropdownMenu.Item onSelect={() => goto(`/dashboard/leads?card=${profile.id}`)}>
															<InboxIcon />
															View leads
														</DropdownMenu.Item>
													</DropdownMenu.Group>
													<DropdownMenu.Separator />
													<DropdownMenu.Group>
														<DropdownMenu.Item variant="destructive" onSelect={() => (deleteTarget = profile)}>
															<Trash2Icon />
															Delete
														</DropdownMenu.Item>
													</DropdownMenu.Group>
												</DropdownMenu.Content>
											</DropdownMenu.Root>
										</div>
									</div>
									<div class="flex min-w-0 flex-col gap-0.5">
										<h3 class="truncate font-semibold">
											<a href="/dashboard/{profile.id}" class="after:absolute after:inset-0">
												{card.name || profile.slug}
											</a>
										</h3>
										<p class="text-muted-foreground truncate text-sm">
											{[card.title, card.company].filter(Boolean).join(' · ') || 'No title yet'}
										</p>
									</div>
									<div class="text-muted-foreground mt-auto flex items-center justify-between gap-3 border-t pt-3 text-xs">
										<span class="truncate font-mono">/p/{profile.slug}</span>
										<span class={profile.lead_count > 0 ? 'text-foreground shrink-0 font-medium' : 'shrink-0'}>
											{plural(profile.lead_count, 'lead')}
										</span>
									</div>
								</div>
							</article>
						{/each}
						<button
							type="button"
							onclick={() => (cards.createOpen = true)}
							class="text-muted-foreground hover:text-foreground hover:border-foreground/30 flex min-h-20 flex-col items-center justify-center gap-2 rounded-xl border border-dashed sm:min-h-56 text-sm font-medium transition-colors"
						>
							<span class="bg-muted grid size-10 place-items-center rounded-full">
								<PlusIcon class="size-5" />
							</span>
							New card
						</button>
					{/if}
				</div>
			</section>

			<section class="flex flex-col gap-4" aria-labelledby="recent-heading">
				<div class="flex items-center justify-between">
					<h2 id="recent-heading" class="text-sm font-semibold">Recent leads</h2>
					{#if totalLeads > 0}
						<a href="/dashboard/leads" class="text-muted-foreground hover:text-foreground text-sm">View all</a>
					{/if}
				</div>
				<div class="bg-card overflow-hidden rounded-xl border">
					{#if recent === null}
						<div class="flex flex-col gap-4 p-5">
							{#each [1, 2, 3] as i (i)}
								<div class="flex items-center gap-3">
									<Skeleton class="size-9 rounded-full" />
									<div class="flex flex-1 flex-col gap-1.5">
										<Skeleton class="h-3 w-32" />
										<Skeleton class="h-2.5 w-44" />
									</div>
								</div>
							{/each}
						</div>
					{:else if recent.length === 0}
						<div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
							<span class="bg-muted text-muted-foreground grid size-10 place-items-center rounded-full">
								<InboxIcon class="size-5" />
							</span>
							<p class="font-medium">No leads yet</p>
							<p class="text-muted-foreground max-w-xs text-sm">
								When someone shares their details from one of your cards, they'll show up here.
							</p>
						</div>
					{:else}
						<ul class="divide-y">
							{#each recent as lead (lead.id)}
								{@const card = normalizeCard(lead.profile.data)}
								<li>
									<a
										href="/dashboard/leads?card={lead.profile.id}"
										class="hover:bg-muted/50 flex items-start gap-3 px-5 py-3.5 transition-colors"
									>
										<Avatar.Root class="size-9 text-xs">
											<Avatar.Fallback class="bg-muted font-semibold">{initials(lead.name)}</Avatar.Fallback>
										</Avatar.Root>
										<span class="flex min-w-0 flex-1 flex-col gap-0.5">
											<span class="flex items-baseline justify-between gap-2">
												<span class="truncate text-sm font-medium">{lead.name}</span>
												<span class="text-muted-foreground shrink-0 text-xs">{timeAgo(lead.created_at)}</span>
											</span>
											<span class="text-muted-foreground truncate text-xs">{lead.email}</span>
											{#if lead.notes}
												<span class="text-foreground/80 mt-1 line-clamp-2 text-sm">{lead.notes}</span>
											{/if}
											<span class="text-muted-foreground mt-1 flex items-center gap-1.5 text-xs">
												<span class="size-2 rounded-full" style="background: {ACCENTS[card.accent]}"></span>
												via {card.name || lead.profile.slug}
											</span>
										</span>
									</a>
								</li>
							{/each}
						</ul>
						{#if totalLeads > recent.length}
							<a
								href="/dashboard/leads"
								class="text-muted-foreground hover:text-foreground hover:bg-muted/50 block border-t px-5 py-3 text-sm transition-colors"
							>
								View all {totalLeads} leads
							</a>
						{/if}
					{/if}
				</div>
			</section>
		</div>
	{/if}
</div>

<QrDialog
	bind:open={qrOpen}
	slug={qrTarget?.slug ?? ''}
	name={qrTarget ? normalizeCard(qrTarget.data).name : ''}
/>
<DeleteCardDialog bind:target={deleteTarget} />
