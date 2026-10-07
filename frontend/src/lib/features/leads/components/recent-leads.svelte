<script lang="ts">
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import type { Lead } from '$lib/features/leads/api';
	import type { Profile } from '$lib/features/cards/api';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { ACCENTS, initials, normalizeCard } from '$lib/features/cards/card';
	import { timeAgo } from '$lib/core/format';

	let {
		leads,
		total,
		showUser = false,
		empty
	}: {
		/** null while loading. */
		leads: (Lead & { profile: Profile })[] | null;
		/** All leads, for the "view all" link. */
		total: number;
		/** Say who held the card (admins). */
		showUser?: boolean;
		empty: string;
	} = $props();
</script>

<section class="flex flex-col gap-4" aria-labelledby="recent-heading">
	<div class="flex items-center justify-between">
		<h2 id="recent-heading" class="text-sm font-semibold">Recent leads</h2>
		{#if total > 0}
			<a href="/dashboard/leads" class="text-muted-foreground hover:text-foreground text-sm">View all</a>
		{/if}
	</div>
	<div class="bg-card overflow-hidden rounded-xl border">
		{#if leads === null}
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
		{:else if leads.length === 0}
			<div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
				<span class="bg-muted text-muted-foreground grid size-10 place-items-center rounded-full">
					<InboxIcon class="size-5" />
				</span>
				<p class="font-medium">No leads yet</p>
				<p class="text-muted-foreground max-w-xs text-sm">{empty}</p>
			</div>
		{:else}
			<ul class="divide-y">
				{#each leads as lead (lead.id)}
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
									<span class="size-2 shrink-0 rounded-full" style="background: {ACCENTS[card.accent]}"></span>
									<span class="truncate">
										via {card.name || lead.profile.slug}{showUser && lead.assigned_user
											? ` · ${lead.assigned_user.username}`
											: ''}
									</span>
								</span>
							</span>
						</a>
					</li>
				{/each}
			</ul>
			{#if total > leads.length}
				<a
					href="/dashboard/leads"
					class="text-muted-foreground hover:text-foreground hover:bg-muted/50 block border-t px-5 py-3 text-sm transition-colors"
				>
					View all {total} leads
				</a>
			{/if}
		{/if}
	</div>
</section>
