<script lang="ts">
	import StarIcon from '@lucide/svelte/icons/star';
	import { cn } from '$lib/utils';

	let { rating, class: className }: { rating: number; class?: string } = $props();
</script>

<span class={cn('flex items-center gap-0.5', className)} role="img" aria-label="Rated {rating} of 5">
	{#each [1, 2, 3, 4, 5] as n (n)}
		<StarIcon class={cn('size-3.5', n <= rating ? 'fill-amber-400 text-amber-400' : 'text-muted-foreground/40')} />
	{/each}
</span>
