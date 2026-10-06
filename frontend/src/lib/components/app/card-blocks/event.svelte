<script lang="ts">
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import type { BlockOf } from '$lib/card/blocks';

	let { block }: { block: BlockOf<'event'> } = $props();

	const name = $derived(block.name.trim());
	const dates = $derived(block.dates.trim());
	const venue = $derived(block.venue.trim());
	const role = $derived(block.role.trim());
</script>

{#if name || dates || venue || role}
	<div class="overflow-hidden rounded-xl border text-center">
		{#if role}
			<p class="bg-(--card-accent) py-2 text-sm font-bold tracking-[0.2em] text-white uppercase">{role}</p>
		{/if}
		{#if name || dates || venue}
			<div class="flex flex-col items-center gap-1.5 px-4 py-4">
				{#if name}
					<p class="text-base font-semibold tracking-tight">{name}</p>
				{/if}
				{#if dates || venue}
					<div class="text-muted-foreground flex flex-wrap justify-center gap-x-4 gap-y-1 text-xs">
						{#if dates}
							<span class="flex items-center gap-1"><CalendarIcon class="size-3.5" />{dates}</span>
						{/if}
						{#if venue}
							<span class="flex items-center gap-1"><MapPinIcon class="size-3.5" />{venue}</span>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}
