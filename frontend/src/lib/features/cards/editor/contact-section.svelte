<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import PhoneInput from '$lib/components/shared/phone-input.svelte';
	import CalendarClockIcon from '@lucide/svelte/icons/calendar-clock';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import { Button } from '$lib/components/ui/button';
	import { session } from '$lib/core/session.svelte';
	import type { Booking } from '../api';
	import { displayUrl, type CardData } from '../card';
	import type { CardErrors } from './validation';

	let {
		card = $bindable(),
		errors,
		booking,
		holder
	}: {
		card: CardData;
		errors: CardErrors;
		/** The booking page the card shows, from Integrations. */
		booking?: Booking | null;
		/** Who holds the card; null is the organisation. */
		holder?: { id: number; username: string } | null;
	} = $props();

	const mine = $derived(!!holder && holder.username === session.username);
	// Where the page comes from, and where it's managed.
	const source = $derived.by(() => {
		if (!booking) return '';
		if (booking.scope === 'org') {
			return mine
				? "Your organisation's default. Connect your own booking page in Integrations to use it instead."
				: "Your organisation's default booking page.";
		}
		return mine ? 'From your connection in Integrations.' : `${holder?.username ?? 'The holder'}'s own booking page.`;
	});
	const manageHref = $derived(
		booking && ((booking.scope === 'user' && mine) || (booking.scope === 'org' && session.isAdmin && !mine))
			? `/dashboard/integrations/${booking.provider}`
			: '/dashboard/integrations#calendar'
	);

	const emailInvalid = $derived(errors.email);
	const phoneInvalid = $derived(errors.phone);
	const websiteInvalid = $derived(errors.website);
</script>

<FormSection
	panel
	id="contact"
	title="Contact"
	description="Shown as quick actions, and included when someone saves your contact."
>
	<Field.Group class="grid gap-5 sm:grid-cols-2">
		<Field.Field data-invalid={emailInvalid || undefined}>
			<Field.Label for="email">Email</Field.Label>
			<Input
				id="email"
				type="email"
				bind:value={card.email}
				placeholder="jane@acme.com"
				aria-invalid={emailInvalid || undefined}
			/>
			{#if emailInvalid}
				<Field.Error>Enter a valid email address.</Field.Error>
			{/if}
		</Field.Field>
		<Field.Field data-invalid={phoneInvalid || undefined}>
			<Field.Label for="phone">Mobile number</Field.Label>
			<PhoneInput
				id="phone"
				bind:country={card.phone_country}
				bind:code={card.phone_country_code}
				bind:number={card.phone_number}
				invalid={phoneInvalid}
			/>
			{#if phoneInvalid}
				<Field.Error>
					{card.phone_country_code
						? 'Enter a valid number for this country.'
						: 'Pick the country code for this number.'}
				</Field.Error>
			{/if}
		</Field.Field>
		<Field.Field class="sm:col-span-2" data-invalid={websiteInvalid || undefined}>
			<Field.Label for="website">Website</Field.Label>
			<Input id="website" bind:value={card.website} placeholder="acme.com" aria-invalid={websiteInvalid || undefined} />
			{#if websiteInvalid}
				<Field.Error>Enter a valid web address.</Field.Error>
			{/if}
		</Field.Field>
		<div class="flex flex-col gap-3 rounded-lg border p-4 sm:col-span-2">
			<div class="flex items-start gap-3">
				<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
					{#if booking}<BrandIcon url={booking.url} kind="calendar" />{:else}<CalendarClockIcon class="size-4" />{/if}
				</span>
				<div class="flex min-w-0 flex-1 flex-col gap-0.5">
					<p class="text-sm font-medium">Booking page</p>
					{#if booking}
						<p class="text-muted-foreground truncate text-sm">
							{booking.name} · <span class="font-mono text-xs">{displayUrl(booking.url)}</span>
						</p>
						<p class="text-muted-foreground text-xs">
							Shown as a “Book a meeting” button. {source}
						</p>
					{:else}
						<p class="text-muted-foreground text-sm">
							Connect Calendly, Microsoft Bookings or another booking page in Integrations to show a “Book a meeting”
							button.
						</p>
					{/if}
				</div>
			</div>
			<Button variant="outline" size="sm" class="self-start" href={manageHref}>
				<ExternalLinkIcon data-icon="inline-start" />
				{booking ? 'Manage in Integrations' : 'Connect a booking page'}
			</Button>
		</div>
	</Field.Group>
</FormSection>
