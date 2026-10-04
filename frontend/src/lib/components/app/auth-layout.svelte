<script lang="ts">
	import type { Snippet } from 'svelte';
	import Logo from '$lib/components/app/logo.svelte';
	import ProfileCard from '$lib/components/app/profile-card.svelte';
	import { emptyCard, type CardData } from '$lib/card/card';

	let { children }: { children: Snippet } = $props();

	// Illustrative sample for the side panel.
	const sample: CardData = {
		...emptyCard('Daniel Reyes'),
		title: 'Founder',
		company: 'Tidewater Coffee',
		location: 'Austin, TX',
		email: 'daniel@example.com',
		phone_country: 'US',
		phone_country_code: '+1',
		phone_number: '5125550199',
		website: 'tidewater.example',
		accent: 'orange',
		links: [{ id: '1', label: '', url: 'https://instagram.com/tidewater' }]
	};
</script>

<!-- Shared frame for sign-in, verification and password reset: the form on the left, a sample card on the right. -->
<div class="grid min-h-svh lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
	<div class="flex flex-col gap-4 p-6 md:p-10 lg:p-12">
		<Logo />
		<div class="flex flex-1 items-center justify-center">
			<div class="flex w-full max-w-sm flex-col gap-7">
				{@render children()}
			</div>
		</div>
	</div>

	<div class="bg-primary text-primary-foreground relative hidden overflow-hidden lg:flex lg:flex-col lg:justify-between lg:p-12">
		<div
			class="absolute inset-0 opacity-60"
			style="background-image: radial-gradient(oklch(1 0 0 / 0.12) 1px, transparent 1px); background-size: 18px 18px; mask-image: radial-gradient(70% 60% at 60% 45%, black, transparent)"
			aria-hidden="true"
		></div>
		<div
			class="absolute top-1/4 left-1/2 size-[520px] -translate-x-1/2 rounded-full opacity-50 blur-3xl"
			style="background: radial-gradient(closest-side, oklch(0.64 0.19 45 / 0.5), transparent)"
			aria-hidden="true"
		></div>

		<div class="relative grid flex-1 place-items-center py-8">
			<div class="pointer-events-none w-full max-w-[340px] rotate-[-2deg] select-none" aria-hidden="true">
				<ProfileCard card={sample} slug="daniel" />
			</div>
		</div>

		<div class="relative flex max-w-md flex-col gap-2">
			<p class="text-xl font-medium tracking-tight text-balance">
				One link for your contact details, your socials, and a way for people to reach you back.
			</p>
			<p class="text-primary-foreground/60 text-sm">Make as many cards as you need. Every lead lands in one place.</p>
		</div>
	</div>
</div>
