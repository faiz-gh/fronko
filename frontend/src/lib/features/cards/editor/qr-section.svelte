<script lang="ts">
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { session } from '$lib/core/session.svelte';
	import { branding } from '$lib/features/branding/store.svelte';
	import { tapUrl, type CardData } from '../card';
	import QrStyleEditor from '../components/qr-style-editor.svelte';

	let {
		card = $bindable(),
		slug
	}: {
		card: CardData;
		/** The saved link: an unsaved edit isn't live yet, so its QR wouldn't resolve. */
		slug: string;
	} = $props();
</script>

<FormSection
	panel
	id="qr"
	title="QR code"
	description="Put your organisation's logo or an image in the middle, and match your colours."
>
	<QrStyleEditor
		bind:style={card.qr}
		url={tapUrl(session.orgHandle, slug, 'qr')}
		orgLogo={branding.value?.logo_file}
		orgName={branding.value?.name}
		brandColor={branding.value?.signature?.brand_color}
	/>
</FormSection>
