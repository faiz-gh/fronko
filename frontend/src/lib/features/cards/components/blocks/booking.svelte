<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import { detectCalendar, safeUrl, type CardData } from '$lib/features/cards/card';

	let { card }: { card: CardData } = $props();

	const calendar = $derived(safeUrl(card.calendar_url));
	const calendarName = $derived(calendar ? detectCalendar(calendar)?.name : undefined);
</script>

{#if calendar}
	<a
		href={calendar}
		target="_blank"
		rel="noopener noreferrer"
		data-track="click"
		data-track-target="booking"
		data-track-label={calendarName ? `Book a meeting (${calendarName})` : 'Book a meeting'}
		class="group flex h-12 items-center gap-3 rounded-xl border-2 border-(--card-accent) px-4 text-sm font-semibold transition-colors hover:bg-(--card-accent) hover:text-white"
	>
		<span class="text-(--card-accent) transition-colors group-hover:text-white">
			<BrandIcon url={calendar} kind="calendar" class="size-[18px]" />
		</span>
		<span class="flex-1">Book a meeting</span>
		{#if calendarName}
			<span class="text-muted-foreground text-xs font-normal transition-colors group-hover:text-white/80">
				{calendarName}
			</span>
		{/if}
		<ChevronRightIcon class="size-4 transition-transform group-hover:translate-x-0.5" />
	</a>
{/if}
