<script lang="ts">
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import ArrowDownRightIcon from '@lucide/svelte/icons/arrow-down-right';
	import { cn } from '$lib/utils';
	import Sparkline from './sparkline.svelte';

	let {
		label,
		value,
		delta = null,
		periodLabel = 'vs previous period',
		hint,
		trend,
		class: className
	}: {
		label: string;
		value: string;
		/** Percent change against the previous period; null when there's nothing to compare. */
		delta?: number | null;
		periodLabel?: string;
		/** A short line under the value (e.g. "12% of visits"). */
		hint?: string;
		/** Daily values for the sparkline. */
		trend?: number[];
		class?: string;
	} = $props();

	const rounded = $derived(delta === null ? null : Math.round(delta));
</script>

<div class={cn('bg-card flex flex-col gap-1 rounded-xl border p-4', className)}>
	<span class="text-muted-foreground text-sm">{label}</span>
	<span class="text-2xl font-semibold tracking-tight">{value}</span>
	<span class="text-muted-foreground flex min-h-4 items-center gap-1 text-xs">
		{#if rounded !== null && rounded !== 0}
			<span
				class={cn(
					'inline-flex items-center font-medium',
					rounded > 0 ? 'text-emerald-700 dark:text-emerald-400' : 'text-rose-700 dark:text-rose-400'
				)}
			>
				{#if rounded > 0}<ArrowUpRightIcon class="size-3.5" />{:else}<ArrowDownRightIcon class="size-3.5" />{/if}
				{Math.abs(rounded)}%
			</span>
			<span>{periodLabel}</span>
		{:else if rounded === 0}
			<span>No change {periodLabel}</span>
		{:else if hint}
			<span>{hint}</span>
		{/if}
	</span>
	{#if trend && trend.length > 1}
		<Sparkline values={trend} class="mt-2" />
	{/if}
</div>
