<script lang="ts">
	import * as Field from '$lib/components/ui/field';
	import FormSection from '$lib/components/shared/form-section.svelte';
	import { branding } from '$lib/features/branding/store.svelte';
	import type { LibraryFile, PublicFile } from '$lib/features/files/api';
	import { applyTemplate, TEMPLATES, type TemplateKey } from '../blocks';
	import { ACCENTS, type CardData } from '../card';
	import BlockListEditor from '../components/block-list-editor.svelte';
	import TemplatePicker from '../components/template-picker.svelte';
	import { isCustomised } from './sections';

	let {
		card = $bindable(),
		files,
		onfile
	}: {
		card: CardData;
		files: Record<string, PublicFile>;
		onfile: (file: LibraryFile) => void;
	} = $props();

	const customised = $derived(isCustomised(card));

	function chooseTemplate(key: TemplateKey) {
		if (key === card.template && !customised) return;
		if (customised && !confirm(`Switch to the ${TEMPLATES[key].label} layout? Your block order and hidden blocks will be reset; text, gallery and event details are kept.`)) return;
		card.blocks = applyTemplate(card.blocks, key);
		card.template = key;
	}
</script>

<FormSection
	panel
	id="layout"
	title="Layout"
	description="Start from a template, then reorder, hide or add blocks."
>
	<div class="flex flex-col gap-6">
		<Field.Field>
			<Field.Label>Template</Field.Label>
			<TemplatePicker
				value={card.template}
				accent={ACCENTS[card.accent]}
				logo={!!branding.value?.logo_file && (branding.value.logo_policy === 'required' || card.show_org_logo)}
				onselect={chooseTemplate}
			/>
			{#if customised}
				<Field.Description>
					Customised from {TEMPLATES[card.template].label}. Pick a template to reset the layout.
				</Field.Description>
			{/if}
		</Field.Field>
		<Field.Field>
			<Field.Label>Blocks</Field.Label>
			<BlockListEditor bind:blocks={card.blocks} {files} {onfile} />
		</Field.Field>
	</div>
</FormSection>
