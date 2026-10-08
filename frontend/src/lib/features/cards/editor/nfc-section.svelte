<script lang="ts">
	import SignatureIcon from '@lucide/svelte/icons/signature';
	import SmartphoneNfcIcon from '@lucide/svelte/icons/smartphone-nfc';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import FormSection from '$lib/components/shared/form-section.svelte';

	let {
		profileId,
		onnfc
	}: {
		profileId: number;
		/** Opens the NFC writer. */
		onnfc: () => void;
	} = $props();
</script>

<FormSection
	panel
	id="nfc"
	title="NFC & signature"
	description="Other places your card can go: an NFC card people tap, and your email signature."
>
	<Field.Group class="gap-4">
		<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
			<Field.Content>
				<Field.Label>NFC card</Field.Label>
				<Field.Description>
					Write this card's link to the chip in a metal or plastic NFC card. Phones open your card when tapped. What a
					tap opens is set under Link &amp; tap.
				</Field.Description>
			</Field.Content>
			<Button variant="outline" onclick={onnfc}>
				<SmartphoneNfcIcon data-icon="inline-start" />
				Write to NFC card
			</Button>
		</Field.Field>
		<Field.Field orientation="horizontal" class="bg-card rounded-xl border p-4">
			<Field.Content>
				<Field.Label>Email signature</Field.Label>
				<Field.Description>
					Turn this card into a signature for Gmail, Outlook or Apple Mail. It uses the card's details and booking page.
				</Field.Description>
			</Field.Content>
			<Button variant="outline" href="/dashboard/signatures?card={profileId}">
				<SignatureIcon data-icon="inline-start" />
				Create signature
			</Button>
		</Field.Field>
	</Field.Group>
</FormSection>
