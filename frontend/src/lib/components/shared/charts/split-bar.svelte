<script lang="ts">
	import { cn } from '$lib/utils';

	type Part = { key: string; label: string; value: number; color: string };

	let {
		parts,
		label,
		format = (n: number) => n.toLocaleString(),
		class: className
	}: {
		parts: Part[];
		/** What the bar shows, for screen readers. */
		label: string;
		format?: (n: number) => string;
		class?: string;
	} = $props();

	const total = $derived(parts.reduce((s, p) => s + p.value, 0));
	const pct = (v: number) => (total > 0 ? (v / total) * 100 : 0);
	let hover = $state<string | null>(null);
</script>

<div class={cn('flex flex-col gap-3', className)}>
	<!-- 2px surface gaps between segments separate them; the legend names them. -->
	<div class="bg-muted flex h-3 w-full gap-0.5 overflow-hidden rounded-full" role="img" aria-label={label}>
		{#if total > 0}
			{#each parts.filter((p) => p.value > 0) as p (p.key)}
				<div
					class="h-full transition-opacity first:rounded-l-full last:rounded-r-full"
					class:opacity-40={hover !== null && hover !== p.key}
					style="width: {pct(p.value)}%; background: {p.color}"
					title="{p.label}: {format(p.value)} ({Math.round(pct(p.value))}%)"
					role="presentation"
					onpointerenter={() => (hover = p.key)}
					onpointerleave={() => (hover = null)}
				></div>
			{/each}
		{/if}
	</div>
	<ul class="flex flex-wrap gap-x-4 gap-y-1.5 text-sm">
		{#each parts as p (p.key)}
			<li
				class="flex items-center gap-1.5"
				onpointerenter={() => (hover = p.key)}
				onpointerleave={() => (hover = null)}
			>
				<span class="inline-block size-2.5 rounded-full" style="background: {p.color}" aria-hidden="true"></span>
				<span class="text-muted-foreground">{p.label}</span>
				<span class="font-medium tabular">{format(p.value)}</span>
				{#if total > 0}<span class="text-muted-foreground text-xs tabular">{Math.round(pct(p.value))}%</span>{/if}
			</li>
		{/each}
	</ul>
</div>
