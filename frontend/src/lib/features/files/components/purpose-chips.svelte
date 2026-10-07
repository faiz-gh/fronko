<script lang="ts">
	import { PURPOSES, type FilePurpose } from '$lib/features/files/api';
	import { cn } from '$lib/utils';
	import PurposeIcon from './purpose-icon.svelte';

	let {
		value = $bindable(null),
		purposes,
		counts,
		total,
		class: className
	}: {
		/** null is every purpose. */
		value?: FilePurpose | null;
		/** The purposes to offer, in order. */
		purposes: FilePurpose[];
		/** Files per purpose; chips with none are dimmed. Omit while loading. */
		counts?: Partial<Record<FilePurpose, number>>;
		total?: number;
		class?: string;
	} = $props();
</script>

<div class={cn('-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1', className)} role="tablist" aria-label="What the files are for">
	{#each [null, ...purposes] as p (p ?? 'all')}
		{@const n = p === null ? total : counts?.[p]}
		{@const active = value === p}
		<button
			type="button"
			role="tab"
			aria-selected={active}
			onclick={() => (value = p)}
			class={cn(
				'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-xs font-medium whitespace-nowrap transition-colors',
				'focus-visible:ring-ring/50 outline-none focus-visible:ring-3',
				active
					? 'border-foreground bg-foreground text-background'
					: 'bg-background text-muted-foreground hover:text-foreground hover:border-foreground/30',
				!active && counts && p !== null && !n && 'opacity-60'
			)}
		>
			<PurposeIcon purpose={p ?? 'all'} class="size-3.5" />
			{p === null ? 'All' : PURPOSES[p].plural}
			{#if n !== undefined}
				<span class={cn('tabular', active ? 'text-background/70' : 'text-muted-foreground/70')}>{n}</span>
			{/if}
		</button>
	{/each}
</div>
