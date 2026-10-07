<script lang="ts">
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import LinkIcon from '@lucide/svelte/icons/link';
	import { detectBrand, detectCalendar } from '$lib/features/cards/card';

	let {
		url,
		kind = 'link',
		class: className = 'size-4'
	}: { url: string; kind?: 'link' | 'calendar'; class?: string } = $props();

	const brand = $derived(kind === 'calendar' ? detectCalendar(url) : detectBrand(url));
</script>

{#if brand?.icon}
	<svg viewBox="0 0 24 24" fill="currentColor" class={className} aria-hidden="true">
		<path d={brand.icon.path} />
	</svg>
{:else if kind === 'calendar'}
	<CalendarIcon class={className} aria-hidden="true" />
{:else}
	<LinkIcon class={className} aria-hidden="true" />
{/if}
