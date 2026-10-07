<script lang="ts">
	import type { CardData } from '$lib/features/cards/card';

	let { card, align = 'center' }: { card: CardData; align?: 'center' | 'start' } = $props();
</script>

{#if card.bio.trim()}
	<p
		class="text-foreground/80 max-w-prose text-sm leading-relaxed whitespace-pre-line @lg:text-base"
		class:text-center={align === 'center'}
		class:mx-auto={align === 'center'}
	>
		{card.bio}
	</p>
{/if}
