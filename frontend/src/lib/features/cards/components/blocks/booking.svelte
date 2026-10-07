<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import type { Booking } from '$lib/features/cards/api';
	import { bookingHref, type Visitor } from '$lib/features/cards/card';

	/**
	 * "Book a meeting": the holder's booking page from Integrations, with the
	 * visitor's details filled in when they've already given them.
	 */
	let { booking, visitor }: { booking?: Booking | null; visitor?: Visitor | null } = $props();

	const href = $derived(booking ? bookingHref(booking, visitor) : null);
</script>

{#if booking && href}
	<a
		{href}
		target="_blank"
		rel="noopener noreferrer"
		data-track="click"
		data-track-target="booking"
		data-track-label="Book a meeting ({booking.name})"
		class="group flex h-12 items-center gap-3 rounded-xl border-2 border-(--card-accent) px-4 text-sm font-semibold transition-colors hover:bg-(--card-accent) hover:text-white"
	>
		<span class="text-(--card-accent) transition-colors group-hover:text-white">
			<BrandIcon url={booking.url} kind="calendar" class="size-[18px]" />
		</span>
		<span class="flex-1">Book a meeting</span>
		{#if booking.provider !== 'booking-link'}
			<span class="text-muted-foreground text-xs font-normal transition-colors group-hover:text-white/80">
				{booking.name}
			</span>
		{/if}
		<ChevronRightIcon class="size-4 transition-transform group-hover:translate-x-0.5" />
	</a>
{/if}
