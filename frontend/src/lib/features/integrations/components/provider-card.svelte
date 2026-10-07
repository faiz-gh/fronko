<script lang="ts">
	import { cn } from '$lib/utils';
	import { catalogBadge } from '../registry';
	import type { CatalogEntry } from '../types';
	import ProviderLogo from './provider-logo.svelte';
	import StatusBadge from './status-badge.svelte';

	let { entry }: { entry: CatalogEntry } = $props();

	const badge = $derived(catalogBadge(entry));
	const soon = $derived(entry.status === 'coming_soon');
</script>

<li
	class={cn(
		'bg-card group relative flex flex-col gap-3 rounded-xl border p-4 transition-shadow',
		soon ? 'border-dashed' : 'hover:shadow-md'
	)}
>
	<div class="flex items-start justify-between gap-3">
		<ProviderLogo id={entry.id} name={entry.name} class={cn('size-10', soon && 'opacity-60 grayscale')} />
		{#if badge}
			<StatusBadge tone={badge.tone} label={badge.label} />
		{/if}
	</div>
	<div class="flex flex-col gap-1">
		<h3 class={cn('text-sm font-semibold', soon && 'text-muted-foreground')}>
			<a href="/dashboard/integrations/{entry.id}" class="after:absolute after:inset-0 focus-visible:outline-none">
				{entry.name}
			</a>
		</h3>
		<p class="text-muted-foreground line-clamp-2 text-sm">{entry.description}</p>
	</div>
	<span
		class="ring-ring pointer-events-none absolute inset-0 rounded-xl ring-offset-2 group-has-[a:focus-visible]:ring-2"
	></span>
</li>
