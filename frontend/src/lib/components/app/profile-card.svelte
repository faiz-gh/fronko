<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { PublicFile } from '$lib/api/files';
	import { ACCENTS, type CardData } from '$lib/card/card';
	import { cn } from '$lib/utils';
	import Header from './card-blocks/header.svelte';
	import Bio from './card-blocks/bio.svelte';
	import QuickActions from './card-blocks/quick-actions.svelte';
	import Booking from './card-blocks/booking.svelte';
	import Links from './card-blocks/links.svelte';
	import Documents from './card-blocks/documents.svelte';
	import Gallery from './card-blocks/gallery.svelte';
	import EventBlock from './card-blocks/event.svelte';

	let {
		card,
		slug,
		actions,
		files,
		class: className
	}: {
		card: CardData;
		slug: string;
		/**
		 * Metadata for the library files the card uses. When given, brochures and
		 * gallery images whose file no longer exists are hidden and sizes are shown.
		 */
		files?: Record<string, PublicFile>;
		/** Visitor buttons (save contact, exchange, …), placed where the card's "actions" block is. */
		actions?: Snippet;
		class?: string;
	} = $props();

	const header = $derived(card.blocks[0]?.type === 'header' ? card.blocks[0] : null);
	const body = $derived(card.blocks.slice(header ? 1 : 0).filter((b) => !b.hidden));
	// A compact header is left-aligned, so the content under it is too.
	const align = $derived(header?.style === 'compact' ? 'start' : 'center');
</script>

<!--
	The card carries its own theme: `.dark` re-scopes the design tokens for this subtree.
	Blocks stack in one column, in the order the owner chose.
-->
<article
	class={cn(card.theme === 'dark' ? 'dark' : 'light', '@container w-full', className)}
	style="--card-accent: {ACCENTS[card.accent]}"
>
	<div class="bg-card text-card-foreground ring-foreground/10 overflow-hidden rounded-3xl shadow-xl ring-1">
		<Header {card} {slug} style={header?.style ?? 'banner'} />

		<div class="flex flex-col gap-5 px-6 pt-5 pb-6 @lg:px-8 @lg:pb-8">
			{#each body as block (block.id)}
				{#if block.type === 'bio'}
					<Bio {card} {align} />
				{:else if block.type === 'quick_actions'}
					<QuickActions {card} {align} />
				{:else if block.type === 'booking'}
					<Booking {card} />
				{:else if block.type === 'actions'}
					{#if actions}
						<div class="flex flex-col gap-2">{@render actions()}</div>
					{/if}
				{:else if block.type === 'links'}
					<Links {card} />
				{:else if block.type === 'documents'}
					<Documents {card} {files} />
				{:else if block.type === 'heading'}
					{#if block.text.trim()}
						<h2 class="-mb-2 text-base font-semibold tracking-tight">{block.text}</h2>
					{/if}
				{:else if block.type === 'text'}
					{#if block.text.trim()}
						<p class="text-foreground/80 text-sm leading-relaxed whitespace-pre-line">{block.text}</p>
					{/if}
				{:else if block.type === 'gallery'}
					<Gallery images={block.images} {files} />
				{:else if block.type === 'event'}
					<EventBlock {block} />
				{:else if block.type === 'divider'}
					<hr class="border-border" />
				{/if}
			{/each}
		</div>
	</div>
</article>
