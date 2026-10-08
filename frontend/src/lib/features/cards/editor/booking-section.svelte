<script lang="ts">
	import CalendarOffIcon from '@lucide/svelte/icons/calendar-off';
	import CalendarPlusIcon from '@lucide/svelte/icons/calendar-plus';
	import CheckIcon from '@lucide/svelte/icons/check';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { session } from '$lib/core/session.svelte';
	import ProviderLogo from '$lib/features/integrations/components/provider-logo.svelte';
	import { cn } from '$lib/utils';
	import type { Booking } from '../api';
	import { ensureBlock } from '../blocks';
	import { displayUrl, type CardData } from '../card';

	let {
		card = $bindable(),
		options,
		holder
	}: {
		card: CardData;
		/** The pages this card can show: its holder's, then the organisation's. */
		options: Booking[];
		/** Who holds the card; null is the organisation. */
		holder: { id: number; username: string } | null;
	} = $props();

	const mine = $derived(!!holder && holder.username === session.username);
	// The chosen page is gone: deleted, paused, or the old holder's.
	const missing = $derived(
		card.booking_connection_id !== null && !options.some((o) => o.connection_id === card.booking_connection_id)
	);
	const hidden = $derived(
		card.booking_connection_id !== null && !card.blocks.some((b) => b.type === 'booking' && !b.hidden)
	);

	function choose(id: number | null) {
		card.booking_connection_id = id;
		// A page needs somewhere to show: bring back (or add) the booking block.
		if (id !== null) card.blocks = ensureBlock(card.blocks, 'booking', ['quick_actions', 'bio', 'header']);
	}

	function owner(o: Booking): string {
		if (o.scope === 'org') return 'Organisation';
		return mine ? 'Yours' : (holder?.username ?? 'Personal');
	}
</script>

<FormSection
	panel
	id="booking"
	title="Booking"
	description="Pick the booking page for this card's “Book a meeting” button."
>
	<div class="flex flex-col gap-4">
		{#if missing}
			<Alert.Root>
				<TriangleAlertIcon />
				<Alert.Title>The page this card used isn't available</Alert.Title>
				<Alert.Description>
					It was removed or paused, or it belongs to the card's previous holder. Visitors don't see a booking button.
					Pick another page below.
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if options.length === 0}
			<Empty.Root class="border">
				<Empty.Header>
					<Empty.Media variant="icon"><CalendarPlusIcon /></Empty.Media>
					<Empty.Title>No booking pages yet</Empty.Title>
					<Empty.Description>
						Connect Calendly, Cal.com, Microsoft Bookings or any booking link in Integrations. You can connect several,
						then choose one per card here.
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button href="/dashboard/integrations#calendar">Connect a booking page</Button>
				</Empty.Content>
			</Empty.Root>
		{:else}
			<div class="flex flex-col gap-2" role="radiogroup" aria-label="Booking page">
				{#each [...options.map((o) => ({ id: o.connection_id, o })), { id: null, o: null }] as { id, o } (id ?? 'none')}
					{@const selected = card.booking_connection_id === id}
					<button
						type="button"
						role="radio"
						aria-checked={selected}
						onclick={() => choose(id)}
						class={cn(
							'bg-card flex items-center gap-3 rounded-xl border p-3 text-left transition-colors',
							selected ? 'border-foreground ring-foreground ring-1' : 'hover:border-foreground/30'
						)}
					>
						{#if o}
							<ProviderLogo id={o.provider} name={o.name} class="size-9" />
							<span class="flex min-w-0 flex-1 flex-col gap-0.5">
								<span class="flex items-center gap-2 text-sm font-medium">
									<span class="truncate">{o.label || o.name}</span>
									<Badge variant="secondary" class="shrink-0">{owner(o)}</Badge>
								</span>
								<span class="text-muted-foreground truncate text-xs">
									{o.name} · <span class="font-mono">{displayUrl(o.url)}</span>
								</span>
							</span>
						{:else}
							<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
								<CalendarOffIcon class="size-4" />
							</span>
							<span class="flex min-w-0 flex-1 flex-col gap-0.5">
								<span class="text-sm font-medium">Don't show a booking button</span>
								<span class="text-muted-foreground text-xs">Visitors can still email, call or leave their details.</span
								>
							</span>
						{/if}
						<CheckIcon class={cn('size-4 shrink-0', selected ? 'opacity-100' : 'opacity-0')} />
					</button>
				{/each}
			</div>

			{#if hidden}
				<p class="text-muted-foreground text-xs">
					The “Book a meeting” block is hidden in Template &amp; blocks, so the button won't show.
					<button
						type="button"
						class="text-foreground font-medium underline underline-offset-4"
						onclick={() => choose(card.booking_connection_id)}
					>
						Show it
					</button>
				</p>
			{/if}

			<p class="text-muted-foreground text-xs">
				{holder
					? `Shows ${mine ? 'your' : `${holder.username}'s`} booking pages and the organisation's.`
					: 'The organisation holds this card, so it can show the organisation’s booking pages.'}
				<a href="/dashboard/integrations#calendar" class="text-foreground font-medium underline underline-offset-4">
					Add another booking page
				</a>
			</p>
		{/if}
	</div>
</FormSection>
