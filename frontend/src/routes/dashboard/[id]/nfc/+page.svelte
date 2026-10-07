<script lang="ts">
	import { page } from '$app/state';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import { getMyProfile, type Profile } from '$lib/api/profile';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import NfcWriter from '$lib/components/app/nfc-writer.svelte';
	import { normalizeCard, tapUrl } from '$lib/card/card';
	import { session } from '$lib/session.svelte';

	// A page of its own so the desktop's QR code can open the writer on a phone.
	const profileId = $derived(Number(page.params.id));
	let profile = $state<Profile | null>(null);
	let loadError = $state('');

	$effect(() => {
		const id = profileId;
		profile = null;
		loadError = '';
		getMyProfile(id)
			.then((p) => {
				if (id === profileId) profile = p;
			})
			.catch((e) => {
				if (id === profileId) loadError = e instanceof Error ? e.message : 'Failed to load card';
			});
	});

	const name = $derived(profile ? normalizeCard(profile.data).name : '');
</script>

<svelte:head>
	<title>Write to NFC card · Fronko</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-md flex-col gap-6 px-4 py-6 sm:px-8 lg:py-10">
	<Button variant="ghost" size="sm" href="/dashboard/{profileId}#sharing" class="-ml-2 self-start">
		<ArrowLeftIcon data-icon="inline-start" />
		Back to card
	</Button>
	<div class="flex flex-col gap-1">
		<h1 class="text-2xl font-semibold tracking-tight">Write to NFC card</h1>
		<p class="text-muted-foreground text-sm">
			{#if profile}
				Put {name || 'this card'}'s link on the NFC chip inside your Fronko card.
			{:else}
				Put a card's link on the NFC chip inside your Fronko card.
			{/if}
		</p>
	</div>
	{#if loadError}
		<p class="text-destructive text-sm">{loadError}</p>
	{:else if !profile}
		<Skeleton class="h-40 rounded-xl" />
	{:else}
		<NfcWriter
			url={tapUrl(session.orgHandle, profile.slug, 'nfc')}
			writerUrl="{location.origin}/dashboard/{profile.id}/nfc"
			{name}
		/>
	{/if}
</div>
