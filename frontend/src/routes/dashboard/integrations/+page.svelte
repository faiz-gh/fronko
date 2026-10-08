<script lang="ts">
	import SearchIcon from '@lucide/svelte/icons/search';
	import { Button } from '$lib/components/ui/button';
	import LoadError from '$lib/components/shared/load-error.svelte';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { session } from '$lib/core/session.svelte';
	import * as Empty from '$lib/components/ui/empty';
	import ProviderCard from '$lib/features/integrations/components/provider-card.svelte';
	import ProviderLogo from '$lib/features/integrations/components/provider-logo.svelte';
	import { CATEGORY_ICONS, matchesSearch } from '$lib/features/integrations/registry';
	import { integrations } from '$lib/features/integrations/store.svelte';
	import type { CatalogEntry, Category } from '$lib/features/integrations/types';

	$effect(() => {
		if (session.ready && session.username) integrations.load(session.username, true);
	});

	let query = $state('');

	// Within a section: connected first, then available, then coming soon.
	function rank(p: CatalogEntry): number {
		if (p.connections.length > 0) return 0;
		if (p.status === 'coming_soon') return 2;
		return 1;
	}
	const sections = $derived.by(() => {
		const c = integrations.catalog;
		if (!c) return [];
		return c.categories
			.map((cat) => {
				const all = c.providers
					.filter((p) => p.category === cat.id && matchesSearch(p, query))
					.map((p, i) => ({ p, i }))
					.sort((a, b) => rank(a.p) - rank(b.p) || a.i - b.i)
					.map(({ p }) => p);
				return {
					...cat,
					count: all.length,
					connected: all.reduce((n, p) => n + p.connections.length, 0),
					// Coming-soon providers get a compact row, so what works today stays in view.
					providers: all.filter((p) => p.status !== 'coming_soon'),
					soon: all.filter((p) => p.status === 'coming_soon')
				};
			})
			.filter((s) => s.count > 0);
	});

	const icon = (c: Category) => CATEGORY_ICONS[c];
</script>

<svelte:head>
	<title>Integrations · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-[1680px] flex-col gap-8 px-4 py-6 sm:px-8 lg:px-10 lg:py-10">
	<header class="flex flex-wrap items-end justify-between gap-4">
		<div class="flex flex-col gap-1">
			<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">Integrations</h1>
			<p class="text-muted-foreground max-w-2xl text-sm">
				{#if session.isAdmin}
					Connect {session.orgName} to the tools you already use: send leads to your CRM, show booking pages on cards, and
					manage people and sign-in from your identity provider.
				{:else}
					Connect your own tools, such as the booking pages your cards can show.
				{/if}
			</p>
		</div>
		<InputGroup.Root class="w-full sm:w-72">
			<InputGroup.Addon><SearchIcon /></InputGroup.Addon>
			<InputGroup.Input bind:value={query} placeholder="Search integrations" aria-label="Search integrations" />
		</InputGroup.Root>
	</header>

	{#if integrations.error && !integrations.catalog}
		<LoadError what="integrations" message={integrations.error} onretry={() => integrations.refresh()} />
	{:else if !integrations.catalog}
		{#each [1, 2] as s (s)}
			<div class="flex flex-col gap-4">
				<Skeleton class="h-6 w-48" />
				<div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,260px),1fr))]">
					{#each [1, 2, 3, 4] as i (i)}<Skeleton class="h-36 rounded-xl" />{/each}
				</div>
			</div>
		{/each}
	{:else if sections.length === 0}
		<Empty.Root class="bg-card rounded-xl border border-dashed py-16">
			<Empty.Header>
				<Empty.Media variant="icon"><SearchIcon /></Empty.Media>
				<Empty.Title>No integrations match “{query}”</Empty.Title>
				<Empty.Description>Try another name, or ask for it in an integration request.</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				<Button variant="outline" onclick={() => (query = '')}>Clear search</Button>
			</Empty.Content>
		</Empty.Root>
	{:else}
		<nav aria-label="Integration categories" class="-mt-2 flex flex-wrap gap-2">
			{#each sections as section (section.id)}
				{@const Icon = icon(section.id)}
				<a
					href="#{section.id}"
					class="bg-card hover:bg-muted inline-flex h-8 items-center gap-2 rounded-full border px-3 text-sm transition-colors"
				>
					<Icon class="text-muted-foreground size-3.5" />
					{section.label}
					{#if section.connected}
						<span
							class="rounded-full bg-emerald-500/15 px-1.5 text-xs font-medium text-emerald-700 dark:text-emerald-400"
						>
							{section.connected} connected
						</span>
					{/if}
				</a>
			{/each}
		</nav>
		{#each sections as section (section.id)}
			{@const Icon = icon(section.id)}
			<section id={section.id} aria-labelledby="cat-{section.id}" class="flex scroll-mt-6 flex-col gap-4">
				<div class="flex items-start gap-3">
					<span class="bg-muted text-muted-foreground grid size-8 shrink-0 place-items-center rounded-lg">
						<Icon class="size-4" />
					</span>
					<div class="flex flex-col gap-0.5">
						<h2 id="cat-{section.id}" class="text-base font-semibold tracking-tight">{section.label}</h2>
						<p class="text-muted-foreground text-sm">{section.description}</p>
					</div>
				</div>
				{#if section.providers.length}
					<ul class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,260px),1fr))]">
						{#each section.providers as entry (entry.id)}
							<ProviderCard {entry} />
						{/each}
					</ul>
				{/if}
				{#if section.soon.length}
					<div class="flex flex-wrap items-center gap-2">
						<span class="text-muted-foreground mr-1 text-xs font-medium">Coming soon</span>
						{#each section.soon as entry (entry.id)}
							<a
								href="/dashboard/integrations/{entry.id}"
								class="text-muted-foreground hover:text-foreground hover:bg-muted inline-flex h-8 items-center gap-2 rounded-full border border-dashed px-2.5 text-sm transition-colors"
							>
								<ProviderLogo
									id={entry.id}
									name={entry.name}
									class="size-5 rounded-md text-[10px] opacity-70 grayscale"
								/>
								{entry.name}
							</a>
						{/each}
					</div>
				{/if}
			</section>
		{/each}
	{/if}
</div>
