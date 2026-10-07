<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import NfcWriter from './nfc-writer.svelte';
	import { tapUrl } from '$lib/features/cards/card';
	import { session } from '$lib/core/session.svelte';

	let {
		open = $bindable(false),
		profileId,
		slug,
		name
	}: {
		open?: boolean;
		profileId: number;
		slug: string;
		name: string;
	} = $props();

	// Marked as an NFC tap, so the card's NFC tap behaviour and analytics apply.
	const url = $derived(slug ? tapUrl(session.orgHandle, slug, 'nfc') : '');
	const writerUrl = $derived(`${location.origin}/dashboard/${profileId}/nfc`);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Write to NFC card</Dialog.Title>
			<Dialog.Description>Put {name || 'this card'}'s link on the NFC chip inside your Fronko card.</Dialog.Description>
		</Dialog.Header>
		{#if open && url}
			<NfcWriter {url} {writerUrl} {name} />
		{/if}
	</Dialog.Content>
</Dialog.Root>
