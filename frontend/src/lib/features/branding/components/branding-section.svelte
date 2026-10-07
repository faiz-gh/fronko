<script lang="ts">
	import FormSection from '$lib/components/shared/form-section.svelte';
	import BrandingSettings from './branding-settings.svelte';

	// The Branding tab in Settings.
</script>

	<FormSection
		id="branding"
		title="Branding"
		description="Your logo on cards and email signatures, and what every signature includes."
	>
		<BrandingSettings />
	</FormSection>
