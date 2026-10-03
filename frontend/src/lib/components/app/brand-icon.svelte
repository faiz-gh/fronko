<script lang="ts">
	import LinkIcon from '@lucide/svelte/icons/link';
	import { detectBrand } from '$lib/card/card';

	let { url, class: className = 'size-4' }: { url: string; class?: string } = $props();

	const brand = $derived(detectBrand(url));
</script>

{#if brand?.icon}
	<svg viewBox="0 0 24 24" fill="currentColor" class={className} aria-hidden="true">
		<path d={brand.icon.path} />
	</svg>
{:else}
	<LinkIcon class={className} aria-hidden="true" />
{/if}
