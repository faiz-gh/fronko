<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import BrandIcon from '$lib/components/shared/brand-icon.svelte';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import PhoneInput from '$lib/components/shared/phone-input.svelte';
	import { detectCalendar, type CardData } from '../card';
	import type { CardErrors } from './validation';

	let { card = $bindable(), errors }: { card: CardData; errors: CardErrors } = $props();

	const emailInvalid = $derived(errors.email);
	const phoneInvalid = $derived(errors.phone);
	const websiteInvalid = $derived(errors.website);
	const calendarInvalid = $derived(errors.calendar);
	const calendarProvider = $derived(detectCalendar(card.calendar_url));
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
		<Field.Field class="sm:col-span-2" data-invalid={calendarInvalid || undefined}>
			<Field.Label for="calendar">Booking link</Field.Label>
			<div class="flex items-center gap-2">
				<span class="bg-muted text-muted-foreground grid size-9 shrink-0 place-items-center rounded-lg">
					<BrandIcon url={card.calendar_url} kind="calendar" />
				</span>
				<Input
					id="calendar"
					bind:value={card.calendar_url}
					placeholder="calendly.com/you"
					aria-invalid={calendarInvalid || undefined}
				/>
			</div>
			{#if calendarInvalid}
				<Field.Error>Enter a valid web address.</Field.Error>
			{:else}
				<Field.Description>
					{calendarProvider
						? `${calendarProvider.name} link. Shown as a “Book a meeting” button.`
						: 'Calendly, Cal.com, Google Calendar or any booking page. Shown as a “Book a meeting” button.'}
				</Field.Description>
			{/if}
		</Field.Field>
	</Field.Group>
</FormSection>
