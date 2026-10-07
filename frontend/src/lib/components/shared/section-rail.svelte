<script lang="ts" module>
	import type { Component } from 'svelte';

	export interface RailItem<T extends string = string> {
		id: T;
		label: string;
		icon: Component;
		/** One line on what the section holds now. */
		summary: string;
		/** A field in the section is invalid. */
		error?: boolean;
	}
</script>

<script lang="ts" generics="Id extends string">
	import { cn } from '$lib/utils';

	let {
		items,
		active,
		onselect,
		label,
		class: className
	}: {
		items: RailItem<Id>[];
		active: Id;
		onselect: (id: Id) => void;
		label: string;
		class?: string;
	} = $props();

	function onkeydown(e: KeyboardEvent, index: number) {
		const step = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }[e.key];
		if (!step) return;
		e.preventDefault();
		const next = items[(index + step + items.length) % items.length];
		onselect(next.id);
		document.getElementById(`rail-${next.id}`)?.focus();
	}
</script>

<!--
	Every section of a long form at a glance, each with a one-line summary;
	only the active one is shown beside it. A vertical list on wide screens,
	a scrolling strip on narrow ones.
-->
<div
	role="tablist"
	aria-label={label}
	aria-orientation="vertical"
	class={cn(
		'-mx-4 flex gap-1 overflow-x-auto px-4 pb-1 lg:mx-0 lg:flex-col lg:overflow-visible lg:px-0 lg:pb-0',
		className
	)}
>
	{#each items as item, i (item.id)}
		{@const selected = item.id === active}
		<button
			type="button"
			role="tab"
			id="rail-{item.id}"
			aria-selected={selected}
			aria-controls="panel-{item.id}"
			aria-labelledby="rail-{item.id}-label"
			aria-describedby="rail-{item.id}-summary"
			tabindex={selected ? 0 : -1}
			onclick={() => onselect(item.id)}
			onkeydown={(e) => onkeydown(e, i)}
			class={cn(
				'group flex shrink-0 items-start gap-2.5 rounded-lg px-2.5 py-2 text-left transition-colors lg:w-full',
				selected ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground'
			)}
		>
			<item.icon class="mt-0.5 size-4 shrink-0" />
			<span class="flex min-w-0 flex-col">
				<span id="rail-{item.id}-label" class="flex items-center gap-1.5 text-sm font-medium">
					{item.label}
					{#if item.error}
						<span class="bg-destructive size-1.5 rounded-full" aria-hidden="true"></span>
						<span class="sr-only">(has a problem)</span>
					{/if}
				</span>
				<span id="rail-{item.id}-summary" class="text-muted-foreground hidden max-w-44 truncate text-xs lg:block"
					>{item.summary}</span
				>
			</span>
		</button>
	{/each}
</div>
