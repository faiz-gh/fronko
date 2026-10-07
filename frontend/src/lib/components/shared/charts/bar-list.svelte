<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils';

	type Item = { key: string; label: string; value: number; sub?: string; href?: string };

	let {
		items,
		label,
		color = 'var(--viz-1)',
		format = (n: number) => n.toLocaleString(),
		empty = 'Nothing yet',
		limit,
		icon,
		class: className
	}: {
		items: Item[];
		/** What the bars measure, for screen readers. */
		label: string;
		color?: string;
		format?: (n: number) => string;
		empty?: string;
		limit?: number;
		/** Rendered before each label (e.g. a brand icon). */
		icon?: Snippet<[Item]>;
		class?: string;
	} = $props();

	const shown = $derived(limit ? items.slice(0, limit) : items);
	const max = $derived(Math.max(1, ...shown.map((i) => i.value)));
</script>

{#if shown.length === 0}
	<p class="text-muted-foreground py-6 text-center text-sm">{empty}</p>
{:else}
	<ul class={cn('flex flex-col gap-2.5', className)} aria-label={label}>
		{#each shown as item (item.key)}
			<li class="flex flex-col gap-1">
				<div class="flex items-baseline gap-2 text-sm">
					{#if icon}{@render icon(item)}{/if}
					{#if item.href}
						<a href={item.href} class="min-w-0 flex-1 truncate hover:underline">{item.label}</a>
					{:else}
						<span class="min-w-0 flex-1 truncate">{item.label}</span>
					{/if}
					{#if item.sub}<span class="text-muted-foreground shrink-0 text-xs tabular">{item.sub}</span>{/if}
					<span class="shrink-0 font-medium tabular">{format(item.value)}</span>
				</div>
				<!-- Thin bar, rounded data end, anchored at the left baseline. -->
				<div class="bg-muted h-1.5 w-full rounded-full">
					<div
						class="h-full rounded-full"
						style="width: {Math.max(item.value > 0 ? 2 : 0, (item.value / max) * 100)}%; background: {color}"
					></div>
				</div>
			</li>
		{/each}
	</ul>
{/if}
