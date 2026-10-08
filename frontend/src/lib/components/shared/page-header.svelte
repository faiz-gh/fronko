<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils';

	let {
		title,
		description,
		actions,
		class: className
	}: {
		title: string;
		/** A line under the title: text, or a snippet for richer content. */
		description?: string | Snippet;
		/** Buttons on the right; they wrap below the title on narrow screens. */
		actions?: Snippet;
		class?: string;
	} = $props();
</script>

<!-- The title row every dashboard page starts with. -->
<header class={cn('flex flex-wrap items-end justify-between gap-4', className)}>
	<div class="flex min-w-0 flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">{title}</h1>
		{#if typeof description === 'string'}
			<p class="text-muted-foreground max-w-3xl text-sm">{description}</p>
		{:else if description}
			<p class="text-muted-foreground max-w-3xl text-sm">{@render description()}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex flex-wrap items-center gap-2">{@render actions()}</div>
	{/if}
</header>
